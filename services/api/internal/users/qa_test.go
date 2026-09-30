package users

import (
	"context"
	"errors"
	"testing"
)

// The Q&A bank against a real Postgres, for the same reason the
// conversation store is: what matters here is a vector search, a
// cascade and a partial unique index, and none of those exist in a
// mock.
//
//	MEETINGS_TEST_DB=postgres://career:career@localhost:55433/career go test ./internal/users/

// vec builds a 768-dim unit-ish vector pointing mostly along one axis,
// so two entries can be made near or far without hand-writing 768
// numbers.
func vec(axis int, noise float32) []float32 {
	v := make([]float32, 768)
	v[axis] = 1
	for i := range v {
		if i != axis {
			v[i] = noise
		}
	}
	return v
}

func makeEntry(t *testing.T, r *Repo, ctx context.Context, e QAEntry) int64 {
	t.Helper()
	id, err := r.CreateQAEntry(ctx, e)
	if err != nil {
		t.Fatalf("create qa entry: %v", err)
	}
	t.Cleanup(func() { _, _ = r.pool.Exec(context.Background(), `DELETE FROM qa_entries WHERE id = $1`, id) })
	return id
}

func TestQAEntryCarriesItsCanonicalPhrasing(t *testing.T) {
	r, ctx := chatRepo(t)
	id := makeEntry(t, r, ctx, QAEntry{
		Question: "Are you open to relocating?",
		Answer:   "I am, for the right role.",
		Sources:  []QASource{{Title: "About", Path: "/about"}},
		Tags:     []string{"logistics"},
		Enabled:  true,
		Phrasings: []QAPhrasing{
			{Text: "would you move?"},
			{Text: "Are you open to relocating?"}, // duplicate of the question
		},
	})

	entries, err := r.ListQAEntries(ctx, true)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var got *QAEntry
	for i := range entries {
		if entries[i].ID == id {
			got = &entries[i]
		}
	}
	if got == nil {
		t.Fatal("entry not listed")
	}
	if len(got.Sources) != 1 || got.Sources[0].Path != "/about" {
		t.Errorf("sources = %+v, want one pointing at /about", got.Sources)
	}
	// The canonical phrasing plus the one variant. The duplicate of the
	// question must not become a second row, or the bank matches the
	// same wording twice and an admin sees a phantom phrasing.
	if len(got.Phrasings) != 2 {
		t.Fatalf("phrasings = %d, want 2 (canonical + one variant): %+v", len(got.Phrasings), got.Phrasings)
	}
	if !got.Phrasings[0].Canonical || got.Phrasings[0].Text != "Are you open to relocating?" {
		t.Errorf("canonical phrasing wrong: %+v", got.Phrasings[0])
	}
	if got.Phrasings[1].Canonical {
		t.Error("a variant was marked canonical; the partial unique index should allow only one")
	}
}

