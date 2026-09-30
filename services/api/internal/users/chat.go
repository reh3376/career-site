package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/reh3376/career-site/services/api/internal/tenant"
)

// Ask Roger's conversation store (migration 00045).
//
// Every read is scoped to the member as well as the row id. A
// conversation id is a small integer and guessing one must return
// nothing rather than someone else's questions, so ownership is part of
// the query rather than a check the caller is trusted to remember.

// Conversation is one thread.
type Conversation struct {
	ID               int64
	UserID           int64
	Title            string
	PersonaVersion   string
	ContextContentID string
	StartedAt        time.Time
	LastMessageAt    time.Time
	MessageCount     int
	DeletedAt        *time.Time
}

// ChatMessage is one message in a thread.
type ChatMessage struct {
	ID             int64
	ConversationID int64
	Role           string // user | assistant | owner
	Text           string
	CreatedAt      time.Time

	// Assistant-side. Four separate situations that all render as a
	// short answer, kept apart so the surface can explain itself.
	OutOfScope     bool
	NoSupport      bool
	Degraded       bool
	QAMatch        bool
	PersonaVersion string

	// Nil rating means unrated, which is not the same as neutral.
	RatingUp      *bool
	RatingComment string
	RatedAt       *time.Time

	RunID     *string
	Citations []ChatCitation
}

// ChatCitation is one source behind an assistant message. Denormalised
// at write time so a re-index cannot rewrite what an answer cited.
type ChatCitation struct {
	ChunkID   int64
	ContentID string
	Title     string
	Path      string
	Heading   string
	Rank      int
}

// ErrConversationNotFound covers both "no such row" and "not yours",
// deliberately. Telling a stranger which of the two it is would let
// them map the id space.
var ErrConversationNotFound = errors.New("no such conversation")

// CreateConversation opens a thread for a member.
func (r *Repo) CreateConversation(ctx context.Context, c Conversation) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
    INSERT INTO chat_conversations
      (tenant_id, user_id, title, persona_version, context_content_id)
    VALUES ($1, $2, $3, $4, $5)
    RETURNING id`,
		tenant.FromContext(ctx).Int64(), c.UserID, c.Title,
		c.PersonaVersion, c.ContextContentID).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create conversation: %w", err)
	}
	return id, nil
}

// ListConversations returns a member's threads, most recent first.
// Soft-deleted threads are excluded: the member asked for them gone.
func (r *Repo) ListConversations(ctx context.Context, userID int64, limit int) ([]Conversation, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
    SELECT c.id, c.user_id, c.title, c.persona_version, c.context_content_id,
           c.started_at, c.last_message_at,
           (SELECT count(*) FROM chat_messages m WHERE m.conversation_id = c.id)
      FROM chat_conversations c
     WHERE c.tenant_id = $1 AND c.user_id = $2 AND c.deleted_at IS NULL
     ORDER BY c.last_message_at DESC
     LIMIT $3`, tenant.FromContext(ctx).Int64(), userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()

	var out []Conversation
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.UserID, &c.Title, &c.PersonaVersion,
			&c.ContextContentID, &c.StartedAt, &c.LastMessageAt, &c.MessageCount); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetConversation returns one thread the member owns, with its
// messages and their citations in order.
func (r *Repo) GetConversation(ctx context.Context, userID, id int64) (Conversation, []ChatMessage, error) {
	var c Conversation
	err := r.pool.QueryRow(ctx, `
    SELECT id, user_id, title, persona_version, context_content_id,
           started_at, last_message_at
      FROM chat_conversations
     WHERE tenant_id = $1 AND id = $2 AND user_id = $3 AND deleted_at IS NULL`,
		tenant.FromContext(ctx).Int64(), id, userID).Scan(
		&c.ID, &c.UserID, &c.Title, &c.PersonaVersion,
		&c.ContextContentID, &c.StartedAt, &c.LastMessageAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Conversation{}, nil, ErrConversationNotFound
	}
	if err != nil {
		return Conversation{}, nil, fmt.Errorf("get conversation: %w", err)
	}

	msgs, err := r.messagesFor(ctx, id)
	if err != nil {
		return Conversation{}, nil, err
	}
	c.MessageCount = len(msgs)
	return c, msgs, nil
}

