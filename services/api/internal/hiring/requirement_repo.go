package hiring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/reh3376/career-site/services/api/internal/tenant"
)

// RequirementInput is one extracted requirement as the extraction
// stage hands it over: what the model produced, validated. Everything
// else on a draft is computed from these (Draft).
type RequirementInput struct {
	LocalID      string
	Text         string
	SourceQuote  string
	Category     string // must | nice
	Weight       int
	NamedParties []string
	// Type is the extractor's type (FR-HF-02); empty when the extractor
	// did not type it, in which case the heuristic does.
	Type string
	// Alternatives are the options the posting itself offers for this
	// requirement, split out in the order written.
	Alternatives []string
}

// Provenance says where a draft came from, for the row's `extraction`.
type Provenance struct {
	PromptID          string
	PromptFingerprint string
	Model             string
}

// Draft turns an extracted requirement into the row the pipeline
// writes: typed, classed, with its thresholds read from the text, its
// evidence standard rendered from the type's template, its equivalence
// notes from the catalog and whether it counts decided (FR-HF-02).
//
// The type is the extractor's, with one exception that only ever makes
// the system more cautious: a requirement either the extractor or the
// heuristic reads as behavioral is behavioral, because a working-style
// line that reaches the judge is the opaque culture-fit judgment
// FR-HF-18 exists to prevent, and the cost of a false behavioral is one
// row a person retypes.
func Draft(in RequirementInput, catalog *Catalog, prov Provenance) Requirement {
	heuristic := InferType(in.Text, in.NamedParties)
	typ, source := in.Type, "model"
	switch {
	case !slices.Contains(RequirementTypes, typ):
		typ, source = heuristic, TypeRule
	case typ != "behavioral" && heuristic == "behavioral":
		typ, source = "behavioral", "model+"+TypeRule
	}
	q := Requirement{
		LocalID:       in.LocalID,
		SchemaVersion: RequirementSchema,
		VocabVersion:  VocabVersion,
		Text:          strings.TrimSpace(in.Text),
		SourceQuote:   strings.TrimSpace(in.SourceQuote),
		Type:          typ,
		Category:      in.Category,
		Weight:        in.Weight,
		NamedParties:  in.NamedParties,
		Status:        StatusDraft,
	}
	if q.Category != "must" {
		q.Category = "nice"
	}
	if q.Weight < 1 || q.Weight > 3 {
		q.Weight = 2
	}
	if q.NamedParties == nil {
		q.NamedParties = []string{}
	}
	q.Class = ClassFromCategory(q.Category)
	if alts, err := cleanList(in.Alternatives, MaxAlternatives, MaxAlternativeText, "alternatives"); err == nil {
		q.Alternatives = alts
	} else {
		// An over-long or over-numerous list from the model is cut to
		// the schema's bounds rather than refused: the requirement is
		// still worth judging.
		for _, a := range in.Alternatives {
			a = strings.TrimSpace(a)
			if a == "" || len(q.Alternatives) >= MaxAlternatives {
				continue
			}
			if len(a) > MaxAlternativeText {
				a = a[:MaxAlternativeText]
			}
			q.Alternatives = append(q.Alternatives, a)
		}
	}
	if q.Alternatives == nil {
		q.Alternatives = []string{}
	}
	// A single "alternative" is the requirement itself restated, not a
	// choice; a choice has at least two branches.
	if len(q.Alternatives) < 2 {
		q.Alternatives = []string{}
	}
	q.Thresholds = DeriveThresholds(q.Text, q.Type, q.NamedParties)
	q.EvidenceStandard, q.EvidenceStandardTemplate = RenderEvidenceStandard(q.Type, q.Thresholds, q.NamedParties)
	var entries []string
	q.EquivalenceNotes, entries = catalog.Notes(q.Text, q.SourceQuote)
	if entries == nil {
		entries = []string{}
	}
	q.Included, _ = Inclusion(q.Type, q.Thresholds, q.Text, q.Equivalency)
	q.Extraction, _ = json.Marshal(map[string]any{
		"prompt_id":          prov.PromptID,
		"prompt_fingerprint": prov.PromptFingerprint,
		"model":              prov.Model,
		"type_source":        source,
		"type_extracted":     in.Type,
		"type_heuristic":     heuristic,
		"type_rule":          TypeRule,
		"thresholds_rule":    ThresholdsRule,
		"template":           q.EvidenceStandardTemplate,
		"catalog_entries":    entries,
	})
	return q
}

