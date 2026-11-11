package hiring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/reh3376/career-site/services/api/internal/tenant"
)

// Repo writes and reads the hiring tables. One writer per table, each
// taking ctx and reading the tenant from it, in the style of
// users/jd_runs.go. It holds the same pool as users.Repo and is built
// beside it in main.
type Repo struct {
	pool *pgxpool.Pool
}

// NewRepo wraps the pool.
func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// ErrNotFound is returned by the point reads when no row matches.
var ErrNotFound = errors.New("hiring: not found")

// EnsureRole returns the role for a submission, creating it on first
// sight. In Phase 0 every role is a submitted posting (source
// 'posting'); the title is refreshed on each call because the hint it
// comes from can change between runs and the row should say what the
// latest run saw.
func (r *Repo) EnsureRole(ctx context.Context, submissionID int64, title string) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
    INSERT INTO roles (tenant_id, source, submission_id, title)
    VALUES ($1, 'posting', $2, $3)
    ON CONFLICT (submission_id) DO UPDATE SET title = EXCLUDED.title
    RETURNING id::text`,
		tenant.FromContext(ctx).Int64(), submissionID, title).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("ensure role: %w", err)
	}
	return id, nil
}

// RequirementInput is one extracted requirement as the pipeline hands
// it over: the prototype's fields plus the type the heuristic set and
// the extraction provenance.
type RequirementInput struct {
	LocalID      string
	Text         string
	SourceQuote  string
	Category     string // must | nice
	Weight       int
	NamedParties []string
	Type         string          // a RequirementTypes code
	Extraction   json.RawMessage // {prompt_id, prompt_version, prompt_fingerprint, model, type_rule}
}

// RequirementRow is the version of a requirement a run was judged
// against: its row id and version, by local id.
type RequirementRow struct {
	ID      string
	LocalID string
	Version int
	Text    string
}

// requirementKey is what makes two extractions of the same local id
// the same version: text, quote, category, weight and parties. The
// type is excluded on purpose, so a change to the type heuristic does
// not mint a new version of every requirement in the database.
func (in RequirementInput) key() string {
	// nil and empty are the same absence: the row stores {} and reads
	// back as an empty slice, while the extractor may hand over nil, and
	// the two must not mint a new version on every run.
	parties := slices.Clone(in.NamedParties)
	if parties == nil {
		parties = []string{}
	}
	slices.Sort(parties)
	b, _ := json.Marshal([]any{in.Text, in.SourceQuote, in.Category, in.Weight, parties})
	return string(b)
}

// SyncRequirementVersions makes the role's active requirement rows
// match this run's extraction, version by version (FR-HF-01): a local
// id whose text, quote, category, weight and parties are unchanged
// keeps its row; one that changed gets a new row with supersedes set
// and the old one retired; one the extractor no longer produces is
// retired. Nothing is rewritten in place, so a judgment's
// requirement_id always names exactly what was judged. Returns the
// rows this run should cite, in input order.
func (r *Repo) SyncRequirementVersions(ctx context.Context, roleID string, in []RequirementInput) ([]RequirementRow, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("sync requirements: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	type existing struct {
		id      string
		version int
		text    string
		key     string
	}
	rows, err := tx.Query(ctx, `
    SELECT id::text, local_id, version, text, source_quote, category, weight, named_parties
    FROM requirements
    WHERE role_id = $1::uuid AND status = 'active'
    FOR UPDATE`, roleID)
	if err != nil {
		return nil, fmt.Errorf("sync requirements: load: %w", err)
	}
	active := map[string]existing{}
	for rows.Next() {
		var e existing
		var localID string
		var cur RequirementInput
		if err := rows.Scan(&e.id, &localID, &e.version, &cur.Text, &cur.SourceQuote, &cur.Category, &cur.Weight, &cur.NamedParties); err != nil {
			rows.Close()
			return nil, fmt.Errorf("sync requirements: scan: %w", err)
		}
		e.text = cur.Text
		e.key = cur.key()
		active[localID] = e
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	tenantID := tenant.FromContext(ctx).Int64()
	out := make([]RequirementRow, 0, len(in))
	seen := map[string]bool{}
	for _, req := range in {
		if req.LocalID == "" || seen[req.LocalID] {
			continue
		}
		seen[req.LocalID] = true
		if e, ok := active[req.LocalID]; ok && e.key == req.key() {
			out = append(out, RequirementRow{ID: e.id, LocalID: req.LocalID, Version: e.version, Text: e.text})
			continue
		}
		// A new version: retire the old row (if any) and insert the next.
		var supersedes *string
		if e, ok := active[req.LocalID]; ok {
			if _, err := tx.Exec(ctx, `UPDATE requirements SET status = 'retired' WHERE id = $1::uuid`, e.id); err != nil {
				return nil, fmt.Errorf("sync requirements: retire %s: %w", req.LocalID, err)
			}
			supersedes = &e.id
		}
		var version int
		if err := tx.QueryRow(ctx, `
      SELECT COALESCE(MAX(version), 0) + 1 FROM requirements WHERE role_id = $1::uuid AND local_id = $2`,
			roleID, req.LocalID).Scan(&version); err != nil {
			return nil, fmt.Errorf("sync requirements: next version: %w", err)
		}
		weight := req.Weight
		if weight < 1 || weight > 3 {
			weight = 2
		}
		category := req.Category
		if category != "must" {
			category = "nice"
		}
		typ := req.Type
		if !slices.Contains(RequirementTypes, typ) {
			typ = "skill"
		}
		parties := req.NamedParties
		if parties == nil {
			parties = []string{}
		}
		extraction := req.Extraction
		if len(extraction) == 0 {
			extraction = nil
		}
		var id string
		if err := tx.QueryRow(ctx, `
      INSERT INTO requirements
        (tenant_id, role_id, local_id, version, supersedes, schema_version, vocab_version,
         text, source_quote, type, category, weight, named_parties, extraction, status)
      VALUES ($1, $2::uuid, $3, $4, $5::uuid, $6, $7, $8, $9, $10, $11, $12, $13, $14, 'active')
      RETURNING id::text`,
			tenantID, roleID, req.LocalID, version, supersedes, RequirementSchema, VocabVersion,
			req.Text, req.SourceQuote, typ, category, weight, parties, extraction).Scan(&id); err != nil {
			return nil, fmt.Errorf("sync requirements: insert %s: %w", req.LocalID, err)
		}
		out = append(out, RequirementRow{ID: id, LocalID: req.LocalID, Version: version, Text: req.Text})
	}
	// Local ids the extractor no longer produces are retired: they are
	// not part of what this run judged, and a later run that produces
	// them again starts a new version chain from the retired row.
	for localID, e := range active {
		if !seen[localID] {
			if _, err := tx.Exec(ctx, `UPDATE requirements SET status = 'retired' WHERE id = $1::uuid`, e.id); err != nil {
				return nil, fmt.Errorf("sync requirements: retire absent %s: %w", localID, err)
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("sync requirements: commit: %w", err)
	}
	return out, nil
}

// EnsureChunkEvidence creates the evidence item for each cited corpus
// chunk that does not have one yet (FR-HF-09; lazily, 14 §6 point 2)
// and returns chunk id to evidence item id. An existing item has its
// text and visibility refreshed from the chunk, because a chunk keeps
// its id when its document is re-ingested and the item should read as
// the chunk does now; the candidate is the document's. The profile
// sheet's chunks become profile_sheet items, everything else
// corpus_document.
func (r *Repo) EnsureChunkEvidence(ctx context.Context, chunkIDs []int64) (map[int64]EvidenceRef, error) {
	out := map[int64]EvidenceRef{}
	if len(chunkIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
    INSERT INTO evidence_items (tenant_id, candidate_id, source_kind, chunk_id, text, visibility, schema_version)
    SELECT $1, d.candidate_id,
           CASE WHEN d.source_kind = 'profile' THEN 'profile_sheet' ELSE 'corpus_document' END,
           c.id, c.text,
           CASE WHEN d.visibility = 'public' THEN 'public' ELSE 'private' END,
           $3
    FROM corpus_chunks c
    JOIN corpus_documents d ON d.id = c.document_id
    WHERE c.id = ANY($2)
    ON CONFLICT (chunk_id) DO UPDATE
      SET text = EXCLUDED.text, visibility = EXCLUDED.visibility
    RETURNING chunk_id, id::text, source_kind`,
		tenant.FromContext(ctx).Int64(), chunkIDs, EvidenceSchema)
	if err != nil {
		return nil, fmt.Errorf("ensure chunk evidence: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var chunkID int64
		var ref EvidenceRef
		if err := rows.Scan(&chunkID, &ref.ID, &ref.SourceKind); err != nil {
			return nil, fmt.Errorf("ensure chunk evidence: scan: %w", err)
		}
		out[chunkID] = ref
	}
	return out, rows.Err()
}