// A disabled entry is a withdrawn statement. Enabling is the approval
// (FR-CHAT-06), so a disabled entry must not match even when it is the
// nearest thing in the bank by a wide margin.
func TestDisabledEntryNeverMatches(t *testing.T) {
	r, ctx := chatRepo(t)
	id := makeEntry(t, r, ctx, QAEntry{
		Question: "What are your salary expectations?",
		Answer:   "Better asked directly.",
		Enabled:  false,
	})
	phrasings, err := r.QAPhrasingsNeedingEmbedding(ctx, 100)
	if err != nil {
		t.Fatalf("needing embedding: %v", err)
	}
	var pid int64
	for _, p := range phrasings {
		if p.EntryID == id {
			pid = p.ID
		}
	}
	if pid == 0 {
		t.Fatal("the canonical phrasing was not offered for embedding")
	}
	if err := r.SetQAPhrasingEmbedding(ctx, pid, vec(3, 0)); err != nil {
		t.Fatalf("set embedding: %v", err)
	}

	// Exactly the vector that was stored: similarity 1.0, and still no
	// match, because the entry is not approved.
	if _, ok, err := r.MatchQA(ctx, vec(3, 0), 0.5); err != nil || ok {
		t.Errorf("a disabled entry matched (ok=%v, err=%v)", ok, err)
	}

	if err := r.SetQAEntryEnabled(ctx, id, true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	m, ok, err := r.MatchQA(ctx, vec(3, 0), 0.5)
	if err != nil || !ok {
		t.Fatalf("enabled entry did not match (ok=%v, err=%v)", ok, err)
	}
	if m.Entry.ID != id || m.Entry.Answer != "Better asked directly." {
		t.Errorf("matched the wrong entry: %+v", m.Entry)
	}
	if m.Similarity < 0.99 {
		t.Errorf("similarity = %.4f for an identical vector, want ~1", m.Similarity)
	}
}

// The threshold is the whole safety property of the bank: below it the
// reader gets the ordinary slow path, above it they get a verbatim
// answer that cannot hedge. A near miss must stay a miss.
func TestMatchQARespectsTheThreshold(t *testing.T) {
	r, ctx := chatRepo(t)
	id := makeEntry(t, r, ctx, QAEntry{
		Question: "Do you have PLC experience?",
		Answer:   "Thirty years of it.",
		Enabled:  true,
	})
	phrasings, _ := r.QAPhrasingsNeedingEmbedding(ctx, 100)
	for _, p := range phrasings {
		if p.EntryID == id {
			if err := r.SetQAPhrasingEmbedding(ctx, p.ID, vec(5, 0)); err != nil {
				t.Fatalf("set embedding: %v", err)
			}
		}
	}
	// A vector with a lot of noise on every other axis: close enough to
	// be the nearest row, nowhere near close enough to serve.
	far := vec(5, 0.05)
	if _, ok, err := r.MatchQA(ctx, far, 0.99); err != nil || ok {
		t.Errorf("a distant question cleared a 0.99 threshold (ok=%v, err=%v)", ok, err)
	}
	if _, ok, err := r.MatchQA(ctx, far, 0.1); err != nil || !ok {
		t.Errorf("the same question missed a 0.1 threshold (ok=%v, err=%v)", ok, err)
	}
}

// Editing the question must invalidate the canonical phrasing's vector.
// A stale vector would keep matching the wording the owner just
// replaced, which is the worst kind of bug here: the bank answers, and
// answers as though nothing changed.
func TestEditingTheQuestionClearsTheStaleVector(t *testing.T) {
	r, ctx := chatRepo(t)
	id := makeEntry(t, r, ctx, QAEntry{
		Question: "Where are you based?",
		Answer:   "Ohio.",
		Enabled:  true,
	})
	phrasings, _ := r.QAPhrasingsNeedingEmbedding(ctx, 100)
	for _, p := range phrasings {
		if p.EntryID == id {
			_ = r.SetQAPhrasingEmbedding(ctx, p.ID, vec(7, 0))
		}
	}
	if _, ok, _ := r.MatchQA(ctx, vec(7, 0), 0.5); !ok {
		t.Fatal("precondition: the entry should match before the edit")
	}

	if err := r.UpdateQAEntry(ctx, QAEntry{
		ID: id, Question: "Which time zone are you in?", Answer: "Eastern.", Enabled: true,
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, ok, _ := r.MatchQA(ctx, vec(7, 0), 0.5); ok {
		t.Error("the old wording still matches; the canonical embedding was not cleared")
	}
	again, err := r.QAPhrasingsNeedingEmbedding(ctx, 100)
	if err != nil {
		t.Fatalf("needing embedding: %v", err)
	}
	var found bool
	for _, p := range again {
		if p.EntryID == id && p.Text == "Which time zone are you in?" {
			found = true
		}
	}
	if !found {
		t.Error("the re-worded phrasing was not offered for re-embedding, so it can never match again")
	}
}

func TestDeletingAnEntryTakesItsPhrasings(t *testing.T) {
	r, ctx := chatRepo(t)
	id, err := r.CreateQAEntry(ctx, QAEntry{Question: "q", Answer: "a", Enabled: true})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := r.DeleteQAEntry(ctx, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var n int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM qa_phrasings WHERE entry_id = $1`, id).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("%d orphaned phrasings survived the entry", n)
	}
	if err := r.DeleteQAEntry(ctx, id); !errors.Is(err, ErrQAEntryNotFound) {
		t.Errorf("second delete: err = %v, want ErrQAEntryNotFound", err)
	}
}

// The canonical phrasing is the entry's own question. Deleting it would
// leave an entry reachable only through its variants, or not at all.
func TestCanonicalPhrasingCannotBeDeleted(t *testing.T) {
	r, ctx := chatRepo(t)
	id := makeEntry(t, r, ctx, QAEntry{Question: "q", Answer: "a", Enabled: true})
	variant, err := r.AddQAPhrasing(ctx, id, "another way of asking q")
	if err != nil {
		t.Fatalf("add phrasing: %v", err)
	}
	entries, _ := r.ListQAEntries(ctx, true)
	var canonical int64
	for _, e := range entries {
		if e.ID != id {
			continue
		}
		for _, p := range e.Phrasings {
			if p.Canonical {
				canonical = p.ID
			}
		}
	}
	if err := r.DeleteQAPhrasing(ctx, id, canonical); !errors.Is(err, ErrQAEntryNotFound) {
		t.Errorf("deleting the canonical phrasing: err = %v, want refused", err)
	}
	if err := r.DeleteQAPhrasing(ctx, id, variant); err != nil {
		t.Errorf("deleting a variant should be allowed: %v", err)
	}
}
