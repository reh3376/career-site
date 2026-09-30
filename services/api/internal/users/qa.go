package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/reh3376/career-site/services/api/internal/tenant"
)

// The Q&A bank (migration 00047).
//
// An entry is the owner's own answer, written in advance and served
// word for word. Nothing here paraphrases, summarises or rewrites one,
// and no model ever sees an entry's text: that is the property that
// makes the bank usable for the topics the persona is otherwise
// forbidden to touch (FR-CHAT-06), and it would be lost the moment an
// answer went through generation "just to fit the voice".

// QAEntry is one approved answer.
type QAEntry struct {
	ID       int64
	Question string
	Answer   string
	Sources  []QASource
	Tags     []string

	// CoversRestricted marks an entry that deliberately answers a topic
	// the persona refuses by default. Recorded so the owner can list
	// every such statement he has made.
	CoversRestricted bool

	// Enabled is the approval. A disabled entry never matches.
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time

	// Phrasings are the ways of asking it. Only populated by the calls
	// that need them.
	Phrasings []QAPhrasing
}

// QASource is a link shown beneath a bank answer (FR-CHAT-04).
type QASource struct {
	Title string `json:"title"`
	Path  string `json:"path"`
}

// QAPhrasing is one way of asking an entry's question.
type QAPhrasing struct {
	ID        int64
	EntryID   int64
	Text      string
	Canonical bool
	HasVector bool
}

// QAMatch is a bank hit.
type QAMatch struct {
	Entry      QAEntry
	Phrasing   string
	Similarity float64
}

// ErrQAEntryNotFound is returned for a missing or other-tenant entry.
var ErrQAEntryNotFound = errors.New("no such Q&A entry")

// QAMatchThreshold is the cosine similarity at which a question is
// treated as already answered in the bank.
//
// Set high on purpose, and the asymmetry is the whole argument. A miss
// costs the reader the ordinary path: retrieval, the model, and the 10
// to 25 seconds this hardware takes to reach a first token. A false
// match costs them a confident, verbatim, owner-signed answer to a
// question they did not ask, with no hedge anywhere in it, because a
// bank answer is not generated and therefore cannot qualify itself.
// The first failure is slow and the second is wrong.
//
// Measured on production against nomic-embed-text on 2026-09-30, with
// one banked question and seven probes (disabled entries, deleted
// afterwards, never servable):
//
//	1.0000  Are you open to relocating?        (the anchor)
//	0.8824  are you willing to relocate        paraphrase
//	0.7941  would you move?                    paraphrase
//	0.7627  can you relocate for this role     paraphrase
//	0.5453  where are you based?               same topic, different question
//	0.4351  what PLC platforms have you used?  unrelated
//	0.4248  what is your favourite pizza       unrelated
//	0.3657  do you know Rockwell ControlLogix  unrelated
//
// That killed the first guess. 0.85 caught only the closest paraphrase
// and would have sent "would you move?" to the model for fifteen to
// twenty-five seconds, which is the exact wait the bank exists to
// avoid, on a question it already had an answer to.
//
// The useful finding is the gap: real paraphrases bottom out around
// 0.76 and the nearest non-match sits at 0.55, so anything in 0.6 to
// 0.75 separates them. 0.72 takes all three paraphrases with 0.04 to
// spare and clears the nearest non-match by 0.17, keeping most of the
// margin on the side where a mistake is expensive.
//
// STILL PROVISIONAL. This is one question family, and a threshold
// generalised from one anchor is a guess with better manners. It is
// changed anyway because 0.85 was demonstrably wrong rather than
// merely unverified. FR-CHAT-15's golden set is what should settle it
// across many questions; see docs/ask-roger.md.
const QAMatchThreshold = 0.72

// MatchQA returns the best bank entry for an embedded question, or
// false if nothing clears the threshold.
//
// One query over qa_phrasings, and no model call at all. This is the
// path that makes a common question instant on a box where generation
// is not.
func (r *Repo) MatchQA(ctx context.Context, embedding []float32, threshold float64) (QAMatch, bool, error) {
	if len(embedding) == 0 {
		return QAMatch{}, false, fmt.Errorf("empty embedding")
	}
	if threshold <= 0 {
		threshold = QAMatchThreshold
	}

	// Ordering by distance and filtering on similarity afterwards, so
	// the HNSW index still drives the scan. A WHERE on the computed
	// similarity would not.
	const q = `
    SELECT e.id, e.question, e.answer, e.sources, e.tags,
           e.covers_restricted, e.enabled, e.created_at, e.updated_at,
           p.text, 1 - (p.embedding <=> $1) AS similarity
      FROM qa_phrasings p
      JOIN qa_entries e ON e.id = p.entry_id
     WHERE p.embedding IS NOT NULL
       AND e.enabled
       AND e.tenant_id = $2
     ORDER BY p.embedding <=> $1
     LIMIT 1`

	var m QAMatch
	var sources []byte
	err := r.pool.QueryRow(ctx, q, vectorLiteral(embedding), tenant.FromContext(ctx).Int64()).Scan(
		&m.Entry.ID, &m.Entry.Question, &m.Entry.Answer, &sources, &m.Entry.Tags,
		&m.Entry.CoversRestricted, &m.Entry.Enabled, &m.Entry.CreatedAt, &m.Entry.UpdatedAt,
		&m.Phrasing, &m.Similarity)
	if errors.Is(err, pgx.ErrNoRows) {
		return QAMatch{}, false, nil
	}
	if err != nil {
		return QAMatch{}, false, fmt.Errorf("match qa: %w", err)
	}
	if m.Similarity < threshold {
		return QAMatch{}, false, nil
	}
	if err := json.Unmarshal(sources, &m.Entry.Sources); err != nil {
		return QAMatch{}, false, fmt.Errorf("decode qa sources: %w", err)
	}
	return m, true, nil
}