// ErrHumanEdited is returned when an extraction would replace
// requirements a person has edited. The edit wins: a run on a role
// with edited requirements judges the released rows and does not
// extract again.
var ErrHumanEdited = errors.New("hiring: the role's requirements have been edited by a person")

const requirementColumns = `
  q.id::text, q.role_id::text, q.local_id, q.version, COALESCE(q.supersedes::text, ''),
  q.schema_version, q.vocab_version, q.text, q.source_quote, q.type, q.category, q.class, q.weight,
  q.evidence_standard, q.evidence_standard_template, q.alternatives, q.equivalence_notes, q.equivalency,
  q.thresholds, q.named_parties, q.included, q.extraction,
  COALESCE(q.edited_by, 0), COALESCE(u.name, ''), q.edited_at, q.edit_reason, q.status,
  q.released_at, COALESCE(q.released_by, 0), q.created_at`

const requirementFrom = ` FROM requirements q LEFT JOIN users u ON u.id = q.edited_by `

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRequirement(row rowScanner) (Requirement, error) {
	var q Requirement
	var alternatives, thresholds, extraction []byte
	if err := row.Scan(
		&q.ID, &q.RoleID, &q.LocalID, &q.Version, &q.Supersedes,
		&q.SchemaVersion, &q.VocabVersion, &q.Text, &q.SourceQuote, &q.Type, &q.Category, &q.Class, &q.Weight,
		&q.EvidenceStandard, &q.EvidenceStandardTemplate, &alternatives, &q.EquivalenceNotes, &q.Equivalency,
		&thresholds, &q.NamedParties, &q.Included, &extraction,
		&q.EditedBy, &q.EditedByDisplay, &q.EditedAt, &q.EditReason, &q.Status,
		&q.ReleasedAt, &q.ReleasedBy, &q.CreatedAt,
	); err != nil {
		return q, err
	}
	_ = json.Unmarshal(alternatives, &q.Alternatives)
	if q.Alternatives == nil {
		q.Alternatives = []string{}
	}
	if q.NamedParties == nil {
		q.NamedParties = []string{}
	}
	q.Thresholds = ParseThresholds(thresholds)
	if len(extraction) > 0 {
		q.Extraction = json.RawMessage(extraction)
	}
	return q, nil
}

