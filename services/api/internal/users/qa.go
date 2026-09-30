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
// Measured twice on production against nomic-embed-text, and the
// second measurement overturned the first.
//
// The first probe used one question and its paraphrases and looked
// clean: paraphrases 0.76 to 0.88, a different question on the same
// topic at 0.55, unrelated questions below 0.44. That suggested 0.72.
// It was wrong, and it was wrong because one question family cannot
// show you a cross-entry collision.
//
// The second probe, over six real entries and thirty phrasings, found
// the bands overlap completely:
//
//	0.766  "do you have leadership experience"
//	       against "How much experience do you have?"     DIFFERENT entries
//	0.605  "do you have a masters"
//	       against "What is your education?"              SAME entry
//	0.572  "how many people have you managed"
//	       against "How large were the teams ...?"        SAME entry
//	0.475  "do you know Ignition"
//	       against "What automation platforms ...?"       SAME entry
//
// The model is scoring shared vocabulary far more than shared intent
// on strings this short. "experience" in both sides carries 0.766
// between two unrelated questions, while "Ignition" and "automation
// platforms" share no words and score 0.475 despite being the same
// question. No single threshold separates those two lists, so tuning
// the number is the wrong move and this stopped being a threshold
// problem.
//
// What follows, and it is a change of mechanism rather than of value:
//
//   - The threshold goes back up, to sit clear of the worst observed
//     cross-entry pair at 0.766. Above it, a match means the wording
//     is genuinely close to something the owner wrote down.
//   - Coverage comes from listing phrasings, not from semantic reach.
//     The bank is an owner-curated near-exact lookup. A question asked
//     in words nobody anticipated falls through to the model, which is
//     slow and correct, rather than matching the wrong entry, which is
//     fast and wrong.
//   - A margin rule backs it up: see QAMatchMargin.
//
// Still provisional in the sense that six entries is not a golden set
// (FR-CHAT-15), but no longer provisional about the shape of the
// problem.
const QAMatchThreshold = 0.85

// QAMatchMargin is how far the winner must beat the best candidate
// from a *different* entry.
//
// The threshold alone answers "is this close enough to something the
// owner wrote". It cannot answer "and is it clearly closer to this
// entry than to another one", which is the failure the second probe
// found: two entries both plausible, one picked on a hair, and the
// reader handed a confident verbatim answer to a question they did not
// ask.
//
// Scale-free on purpose. A margin survives the absolute similarities
// drifting when the embedding model changes, which a threshold does
// not, so this is the part expected to age well.
//
// Zero disables the check, which is what a caller passing an explicit
// threshold of its own gets unless it opts in.
const QAMatchMargin = 0.05

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
     LIMIT 8`

	rows, err := r.pool.Query(ctx, q, vectorLiteral(embedding), tenant.FromContext(ctx).Int64())
	if err != nil {
		return QAMatch{}, false, fmt.Errorf("match qa: %w", err)
	}
	defer rows.Close()

	// The top few rather than the top one, so the margin below can be
	// computed. Eight is enough to reach a different entry in a bank of
	// any realistic size, and the rows are tiny.
	var best QAMatch
	var bestSources []byte
	var runnerUp float64
	var have bool
	for rows.Next() {
		var m QAMatch
		var sources []byte
		if err := rows.Scan(
			&m.Entry.ID, &m.Entry.Question, &m.Entry.Answer, &sources, &m.Entry.Tags,
			&m.Entry.CoversRestricted, &m.Entry.Enabled, &m.Entry.CreatedAt, &m.Entry.UpdatedAt,
			&m.Phrasing, &m.Similarity); err != nil {
			return QAMatch{}, false, fmt.Errorf("scan qa match: %w", err)
		}
		if !have {
			best, bestSources, have = m, sources, true
			continue
		}
		// The first row belonging to a different entry is the one the
		// winner has to beat.
		if m.Entry.ID != best.Entry.ID && runnerUp == 0 {
			runnerUp = m.Similarity
		}
	}
	if err := rows.Err(); err != nil {
		return QAMatch{}, false, fmt.Errorf("match qa: %w", err)
	}
	if !have {
		return QAMatch{}, false, nil
	}
	m, sources := best, bestSources

	if m.Similarity < threshold {
		return QAMatch{}, false, nil
	}
	// Close enough to two entries is not a match. Measured on
	// production, two unrelated questions sharing the word "experience"
	// scored 0.766 against each other, so "clears the bar" and "is
	// clearly this one" are different questions and both have to be
	// answered before an owner-signed answer is served.
	if runnerUp > 0 && m.Similarity-runnerUp < QAMatchMargin {
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