func (r *Repo) messagesFor(ctx context.Context, conversationID int64) ([]ChatMessage, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT id, conversation_id, role, text, created_at,
           out_of_scope, no_support, degraded, qa_match, persona_version,
           rating_up, rating_comment, rated_at, run_id::text
      FROM chat_messages
     WHERE conversation_id = $1
     ORDER BY id`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	var out []ChatMessage
	byID := map[int64]int{}
	for rows.Next() {
		var m ChatMessage
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Text, &m.CreatedAt,
			&m.OutOfScope, &m.NoSupport, &m.Degraded, &m.QAMatch, &m.PersonaVersion,
			&m.RatingUp, &m.RatingComment, &m.RatedAt, &m.RunID); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		byID[m.ID] = len(out)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}

	// Citations in one pass rather than per message: a thread of twenty
	// answers would otherwise be twenty-one queries to render one page.
	crows, err := r.pool.Query(ctx, `
    SELECT ct.message_id, ct.chunk_id, ct.content_id, ct.title, ct.path, ct.heading, ct.rank
      FROM chat_citations ct
      JOIN chat_messages m ON m.id = ct.message_id
     WHERE m.conversation_id = $1
     ORDER BY ct.message_id, ct.rank`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("list citations: %w", err)
	}
	defer crows.Close()
	for crows.Next() {
		var mid int64
		var ct ChatCitation
		if err := crows.Scan(&mid, &ct.ChunkID, &ct.ContentID, &ct.Title,
			&ct.Path, &ct.Heading, &ct.Rank); err != nil {
			return nil, fmt.Errorf("scan citation: %w", err)
		}
		if i, ok := byID[mid]; ok {
			out[i].Citations = append(out[i].Citations, ct)
		}
	}
	return out, crows.Err()
}

// AppendMessage stores one message and its citations, and moves the
// conversation's last_message_at.
//
// One transaction, because a message whose citations did not land would
// be an answer that silently lost its sources, and an answer that
// cannot show its working is exactly what this feature must not
// produce.
func (r *Repo) AppendMessage(ctx context.Context, m ChatMessage) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("append message: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id int64
	if err := tx.QueryRow(ctx, `
    INSERT INTO chat_messages
      (tenant_id, conversation_id, role, text,
       out_of_scope, no_support, degraded, qa_match, persona_version, run_id)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10,'')::uuid)
    RETURNING id`,
		tenant.FromContext(ctx).Int64(), m.ConversationID, m.Role, m.Text,
		m.OutOfScope, m.NoSupport, m.Degraded, m.QAMatch, m.PersonaVersion,
		derefOr(m.RunID, "")).Scan(&id); err != nil {
		return 0, fmt.Errorf("insert message: %w", err)
	}

	for _, c := range m.Citations {
		if _, err := tx.Exec(ctx, `
    INSERT INTO chat_citations (message_id, chunk_id, content_id, title, path, heading, rank)
    VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			id, c.ChunkID, c.ContentID, c.Title, c.Path, c.Heading, c.Rank); err != nil {
			return 0, fmt.Errorf("insert citation: %w", err)
		}
	}

	if _, err := tx.Exec(ctx,
		`UPDATE chat_conversations SET last_message_at = now() WHERE id = $1`,
		m.ConversationID); err != nil {
		return 0, fmt.Errorf("touch conversation: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("append message: %w", err)
	}
	return id, nil
}

// SetConversationTitle renames a thread the member owns.
func (r *Repo) SetConversationTitle(ctx context.Context, userID, id int64, title string) error {
	tag, err := r.pool.Exec(ctx, `
    UPDATE chat_conversations SET title = $4
     WHERE tenant_id = $1 AND id = $2 AND user_id = $3 AND deleted_at IS NULL`,
		tenant.FromContext(ctx).Int64(), id, userID, title)
	if err != nil {
		return fmt.Errorf("set conversation title: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrConversationNotFound
	}
	return nil
}

// DeleteConversation soft-deletes a thread. The member sees it gone at
// once; a job removes it later, which is what makes an accidental
// delete recoverable inside that window (FR-CHAT-08).
func (r *Repo) DeleteConversation(ctx context.Context, userID, id int64) error {
	tag, err := r.pool.Exec(ctx, `
    UPDATE chat_conversations SET deleted_at = now()
     WHERE tenant_id = $1 AND id = $2 AND user_id = $3 AND deleted_at IS NULL`,
		tenant.FromContext(ctx).Int64(), id, userID)
	if err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrConversationNotFound
	}
	return nil
}

// PurgeDeletedConversations hard-deletes what was soft-deleted longer
// ago than the retention window. Messages and citations go with them by
// cascade.
func (r *Repo) PurgeDeletedConversations(ctx context.Context, olderThan time.Duration) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
    DELETE FROM chat_conversations
     WHERE deleted_at IS NOT NULL AND deleted_at < now() - $1::interval`,
		fmt.Sprintf("%d seconds", int(olderThan.Seconds())))
	if err != nil {
		return 0, fmt.Errorf("purge deleted conversations: %w", err)
	}
	return tag.RowsAffected(), nil
}

// RateMessage records the member's rating, scoped through the
// conversation so a member can only rate their own thread.
func (r *Repo) RateMessage(ctx context.Context, userID, messageID int64, up bool, comment string) error {
	tag, err := r.pool.Exec(ctx, `
    UPDATE chat_messages m
       SET rating_up = $4, rating_comment = $5, rated_at = now()
      FROM chat_conversations c
     WHERE m.id = $2 AND m.conversation_id = c.id
       AND c.tenant_id = $1 AND c.user_id = $3 AND c.deleted_at IS NULL
       AND m.role = 'assistant'`,
		tenant.FromContext(ctx).Int64(), messageID, userID, up, comment)
	if err != nil {
		return fmt.Errorf("rate message: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Covers not-yours, not-found, and rating your own question,
		// which is not a thing.
		return ErrConversationNotFound
	}
	return nil
}

func derefOr(p *string, fallback string) string {
	if p == nil {
		return fallback
	}
	return *p
}