func queryRequirements(ctx context.Context, db interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, where string, args ...any) ([]Requirement, error) {
	rows, err := db.Query(ctx, `SELECT `+requirementColumns+requirementFrom+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Requirement
	for rows.Next() {
		q, err := scanRequirement(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// localOrder sorts r2 before r10.
func localOrder(a, b Requirement) int {
	na, nb := localNumber(a.LocalID), localNumber(b.LocalID)
	if na != nb {
		return na - nb
	}
	if a.LocalID != b.LocalID {
		return strings.Compare(a.LocalID, b.LocalID)
	}
	return a.Version - b.Version
}

func localNumber(id string) int {
	n := 0
	seen := false
	for _, r := range id {
		if r >= '0' && r <= '9' {
			n = n*10 + int(r-'0')
			seen = true
		}
	}
	if !seen {
		return 1 << 30
	}
	return n
}

// insertRequirement writes one version and returns it as stored.
func insertRequirement(ctx context.Context, tx pgx.Tx, tenantID int64, q Requirement) (Requirement, error) {
	alternatives, _ := json.Marshal(q.Alternatives)
	if q.Alternatives == nil {
		alternatives = []byte(`[]`)
	}
	parties := q.NamedParties
	if parties == nil {
		parties = []string{}
	}
	var supersedes *string
	if q.Supersedes != "" {
		supersedes = &q.Supersedes
	}
	var extraction []byte
	if len(q.Extraction) > 0 && string(q.Extraction) != "null" {
		extraction = q.Extraction
	}
	var editedBy, releasedBy *int64
	if q.EditedBy > 0 {
		editedBy = &q.EditedBy
	}
	if q.ReleasedBy > 0 {
		releasedBy = &q.ReleasedBy
	}
	err := tx.QueryRow(ctx, `
    INSERT INTO requirements
      (tenant_id, role_id, local_id, version, supersedes, schema_version, vocab_version,
       text, source_quote, type, category, class, weight,
       evidence_standard, evidence_standard_template, alternatives, equivalence_notes, equivalency,
       thresholds, named_parties, included, extraction,
       edited_by, edited_at, edit_reason, status, released_at, released_by)
    VALUES ($1, $2::uuid, $3, $4, $5::uuid, $6, $7,
            $8, $9, $10, $11, $12, $13,
            $14, $15, $16, $17, $18,
            $19, $20, $21, $22,
            $23, CASE WHEN $23::bigint IS NULL THEN NULL ELSE now() END, $24, $25,
            CASE WHEN $25 = 'draft' THEN NULL ELSE now() END, $26)
    RETURNING id::text, edited_at, released_at, created_at`,
		tenantID, q.RoleID, q.LocalID, q.Version, supersedes, RequirementSchema, VocabVersion,
		q.Text, q.SourceQuote, q.Type, q.Category, q.Class, q.Weight,
		q.EvidenceStandard, q.EvidenceStandardTemplate, alternatives, q.EquivalenceNotes, q.Equivalency,
		q.Thresholds.JSON(), parties, q.Included, extraction,
		editedBy, q.EditReason, q.Status, releasedBy,
	).Scan(&q.ID, &q.EditedAt, &q.ReleasedAt, &q.CreatedAt)
	if err != nil {
		return q, err
	}
	q.SchemaVersion, q.VocabVersion = RequirementSchema, VocabVersion
	return q, nil
}

// SyncRequirementVersions makes the role's released requirements match
// this run's extraction, version by version (FR-HF-01): a requirement
// whose judged fields are unchanged keeps its row; one that changed
// gets a new row with supersedes set and the old one retired; one the
// extractor no longer produces is retired. Nothing is rewritten in
// place, so a judgment's requirement_id always names exactly what was
// judged. The rows are released as they are written, by the pipeline
// (released_by null): this is the path of a run that is not held for a
// person's review. Returns the rows this run should judge, in input
// order.
//
// It refuses, with ErrHumanEdited and no change, when a person has
// edited any current requirement of the role: an extraction must never
// overwrite an edit, and the local ids of two extractions do not line
// up well enough to merge them.
func (r *Repo) SyncRequirementVersions(ctx context.Context, roleID string, drafts []Requirement) ([]Requirement, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("sync requirements: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := queryRequirements(ctx, tx, `WHERE q.role_id = $1::uuid AND q.status IN ('active', 'draft') FOR UPDATE OF q`, roleID)
	if err != nil {
		return nil, fmt.Errorf("sync requirements: load: %w", err)
	}
	active := map[string]Requirement{}
	for _, q := range current {
		if q.EditedBy > 0 || q.Status == StatusDraft {
			return nil, ErrHumanEdited
		}
		active[q.LocalID] = q
	}

	tenantID := tenant.FromContext(ctx).Int64()
	out := make([]Requirement, 0, len(drafts))
	seen := map[string]bool{}
	for _, d := range drafts {
		if d.LocalID == "" || seen[d.LocalID] {
			continue
		}
		seen[d.LocalID] = true
		d.RoleID = roleID
		cur, exists := active[d.LocalID]
		if exists && sameJudged(cur, d) {
			out = append(out, cur)
			continue
		}
		// A new version: retire the old row (if any) and insert the next.
		if exists {
			if _, err := tx.Exec(ctx, `UPDATE requirements SET status = 'retired' WHERE id = $1::uuid`, cur.ID); err != nil {
				return nil, fmt.Errorf("sync requirements: retire %s: %w", d.LocalID, err)
			}
			d.Supersedes = cur.ID
		}
		if err := tx.QueryRow(ctx, `
      SELECT COALESCE(MAX(version), 0) + 1 FROM requirements WHERE role_id = $1::uuid AND local_id = $2`,
			roleID, d.LocalID).Scan(&d.Version); err != nil {
			return nil, fmt.Errorf("sync requirements: next version: %w", err)
		}
		d.Status = StatusActive
		d.EditedBy, d.EditReason, d.ReleasedBy = 0, "", 0
		row, err := insertRequirement(ctx, tx, tenantID, d)
		if err != nil {
			return nil, fmt.Errorf("sync requirements: insert %s: %w", d.LocalID, err)
		}
		out = append(out, row)
	}
	// Requirements the extractor no longer produces are retired: they
	// are not part of what this run judges, and a later run that
	// produces them again starts a new version from the retired row.
	for localID, cur := range active {
		if !seen[localID] {
			if _, err := tx.Exec(ctx, `UPDATE requirements SET status = 'retired' WHERE id = $1::uuid`, cur.ID); err != nil {
				return nil, fmt.Errorf("sync requirements: retire absent %s: %w", localID, err)
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("sync requirements: commit: %w", err)
	}
	return out, nil
}

// RoleForSubmission returns the role of a submitted posting, or false
// when the posting has not been through the pipeline since the hiring
// tables began.
func (r *Repo) RoleForSubmission(ctx context.Context, submissionID int64) (string, bool, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
    SELECT id::text FROM roles WHERE submission_id = $1 AND tenant_id = $2`,
		submissionID, tenant.FromContext(ctx).Int64()).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("role for submission: %w", err)
	}
	return id, true, nil
}

// SubmissionForRole returns the submission a role was created from; 0
// for an authored role, which Phase 0 does not have.
func (r *Repo) SubmissionForRole(ctx context.Context, roleID string) (int64, error) {
	var id *int64
	err := r.pool.QueryRow(ctx, `
    SELECT submission_id FROM roles WHERE id = $1::uuid AND tenant_id = $2`,
		roleID, tenant.FromContext(ctx).Int64()).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("submission for role: %w", err)
	}
	if id == nil {
		return 0, nil
	}
	return *id, nil
}

// ListRequirements returns a role's requirements in reading order: the
// released version of each, any pending draft beside it, and the
// retired versions when asked for, so an editor can show what changed.
func (r *Repo) ListRequirements(ctx context.Context, roleID string, includeRetired bool) ([]Requirement, error) {
	where := `WHERE q.role_id = $1::uuid AND q.tenant_id = $2`
	if !includeRetired {
		where += ` AND q.status IN ('active', 'draft')`
	}
	out, err := queryRequirements(ctx, r.pool, where, roleID, tenant.FromContext(ctx).Int64())
	if err != nil {
		return nil, fmt.Errorf("list requirements: %w", err)
	}
	slices.SortStableFunc(out, localOrder)
	return out, nil
}

// ActiveRequirements returns the released version of each of a role's
// requirements, in reading order: what a run judges.
func (r *Repo) ActiveRequirements(ctx context.Context, roleID string) ([]Requirement, error) {
	out, err := queryRequirements(ctx, r.pool, `WHERE q.role_id = $1::uuid AND q.status = 'active'`, roleID)
	if err != nil {
		return nil, fmt.Errorf("active requirements: %w", err)
	}
	slices.SortStableFunc(out, localOrder)
	return out, nil
}

// GetRequirement returns one version by id.
func (r *Repo) GetRequirement(ctx context.Context, id string) (Requirement, error) {
	q, err := scanRequirement(r.pool.QueryRow(ctx, `SELECT `+requirementColumns+requirementFrom+
		`WHERE q.id = $1::uuid AND q.tenant_id = $2`, id, tenant.FromContext(ctx).Int64()))
	if errors.Is(err, pgx.ErrNoRows) {
		return q, ErrNotFound
	}
	if err != nil {
		return q, fmt.Errorf("get requirement: %w", err)
	}
	return q, nil
}

// UpdateRequirement saves an edit as a new version (FR-HF-03). The
// version being edited must be the current one for its requirement:
// the pending draft when there is one, otherwise the released version.
// The new version is a draft with supersedes pointing at the one it
// came from, the editor and a required reason. A released version
// stays released, and stays what a run judges, until the drafts are
// released; an earlier draft it replaces is retired, never having been
// judged. The catalog may be nil, in which case equivalence notes are
// carried over as they are.
func (r *Repo) UpdateRequirement(ctx context.Context, id string, patch RequirementPatch, editedBy int64, reason string, catalog *Catalog) (Requirement, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return Requirement{}, invalid("an edit reason is required")
	}
	if len(reason) > MaxEditReason {
		return Requirement{}, invalid("the edit reason is longer than %d characters", MaxEditReason)
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Requirement{}, fmt.Errorf("update requirement: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tenantID := tenant.FromContext(ctx).Int64()
	cur, err := scanRequirement(tx.QueryRow(ctx, `SELECT `+requirementColumns+requirementFrom+
		`WHERE q.id = $1::uuid AND q.tenant_id = $2 FOR UPDATE OF q`, id, tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Requirement{}, ErrNotFound
	}
	if err != nil {
		return Requirement{}, fmt.Errorf("update requirement: load: %w", err)
	}
	switch cur.Status {
	case StatusRetired:
		return Requirement{}, fmt.Errorf("%w: it has been superseded; edit the current version", ErrNotEditable)
	case StatusActive:
		var drafts int
		if err := tx.QueryRow(ctx, `
      SELECT count(*) FROM requirements WHERE role_id = $1::uuid AND local_id = $2 AND status = 'draft'`,
			cur.RoleID, cur.LocalID).Scan(&drafts); err != nil {
			return Requirement{}, fmt.Errorf("update requirement: drafts: %w", err)
		}
		if drafts > 0 {
			return Requirement{}, fmt.Errorf("%w: a draft of this requirement is pending; edit the draft", ErrNotEditable)
		}
	}
	next, err := applyPatch(cur, patch, catalog)
	if err != nil {
		return Requirement{}, err
	}
	if err := tx.QueryRow(ctx, `
    SELECT COALESCE(MAX(version), 0) + 1 FROM requirements WHERE role_id = $1::uuid AND local_id = $2`,
		cur.RoleID, cur.LocalID).Scan(&next.Version); err != nil {
		return Requirement{}, fmt.Errorf("update requirement: next version: %w", err)
	}
	if cur.Status == StatusDraft {
		if _, err := tx.Exec(ctx, `UPDATE requirements SET status = 'retired' WHERE id = $1::uuid`, cur.ID); err != nil {
			return Requirement{}, fmt.Errorf("update requirement: retire draft: %w", err)
		}
	}
	next.Supersedes = cur.ID
	next.Status = StatusDraft
	next.EditedBy, next.EditReason = editedBy, reason
	next.ReleasedBy, next.ReleasedAt = 0, nil
	row, err := insertRequirement(ctx, tx, tenantID, next)
	if err != nil {
		return Requirement{}, fmt.Errorf("update requirement: insert: %w", err)
	}
	if err := tx.QueryRow(ctx, `SELECT COALESCE(name, '') FROM users WHERE id = $1`, editedBy).Scan(&row.EditedByDisplay); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Requirement{}, fmt.Errorf("update requirement: editor: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Requirement{}, fmt.Errorf("update requirement: commit: %w", err)
	}
	return row, nil
}

// ReleaseRequirements releases every pending draft of a role
// (FR-HF-03): each draft becomes the released version of its
// requirement and the version it replaces is retired, in one
// transaction, so a run never sees half a release. Returns the drafts
// released; none is not an error, because releasing is also how a
// re-judge of an unchanged list is asked for.
func (r *Repo) ReleaseRequirements(ctx context.Context, roleID string, releasedBy int64) ([]Requirement, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("release requirements: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := tenant.FromContext(ctx).Int64()
	drafts, err := queryRequirements(ctx, tx, `WHERE q.role_id = $1::uuid AND q.tenant_id = $2 AND q.status = 'draft' FOR UPDATE OF q`, roleID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("release requirements: load: %w", err)
	}
	var by *int64
	if releasedBy > 0 {
		by = &releasedBy
	}
	out := make([]Requirement, 0, len(drafts))
	for _, d := range drafts {
		if _, err := tx.Exec(ctx, `
      UPDATE requirements SET status = 'retired'
      WHERE role_id = $1::uuid AND local_id = $2 AND status = 'active'`, roleID, d.LocalID); err != nil {
			return nil, fmt.Errorf("release requirements: retire %s: %w", d.LocalID, err)
		}
		if err := tx.QueryRow(ctx, `
      UPDATE requirements SET status = 'active', released_at = now(), released_by = $2
      WHERE id = $1::uuid RETURNING released_at`, d.ID, by).Scan(&d.ReleasedAt); err != nil {
			return nil, fmt.Errorf("release requirements: activate %s: %w", d.LocalID, err)
		}
		d.Status, d.ReleasedBy = StatusActive, releasedBy
		out = append(out, d)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("release requirements: commit: %w", err)
	}
	slices.SortStableFunc(out, localOrder)
	return out, nil
}

// HumanEdited reports whether any current requirement of a role, the
// released one or a pending draft, was written by a person. A run on
// such a role judges the released rows and never extracts over them.
func (r *Repo) HumanEdited(ctx context.Context, roleID string) (bool, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
    SELECT count(*) FROM requirements
    WHERE role_id = $1::uuid AND status IN ('active', 'draft') AND (edited_by IS NOT NULL OR status = 'draft')`,
		roleID).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("human edited: %w", err)
	}
	return n > 0, nil
}
