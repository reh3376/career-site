package users

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/reh3376/career-site/services/api/internal/tenant"
)

// Ask Roger's store, against a real Postgres for the same reason the
// booking tests are: the properties that matter here are ownership
// scoping and a cascade, and a mock would assert that the code calls
// what the code calls.
//
//	MEETINGS_TEST_DB=postgres://career:career@localhost:55433/career go test ./internal/users/

func chatRepo(t *testing.T) (*Repo, context.Context) {
	t.Helper()
	pool := testPool(t)
	return New(pool), tenant.WithID(context.Background(), tenant.Default)
}

func makeMember(t *testing.T, r *Repo, ctx context.Context, email string) int64 {
	t.Helper()
	var id int64
	err := r.pool.QueryRow(ctx, `
    INSERT INTO users (email, name, password_hash, role, status)
    VALUES ($1, 'Test Member', 'x', 'member', 'active') RETURNING id`, email).Scan(&id)
	if err != nil {
		t.Fatalf("make member: %v", err)
	}
	t.Cleanup(func() { _, _ = r.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) })
	return id
}

func TestConversationRoundTripsWithCitations(t *testing.T) {
	r, ctx := chatRepo(t)
	u := makeMember(t, r, ctx, "chat-rt@example.test")

	cid, err := r.CreateConversation(ctx, Conversation{UserID: u, Title: "Bourbon", PersonaVersion: "v1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := r.AppendMessage(ctx, ChatMessage{
		ConversationID: cid, Role: "user", Text: "How long in bourbon?",
	}); err != nil {
		t.Fatalf("append question: %v", err)
	}
	if _, err := r.AppendMessage(ctx, ChatMessage{
		ConversationID: cid, Role: "assistant", Text: "Eight years.", PersonaVersion: "v1",
		Citations: []ChatCitation{
			{ChunkID: 337, Title: "LLM and applied AI", Path: "/articles/llm", Rank: 1},
			{ChunkID: 42, Title: "A private note", Rank: 2}, // no path: private source
		},
	}); err != nil {
		t.Fatalf("append answer: %v", err)
	}

	conv, msgs, err := r.GetConversation(ctx, u, cid)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if conv.MessageCount != 2 || len(msgs) != 2 {
		t.Fatalf("got %d messages", len(msgs))
	}
	if len(msgs[1].Citations) != 2 {
		t.Fatalf("answer has %d citations, want 2", len(msgs[1].Citations))
	}
	// Order is by rank, and the private source keeps an empty path so
	// the surface can render it as a title with no link.
	if msgs[1].Citations[0].Rank != 1 || msgs[1].Citations[1].Path != "" {
		t.Errorf("citations came back wrong: %+v", msgs[1].Citations)
	}
	// Unrated must not read as neutral.
	if msgs[1].RatingUp != nil {
		t.Error("a new message arrived already rated")
	}
}

// A conversation id is a small integer. Guessing one must return
// nothing, not somebody else's questions.
func TestAnotherMemberCannotReadOrTouchIt(t *testing.T) {
	r, ctx := chatRepo(t)
	owner := makeMember(t, r, ctx, "chat-owner@example.test")
	other := makeMember(t, r, ctx, "chat-other@example.test")

	cid, err := r.CreateConversation(ctx, Conversation{UserID: owner, Title: "Private"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	mid, err := r.AppendMessage(ctx, ChatMessage{ConversationID: cid, Role: "assistant", Text: "hi"})
	if err != nil {
		t.Fatalf("append: %v", err)
	}

	if _, _, err := r.GetConversation(ctx, other, cid); !errors.Is(err, ErrConversationNotFound) {
		t.Errorf("read by a stranger: got %v, want ErrConversationNotFound", err)
	}
	if err := r.DeleteConversation(ctx, other, cid); !errors.Is(err, ErrConversationNotFound) {
		t.Errorf("delete by a stranger: got %v", err)
	}
	if err := r.SetConversationTitle(ctx, other, cid, "hacked"); !errors.Is(err, ErrConversationNotFound) {
		t.Errorf("rename by a stranger: got %v", err)
	}
	if err := r.RateMessage(ctx, other, mid, true, ""); !errors.Is(err, ErrConversationNotFound) {
		t.Errorf("rate by a stranger: got %v", err)
	}
	// And the owner still has it intact.
	if _, msgs, err := r.GetConversation(ctx, owner, cid); err != nil || len(msgs) != 1 {
		t.Errorf("owner lost access: %v, %d messages", err, len(msgs))
	}
}

// Deletion is what the member asked for, immediately, and reversible
// until the purge runs.
func TestDeleteHidesThenPurgeRemoves(t *testing.T) {
	r, ctx := chatRepo(t)
	u := makeMember(t, r, ctx, "chat-del@example.test")
	cid, err := r.CreateConversation(ctx, Conversation{UserID: u, Title: "Gone soon"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := r.AppendMessage(ctx, ChatMessage{ConversationID: cid, Role: "user", Text: "q"}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := r.DeleteConversation(ctx, u, cid); err != nil {
		t.Fatalf("delete: %v", err)
	}

	list, err := r.ListConversations(ctx, u, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, c := range list {
		if c.ID == cid {
			t.Fatal("a deleted conversation is still listed")
		}
	}
	if _, _, err := r.GetConversation(ctx, u, cid); !errors.Is(err, ErrConversationNotFound) {
		t.Errorf("deleted conversation still readable: %v", err)
	}

	// Still there underneath, which is what makes it recoverable.
	var n int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM chat_messages WHERE conversation_id=$1`, cid).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("soft delete removed the message immediately; got %d", n)
	}

	// The purge is what actually removes it, and takes the message with it.
	if _, err := r.PurgeDeletedConversations(ctx, 0); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM chat_messages WHERE conversation_id=$1`, cid).Scan(&n); err != nil {
		t.Fatalf("count after purge: %v", err)
	}
	if n != 0 {
		t.Errorf("purge left %d messages behind", n)
	}
}

// A member cannot rate their own question, only an answer.
func TestOnlyAssistantMessagesCanBeRated(t *testing.T) {
	r, ctx := chatRepo(t)
	u := makeMember(t, r, ctx, "chat-rate@example.test")
	cid, _ := r.CreateConversation(ctx, Conversation{UserID: u})
	qid, err := r.AppendMessage(ctx, ChatMessage{ConversationID: cid, Role: "user", Text: "q"})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := r.RateMessage(ctx, u, qid, true, ""); !errors.Is(err, ErrConversationNotFound) {
		t.Errorf("rating a question was allowed: %v", err)
	}
}

// Appending must move the thread's ordering key, or the list sorts by
// when a conversation started rather than when it was last used.
func TestAppendingMovesTheConversationUpTheList(t *testing.T) {
	r, ctx := chatRepo(t)
	u := makeMember(t, r, ctx, "chat-order@example.test")
	older, _ := r.CreateConversation(ctx, Conversation{UserID: u, Title: "older"})
	newer, _ := r.CreateConversation(ctx, Conversation{UserID: u, Title: "newer"})

	time.Sleep(10 * time.Millisecond)
	if _, err := r.AppendMessage(ctx, ChatMessage{ConversationID: older, Role: "user", Text: "q"}); err != nil {
		t.Fatalf("append: %v", err)
	}

	list, err := r.ListConversations(ctx, u, 50)
	if err != nil || len(list) < 2 {
		t.Fatalf("list: %v, %d rows", err, len(list))
	}
	if list[0].ID != older {
		t.Errorf("the thread just used is not first; got %q", list[0].Title)
	}
	_ = newer
}