// CreateQAEntry writes an entry and its canonical phrasing.
//
// One transaction: an entry whose canonical phrasing did not land is an
// answer that can never be matched, which looks like the bank quietly
// not working rather than like a failure.
func (r *Repo) CreateQAEntry(ctx context.Context, e QAEntry) (int64, error) {
	sources, err := json.Marshal(nonNilSources(e.Sources))
	if err != nil {
		return 0, fmt.Errorf("encode qa sources: %w", err)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("create qa entry: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id int64
	if err := tx.QueryRow(ctx, `
    INSERT INTO qa_entries
      (tenant_id, question, answer, sources, tags, covers_restricted, enabled)
    VALUES ($1, $2, $3, $4, $5, $6, $7)
    RETURNING id`,
		tenant.FromContext(ctx).Int64(), e.Question, e.Answer, sources,
		nonNilTags(e.Tags), e.CoversRestricted, e.Enabled).Scan(&id); err != nil {
		return 0, fmt.Errorf("insert qa entry: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO qa_phrasings (entry_id, text, canonical) VALUES ($1, $2, true)`,
		id, e.Question); err != nil {
		return 0, fmt.Errorf("insert canonical phrasing: %w", err)
	}
	// Extra phrasings supplied at creation time. The canonical one is
	// already in, so skip a duplicate of it.
	for _, p := range e.Phrasings {
		if p.Text == "" || p.Text == e.Question {
			continue
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO qa_phrasings (entry_id, text, canonical) VALUES ($1, $2, false)`,
			id, p.Text); err != nil {
			return 0, fmt.Errorf("insert phrasing: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("create qa entry: %w", err)
	}
	return id, nil
}

// UpdateQAEntry replaces an entry's editable fields.
//
// Editing the question rewrites the canonical phrasing and clears its
// embedding, because a phrasing whose text changed under a stale vector
// would keep matching the old wording. The re-embed job picks it up;
// until it does the entry matches on its other phrasings, or not at
// all, which is the right way round.
func (r *Repo) UpdateQAEntry(ctx context.Context, e QAEntry) error {
	sources, err := json.Marshal(nonNilSources(e.Sources))
	if err != nil {
		return fmt.Errorf("encode qa sources: %w", err)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("update qa entry: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
    UPDATE qa_entries
       SET question = $3, answer = $4, sources = $5, tags = $6,
           covers_restricted = $7, enabled = $8, updated_at = now()
     WHERE tenant_id = $1 AND id = $2`,
		tenant.FromContext(ctx).Int64(), e.ID, e.Question, e.Answer, sources,
		nonNilTags(e.Tags), e.CoversRestricted, e.Enabled)
	if err != nil {
		return fmt.Errorf("update qa entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrQAEntryNotFound
	}
	if _, err := tx.Exec(ctx, `
    UPDATE qa_phrasings SET text = $2, embedding = NULL
     WHERE entry_id = $1 AND canonical AND text IS DISTINCT FROM $2`,
		e.ID, e.Question); err != nil {
		return fmt.Errorf("update canonical phrasing: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("update qa entry: %w", err)
	}
	return nil
}

// SetQAEntryEnabled approves or withdraws an entry.
func (r *Repo) SetQAEntryEnabled(ctx context.Context, id int64, enabled bool) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE qa_entries SET enabled = $3, updated_at = now() WHERE tenant_id = $1 AND id = $2`,
		tenant.FromContext(ctx).Int64(), id, enabled)
	if err != nil {
		return fmt.Errorf("set qa entry enabled: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrQAEntryNotFound
	}
	return nil
}

// DeleteQAEntry removes an entry and its phrasings.
func (r *Repo) DeleteQAEntry(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM qa_entries WHERE tenant_id = $1 AND id = $2`,
		tenant.FromContext(ctx).Int64(), id)
	if err != nil {
		return fmt.Errorf("delete qa entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrQAEntryNotFound
	}
	return nil
}

// AddQAPhrasing adds another way of asking an existing entry.
func (r *Repo) AddQAPhrasing(ctx context.Context, entryID int64, text string) (int64, error) {
	// Scoped through the entry so a phrasing cannot be attached to
	// another tenant's answer.
	var id int64
	err := r.pool.QueryRow(ctx, `
    INSERT INTO qa_phrasings (entry_id, text)
    SELECT e.id, $3 FROM qa_entries e WHERE e.id = $2 AND e.tenant_id = $1
    RETURNING id`,
		tenant.FromContext(ctx).Int64(), entryID, text).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrQAEntryNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("add qa phrasing: %w", err)
	}
	return id, nil
}

// DeleteQAPhrasing removes one phrasing. The canonical one stays: it is
// the entry's own question and deleting it would leave the entry
// matchable only by its variants.
func (r *Repo) DeleteQAPhrasing(ctx context.Context, entryID, phrasingID int64) error {
	tag, err := r.pool.Exec(ctx, `
    DELETE FROM qa_phrasings p
     USING qa_entries e
     WHERE p.id = $3 AND p.entry_id = $2 AND e.id = p.entry_id
       AND e.tenant_id = $1 AND NOT p.canonical`,
		tenant.FromContext(ctx).Int64(), entryID, phrasingID)
	if err != nil {
		return fmt.Errorf("delete qa phrasing: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrQAEntryNotFound
	}
	return nil
}

// ListQAEntries returns the bank for the admin surface, newest first.
// includeDisabled is false for anything member-facing.
func (r *Repo) ListQAEntries(ctx context.Context, includeDisabled bool) ([]QAEntry, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT id, question, answer, sources, tags, covers_restricted, enabled,
           created_at, updated_at
      FROM qa_entries
     WHERE tenant_id = $1 AND ($2 OR enabled)
     ORDER BY id DESC`,
		tenant.FromContext(ctx).Int64(), includeDisabled)
	if err != nil {
		return nil, fmt.Errorf("list qa entries: %w", err)
	}
	defer rows.Close()

	var out []QAEntry
	byID := map[int64]int{}
	for rows.Next() {
		var e QAEntry
		var sources []byte
		if err := rows.Scan(&e.ID, &e.Question, &e.Answer, &sources, &e.Tags,
			&e.CoversRestricted, &e.Enabled, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan qa entry: %w", err)
		}
		if err := json.Unmarshal(sources, &e.Sources); err != nil {
			return nil, fmt.Errorf("decode qa sources: %w", err)
		}
		byID[e.ID] = len(out)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}

	// Phrasings in one pass rather than per entry, for the same reason
	// citations are fetched that way in chat.go: the admin page shows
	// all of them and a query per row is a query per row.
	prows, err := r.pool.Query(ctx, `
    SELECT p.id, p.entry_id, p.text, p.canonical, p.embedding IS NOT NULL
      FROM qa_phrasings p
      JOIN qa_entries e ON e.id = p.entry_id
     WHERE e.tenant_id = $1
     ORDER BY p.entry_id, p.canonical DESC, p.id`,
		tenant.FromContext(ctx).Int64())
	if err != nil {
		return nil, fmt.Errorf("list qa phrasings: %w", err)
	}
	defer prows.Close()
	for prows.Next() {
		var p QAPhrasing
		if err := prows.Scan(&p.ID, &p.EntryID, &p.Text, &p.Canonical, &p.HasVector); err != nil {
			return nil, fmt.Errorf("scan qa phrasing: %w", err)
		}
		if i, ok := byID[p.EntryID]; ok {
			out[i].Phrasings = append(out[i].Phrasings, p)
		}
	}
	return out, prows.Err()
}

// QAPhrasingsNeedingEmbedding returns phrasings with no vector yet, for
// the embed job. A phrasing without one never matches, so this is the
// difference between an entry being in the bank and being reachable.
func (r *Repo) QAPhrasingsNeedingEmbedding(ctx context.Context, limit int) ([]QAPhrasing, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
    SELECT p.id, p.entry_id, p.text, p.canonical
      FROM qa_phrasings p
      JOIN qa_entries e ON e.id = p.entry_id
     WHERE p.embedding IS NULL AND e.tenant_id = $1
     ORDER BY p.id
     LIMIT $2`, tenant.FromContext(ctx).Int64(), limit)
	if err != nil {
		return nil, fmt.Errorf("list unembedded phrasings: %w", err)
	}
	defer rows.Close()

	var out []QAPhrasing
	for rows.Next() {
		var p QAPhrasing
		if err := rows.Scan(&p.ID, &p.EntryID, &p.Text, &p.Canonical); err != nil {
			return nil, fmt.Errorf("scan phrasing: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SetQAPhrasingEmbedding stores a phrasing's vector.
func (r *Repo) SetQAPhrasingEmbedding(ctx context.Context, phrasingID int64, embedding []float32) error {
	if len(embedding) == 0 {
		return fmt.Errorf("empty embedding")
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE qa_phrasings SET embedding = $2 WHERE id = $1`,
		phrasingID, vectorLiteral(embedding))
	if err != nil {
		return fmt.Errorf("set phrasing embedding: %w", err)
	}
	return nil
}

// nonNilSources keeps an empty list encoding as [] rather than null, so
// the column matches its own default.
func nonNilSources(s []QASource) []QASource {
	if s == nil {
		return []QASource{}
	}
	return s
}

func nonNilTags(t []string) []string {
	if t == nil {
		return []string{}
	}
	return t
}