// EvidenceRef is an evidence item's row id and source kind, by the
// chunk it was made from.
type EvidenceRef struct {
	ID         string
	SourceKind string
}

// JudgmentRow is one row of judgments as the writer stores it
// (FR-HF-04): the model's answer after the code rules, its provenance,
// and the derived verdict (FR-HF-07).
type JudgmentRow struct {
	RequirementID     string // requirements.id, the exact version judged
	LocalID           string
	Determination     string
	Basis             string
	Uncertainty       []string
	EvidenceIDs       []string // evidence_items ids
	Rationale         string
	ReportedFacts     json.RawMessage
	Confidence        *float64
	RuleAdjustments   json.RawMessage
	RationaleIssues   json.RawMessage
	ProvenanceSummary json.RawMessage
	DerivedVerdict    string
	DecisionID        *int64
	PromptID          string
	PromptVersion     int
	PromptFingerprint string
	Model             string
}

// ReplaceJudgments writes a run's judgments in one transaction,
// removing any earlier rows for the run first, so a retried persist of
// the same run is one set of rows. Codes are written as given; the
// registry check lives in TestWrittenCodesAreRegistered rather than a
// CHECK constraint (14 §1).
func (r *Repo) ReplaceJudgments(ctx context.Context, runID string, rows []JudgmentRow) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("judgments: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM judgments WHERE run_id = $1::uuid`, runID); err != nil {
		return fmt.Errorf("judgments: clear: %w", err)
	}
	tid := tenant.FromContext(ctx).Int64()
	for _, j := range rows {
		unc := j.Uncertainty
		if unc == nil {
			unc = []string{}
		}
		ev := j.EvidenceIDs
		if ev == nil {
			ev = []string{}
		}
		if _, err := tx.Exec(ctx, `
      INSERT INTO judgments
        (tenant_id, run_id, requirement_id, requirement_local_id, candidate_id, schema_version, vocab_version,
         determination, basis, uncertainty, evidence_ids, rationale, reported_facts, confidence,
         rule_adjustments, rationale_issues, provenance_summary, derived_verdict,
         decision_id, prompt_id, prompt_version, prompt_fingerprint, model)
      VALUES ($1, $2::uuid, $3::uuid, $4, $5::uuid, $6, $7,
              $8, $9, $10, $11::uuid[], $12, $13, $14,
              $15, $16, $17, $18,
              $19, $20, $21, $22, $23)`,
			tid, runID, j.RequirementID, j.LocalID, OwnerCandidateID, JudgmentSchema, VocabVersion,
			j.Determination, j.Basis, unc, ev, j.Rationale, orEmptyObject(j.ReportedFacts), j.Confidence,
			orEmptyArray(j.RuleAdjustments), orEmptyArray(j.RationaleIssues), orEmptyObject(j.ProvenanceSummary), j.DerivedVerdict,
			j.DecisionID, j.PromptID, j.PromptVersion, j.PromptFingerprint, j.Model,
		); err != nil {
			return fmt.Errorf("judgments: insert %s: %w", j.LocalID, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("judgments: commit: %w", err)
	}
	return nil
}

func orEmptyObject(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}

func orEmptyArray(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`[]`)
	}
	return raw
}

// MatchRecord is one row of match_records. Pointers are the numbers a
// formula has not produced yet: during the dual write only Evidenced
// is set (to the v2 score) and the rest stay null (14 §4).
type MatchRecord struct {
	RunID             string
	RoleID            string
	CandidateID       string
	Attempt           int
	SupersedesRunID   string // "" for the first attempt
	RequirementIDs    []string
	FormulaVersion    string
	Evidenced         *float64
	Unresolved        *float64
	Unmet             *float64
	EquivalentShare   *float64
	SelfAttestedShare *float64
	KeywordBaseline   json.RawMessage
	Versions          json.RawMessage
}

// UpsertMatchRecord writes the record for a run. Upsert on run_id so a
// retried persist of the same run is one row, not an error.
func (r *Repo) UpsertMatchRecord(ctx context.Context, m MatchRecord) error {
	var supersedes *string
	if m.SupersedesRunID != "" {
		supersedes = &m.SupersedesRunID
	}
	kb := m.KeywordBaseline
	if len(kb) == 0 {
		kb = json.RawMessage(`{}`)
	}
	versions := m.Versions
	if len(versions) == 0 {
		versions = json.RawMessage(`{}`)
	}
	ids := m.RequirementIDs
	if ids == nil {
		ids = []string{}
	}
	_, err := r.pool.Exec(ctx, `
    INSERT INTO match_records
      (run_id, tenant_id, role_id, candidate_id, attempt, supersedes_run_id, requirement_ids,
       formula_version, evidenced, unresolved, unmet, coverage_equivalent_share, self_attested_share,
       keyword_baseline, versions, schema_version)
    VALUES ($1::uuid, $2, $3::uuid, $4::uuid, $5, $6::uuid, $7::uuid[],
            $8, $9, $10, $11, $12, $13, $14, $15, $16)
    ON CONFLICT (run_id) DO UPDATE SET
      role_id = EXCLUDED.role_id, attempt = EXCLUDED.attempt, supersedes_run_id = EXCLUDED.supersedes_run_id,
      requirement_ids = EXCLUDED.requirement_ids, formula_version = EXCLUDED.formula_version,
      evidenced = EXCLUDED.evidenced, unresolved = EXCLUDED.unresolved, unmet = EXCLUDED.unmet,
      coverage_equivalent_share = EXCLUDED.coverage_equivalent_share, self_attested_share = EXCLUDED.self_attested_share,
      keyword_baseline = EXCLUDED.keyword_baseline, versions = EXCLUDED.versions`,
		m.RunID, tenant.FromContext(ctx).Int64(), m.RoleID, m.CandidateID, m.Attempt, supersedes, ids,
		m.FormulaVersion, m.Evidenced, m.Unresolved, m.Unmet, m.EquivalentShare, m.SelfAttestedShare,
		kb, versions, MatchRecordSchema)
	if err != nil {
		return fmt.Errorf("upsert match record: %w", err)
	}
	return nil
}

// PreviousRunID is the run this attempt supersedes: the latest earlier
// attempt for the same submission, or "" for the first.
func (r *Repo) PreviousRunID(ctx context.Context, submissionID int64, attempt int) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
    SELECT run_id::text FROM jd_runs
    WHERE submission_id = $1 AND attempt < $2
    ORDER BY attempt DESC LIMIT 1`, submissionID, attempt).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("previous run: %w", err)
	}
	return id, nil
}

// ActiveRequirements lists a role's active requirement rows by local
// id, for tests and for the viewer at step 6.
func (r *Repo) ActiveRequirements(ctx context.Context, roleID string) ([]RequirementRow, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT id::text, local_id, version, text FROM requirements
    WHERE role_id = $1::uuid AND status = 'active' ORDER BY local_id`, roleID)
	if err != nil {
		return nil, fmt.Errorf("active requirements: %w", err)
	}
	defer rows.Close()
	var out []RequirementRow
	for rows.Next() {
		var q RequirementRow
		if err := rows.Scan(&q.ID, &q.LocalID, &q.Version, &q.Text); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}
