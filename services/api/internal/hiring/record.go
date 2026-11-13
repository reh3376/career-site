package hiring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/reh3376/career-site/services/api/internal/tenant"
)

// The match record as a person reads it, and the override of a
// judgment (FSD FR-HF-08, FR-HF-10, FR-HF-17; Phase 0 step 6). The
// record is read through v_match_record and v_judgments_effective, the
// reading boundary 14 §2.4 names, so the viewer, an export and a metric
// see the same columns.

// Gap classes (FR-HF-17): what kind of distance lies between the
// evidence and the requirement, derived from the basis. A lexical gap
// is a difference of wording, tool or credential that an equivalency
// could bridge, which is the gap a keyword screen turns into a
// rejection; a substantive gap is the work itself falling short; an
// unaddressed one is silence.
const (
	GapLexical     = "lexical"
	GapSubstantive = "substantive"
	GapUnaddressed = "unaddressed"
)

// GapClass derives the gap class from a basis. Direct has none, and a
// code this build does not know has none rather than a guess.
func GapClass(basis string) string {
	switch {
	case IsEquivalencyBasis(basis):
		return GapLexical
	case ShortfallBases[basis] != "" || basis == BasisContradicted:
		return GapSubstantive
	case basis == BasisNotAddressed:
		return GapUnaddressed
	}
	return ""
}

// The override roles (vocabulary override_role). A candidate's dispute
// is reserved until D-38 is decided and cannot be set.
const (
	RoleHiringManager    = "hiring_manager"
	RoleRecruiter        = "recruiter"
	RoleSystemAdmin      = "system_admin"
	RoleCandidateDispute = "candidate_dispute"
)

// MaxOverrideReason bounds an override's reason (04 §2.1).
const MaxOverrideReason = 1000

// TypeBehavioral is the requirement type that is recorded and never
// judged from documents (FR-HF-18).
const TypeBehavioral = "behavioral"

// OverrideBases lists, from all, the bases a person may pair with a
// determination. They are the consistency rules of 04 §2.3 as they
// apply to a person: met is direct, met through an equivalency names
// the kind of equivalency, an unresolved or unmet row names what stands
// between the evidence and the requirement. A person is not held to the
// evidence-count rules the model is, because a reviewer may know what
// the documents do not say.
func OverrideBases(determination string, all []string) []string {
	var out []string
	for _, b := range all {
		if OverrideViolation(determination, b) == "" {
			out = append(out, b)
		}
	}
	return out
}

// OverrideViolation says why a determination and basis contradict each
// other, as a sentence for the reviewer, or "" when they do not.
func OverrideViolation(determination, basis string) string {
	switch determination {
	case DetMet:
		if basis != BasisDirect {
			return "a verdict of met rests on the basis direct; a requirement satisfied by something different in name is met by equivalence"
		}
	case DetMetEquivalent:
		if !IsEquivalencyBasis(basis) {
			return "a verdict of met by equivalence names the kind of equivalency: " + strings.Join(EquivalencyBases, ", ")
		}
	case DetUnresolved:
		if basis == BasisDirect || basis == BasisNotAddressed || basis == BasisContradicted {
			return "an unresolved verdict names what is suggested and missing; with basis " + basis + " the verdict is met or not met"
		}
	case DetNotMet:
		if basis == BasisDirect {
			return "a verdict of not met cannot rest on the basis direct"
		}
	default:
		return "determination " + quote(determination) + " is not one a reviewer may set"
	}
	return ""
}

// ErrInvalidOverride wraps the sentence that says what was wrong with
// an override as asked for.
var ErrInvalidOverride = errors.New("hiring: invalid override")

// ErrOverrideRefused wraps the sentence that says why this judgment
// takes no override now.
var ErrOverrideRefused = errors.New("hiring: override refused")

func invalidOverride(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidOverride, fmt.Sprintf(format, args...))
}

func overrideRefused(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrOverrideRefused, fmt.Sprintf(format, args...))
}

// RecordEvidence is one citation on a judgment: what kind of thing was
// cited and, for a passage of a document, which document. The text is
// never part of it.
type RecordEvidence struct {
	ID         string
	SourceKind string
	Title      string
	ChunkID    int64
	// Public is whether the item's document is public. A reader who is
	// not an admin is shown the title only when it is.
	Public    bool
	Withdrawn bool
}

// RecordOverride is a person's override as stored.
type RecordOverride struct {
	Determination string
	Basis         string
	Role          string
	Reason        string
	By            int64
	ByDisplay     string
	At            time.Time
}

// EarlierOverride is an override on an earlier run's judgment of the
// same requirement, for a judgment that carries none.
type EarlierOverride struct {
	Attempt       int
	Determination string
	Basis         string
	Role          string
	At            time.Time
	SameVersion   bool
}

// RecordJudgment is one judgment as the record shows it.
type RecordJudgment struct {
	ID                     string
	RequirementID          string
	LocalID                string
	Determination          string
	Basis                  string
	Uncertainty            []string
	EffectiveDetermination string
	EffectiveBasis         string
	Override               *RecordOverride
	DerivedVerdict         string
	Rationale              string
	Evidence               []RecordEvidence
	RuleAdjustments        []RuleAdjustment
	Facts                  ReportedFacts
	GapClass               string
	NotAssessed            bool
	Stale                  bool
	KeywordKnown           bool
	KeywordFound           bool
	KeywordTerms           []string
	DecisionID             int64
	Earlier                *EarlierOverride
}

// Record is one run's match record with everything a reader is shown.
type Record struct {
	RunID             string
	RoleID            string
	RoleTitle         string
	SubmissionID      int64
	Attempt           int
	SupersedesRunID   string
	Superseded        bool
	RunTrigger        string
	FormulaVersion    string
	PreferredFraction float64
	// HasNumbers is false for a record written under the v2 arithmetic,
	// which holds a score and not the three numbers.
	HasNumbers         bool
	Coverage           Coverage
	Requirements       []Requirement
	Judgments          []RecordJudgment
	OverrideCount      int
	KeywordRule        string
	KeywordSkipped     string
	Versions           map[string]string
	Fit                string
	MatchScore         *float64
	FinishedAt         *time.Time
	CorpusDocuments    int
	Model              string
	RequirementsEdited bool
}

// LatestRecordRun names the newest run of a submission that has a match
// record with judgment rows, which is the record a reader is shown.
// latestAttempt is the newest run of any kind, so a caller can tell a
// current record from one a later run is replacing. ok is false when
// the posting has no such record: one last scored before the judgment
// rows began.
func (r *Repo) LatestRecordRun(ctx context.Context, submissionID int64) (runID string, attempt, latestAttempt int, ok bool, err error) {
	err = r.pool.QueryRow(ctx, `
    SELECT v.run_id::text, v.attempt,
           (SELECT COALESCE(max(x.attempt), 0) FROM jd_runs x WHERE x.submission_id = v.submission_id)
    FROM v_match_record v
    WHERE v.submission_id = $1 AND v.tenant_id = $2
      AND EXISTS (SELECT 1 FROM judgments j WHERE j.run_id = v.run_id)
    ORDER BY v.attempt DESC LIMIT 1`,
		submissionID, tenant.FromContext(ctx).Int64()).Scan(&runID, &attempt, &latestAttempt)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, 0, false, nil
	}
	if err != nil {
		return "", 0, 0, false, fmt.Errorf("latest record run: %w", err)
	}
	return runID, attempt, latestAttempt, true, nil
}

// GetRecord reads one run's match record. ErrNotFound when the run has
// no record or no judgment rows.
func (r *Repo) GetRecord(ctx context.Context, runID string) (Record, error) {
	tid := tenant.FromContext(ctx).Int64()
	var rec Record
	var evidenced, unresolved, unmet, eqShare, selfShare *float64
	var keyword, versions []byte
	var reqIDs []string
	err := r.pool.QueryRow(ctx, `
    SELECT v.run_id::text, v.role_id::text, COALESCE(ro.title, ''), v.submission_id, v.attempt,
           COALESCE(v.supersedes_run_id::text, ''), v.run_trigger, v.formula_version,
           v.evidenced, v.unresolved, v.unmet, v.coverage_equivalent_share, v.self_attested_share,
           v.override_count, v.keyword_baseline, v.versions, v.model, v.fit, v.run_match_score,
           v.finished_at, v.corpus_documents, v.requirement_ids::text[]
    FROM v_match_record v
    LEFT JOIN roles ro ON ro.id = v.role_id
    WHERE v.run_id = $1::uuid AND v.tenant_id = $2`, runID, tid).Scan(
		&rec.RunID, &rec.RoleID, &rec.RoleTitle, &rec.SubmissionID, &rec.Attempt,
		&rec.SupersedesRunID, &rec.RunTrigger, &rec.FormulaVersion,
		&evidenced, &unresolved, &unmet, &eqShare, &selfShare,
		&rec.OverrideCount, &keyword, &versions, &rec.Model, &rec.Fit, &rec.MatchScore,
		&rec.FinishedAt, &rec.CorpusDocuments, &reqIDs)
	if errors.Is(err, pgx.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, fmt.Errorf("match record: %w", err)
	}
	rec.Versions, rec.PreferredFraction = flattenVersions(versions)
	if evidenced != nil && unresolved != nil && unmet != nil {
		rec.HasNumbers = true
		rec.Coverage = Coverage{Evidenced: *evidenced, Unresolved: *unresolved, Unmet: *unmet}
		if eqShare != nil {
			rec.Coverage.EquivalentShare = *eqShare
		}
		if selfShare != nil {
			rec.Coverage.SelfAttestedShare = *selfShare
		}
	}

	// The requirement versions judged, in the order they were judged.
	reqs, err := queryRequirements(ctx, r.pool, `WHERE q.id = ANY($1::uuid[])`, reqIDs)
	if err != nil {
		return Record{}, fmt.Errorf("match record: requirements: %w", err)
	}
	position := map[string]int{}
	for i, id := range reqIDs {
		position[id] = i
	}
	sort.SliceStable(reqs, func(a, b int) bool { return position[reqs[a].ID] < position[reqs[b].ID] })
	rec.Requirements = reqs
	typeOf := map[string]string{}
	for _, q := range reqs {
		typeOf[q.ID] = q.Type
		if q.EditedBy > 0 {
			rec.RequirementsEdited = true
		}
	}

	// The keyword screen, per requirement local id (FR-HF-12).
	var baseline struct {
		Rule    string `json:"rule"`
		Skipped string `json:"skipped"`
		Items   map[string]struct {
			Found bool     `json:"found"`
			Terms []string `json:"terms"`
		} `json:"items"`
	}
	_ = json.Unmarshal(keyword, &baseline)
	rec.KeywordRule, rec.KeywordSkipped = baseline.Rule, baseline.Skipped

	rows, err := r.pool.Query(ctx, `
    SELECT j.id::text, j.requirement_id::text, j.requirement_local_id,
           j.determination, j.basis, j.uncertainty,
           j.effective_determination, j.effective_basis, j.derived_verdict,
           j.overridden, COALESCE(j.override_determination, ''), COALESCE(j.override_basis, ''),
           COALESCE(j.override_role, ''), COALESCE(j.override_reason, ''), COALESCE(j.override_by, 0),
           j.override_by_name, j.override_at,
           j.rationale, j.rule_adjustments, j.reported_facts, j.stale,
           j.evidence_ids::text[], COALESCE(j.decision_id, 0)
    FROM v_judgments_effective j
    WHERE j.run_id = $1::uuid AND j.tenant_id = $2`, runID, tid)
	if err != nil {
		return Record{}, fmt.Errorf("match record: judgments: %w", err)
	}
	defer rows.Close()
	evidenceOf := map[string][]string{}
	var allEvidence []string
	for rows.Next() {
		var j RecordJudgment
		var overridden bool
		var o RecordOverride
		var at *time.Time
		var adjustments, facts []byte
		var evidenceIDs []string
		if err := rows.Scan(
			&j.ID, &j.RequirementID, &j.LocalID,
			&j.Determination, &j.Basis, &j.Uncertainty,
			&j.EffectiveDetermination, &j.EffectiveBasis, &j.DerivedVerdict,
			&overridden, &o.Determination, &o.Basis, &o.Role, &o.Reason, &o.By, &o.ByDisplay, &at,
			&j.Rationale, &adjustments, &facts, &j.Stale,
			&evidenceIDs, &j.DecisionID,
		); err != nil {
			return Record{}, fmt.Errorf("match record: judgment: %w", err)
		}
		if overridden {
			if at != nil {
				o.At = *at
			}
			j.Override = &o
		}
		if j.Uncertainty == nil {
			j.Uncertainty = []string{}
		}
		_ = json.Unmarshal(adjustments, &j.RuleAdjustments)
		_ = json.Unmarshal(facts, &j.Facts)
		j.NotAssessed = typeOf[j.RequirementID] == TypeBehavioral
		if !j.NotAssessed {
			j.GapClass = GapClass(j.EffectiveBasis)
		}
		if item, ok := baseline.Items[j.LocalID]; ok && baseline.Skipped == "" {
			j.KeywordKnown, j.KeywordFound, j.KeywordTerms = true, item.Found, item.Terms
		}
		evidenceOf[j.ID] = evidenceIDs
		for _, id := range evidenceIDs {
			if !slices.Contains(allEvidence, id) {
				allEvidence = append(allEvidence, id)
			}
		}
		rec.Judgments = append(rec.Judgments, j)
	}
	if err := rows.Err(); err != nil {
		return Record{}, fmt.Errorf("match record: judgments: %w", err)
	}
	if len(rec.Judgments) == 0 {
		return Record{}, ErrNotFound
	}
	sort.SliceStable(rec.Judgments, func(a, b int) bool {
		return position[rec.Judgments[a].RequirementID] < position[rec.Judgments[b].RequirementID]
	})

	evidence, err := r.recordEvidence(ctx, allEvidence)
	if err != nil {
		return Record{}, err
	}
	for i := range rec.Judgments {
		for _, id := range evidenceOf[rec.Judgments[i].ID] {
			if e, ok := evidence[id]; ok {
				rec.Judgments[i].Evidence = append(rec.Judgments[i].Evidence, e)
			}
		}
	}

	if err := r.pool.QueryRow(ctx, `
    SELECT EXISTS (
      SELECT 1 FROM v_match_record v
      WHERE v.submission_id = $1 AND v.tenant_id = $2 AND v.attempt > $3
        AND EXISTS (SELECT 1 FROM judgments j WHERE j.run_id = v.run_id))`,
		rec.SubmissionID, tid, rec.Attempt).Scan(&rec.Superseded); err != nil {
		return Record{}, fmt.Errorf("match record: superseded: %w", err)
	}
	if err := r.earlierOverrides(ctx, &rec); err != nil {
		return Record{}, err
	}
	return rec, nil
}

// recordEvidence reads the citations behind a set of evidence items.
func (r *Repo) recordEvidence(ctx context.Context, ids []string) (map[string]RecordEvidence, error) {
	out := map[string]RecordEvidence{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
    SELECT e.id::text, e.source_kind, COALESCE(d.title, ''), COALESCE(e.chunk_id, 0),
           e.visibility = 'public', e.withdrawn_at IS NOT NULL
    FROM evidence_items e
    LEFT JOIN corpus_chunks c ON c.id = e.chunk_id
    LEFT JOIN corpus_documents d ON d.id = c.document_id
    WHERE e.id = ANY($1::uuid[]) AND e.tenant_id = $2`, ids, tenant.FromContext(ctx).Int64())
	if err != nil {
		return nil, fmt.Errorf("match record: evidence: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var e RecordEvidence
		if err := rows.Scan(&e.ID, &e.SourceKind, &e.Title, &e.ChunkID, &e.Public, &e.Withdrawn); err != nil {
			return nil, fmt.Errorf("match record: evidence: %w", err)
		}
		out[e.ID] = e
	}
	return out, rows.Err()
}

// earlierOverrides marks each judgment that carries no override with
// the newest override an earlier run of the same posting put on the
// same requirement. A new run judges every requirement again and
// starts without overrides; without this a reviewer would see a row
// they had decided come back undecided and not know why.
func (r *Repo) earlierOverrides(ctx context.Context, rec *Record) error {
	if rec.Attempt <= 1 {
		return nil
	}
	rows, err := r.pool.Query(ctx, `
    SELECT DISTINCT ON (j.requirement_local_id)
           j.requirement_local_id, j.requirement_id::text, run.attempt,
           j.override_determination, COALESCE(j.override_basis, ''), COALESCE(j.override_role, ''), j.override_at
    FROM judgments j
    JOIN jd_runs run ON run.run_id = j.run_id
    WHERE run.submission_id = $1 AND j.tenant_id = $2 AND run.attempt < $3
      AND j.override_determination IS NOT NULL
    ORDER BY j.requirement_local_id, run.attempt DESC`,
		rec.SubmissionID, tenant.FromContext(ctx).Int64(), rec.Attempt)
	if err != nil {
		return fmt.Errorf("match record: earlier overrides: %w", err)
	}
	defer rows.Close()
	type found struct {
		requirementID string
		o             EarlierOverride
	}
	byLocal := map[string]found{}
	for rows.Next() {
		var local string
		var f found
		var at *time.Time
		if err := rows.Scan(&local, &f.requirementID, &f.o.Attempt, &f.o.Determination, &f.o.Basis, &f.o.Role, &at); err != nil {
			return fmt.Errorf("match record: earlier overrides: %w", err)
		}
		if at != nil {
			f.o.At = *at
		}
		byLocal[local] = f
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("match record: earlier overrides: %w", err)
	}
	for i := range rec.Judgments {
		j := &rec.Judgments[i]
		if j.Override != nil {
			continue
		}
		if f, ok := byLocal[j.LocalID]; ok {
			o := f.o
			o.SameVersion = f.requirementID == j.RequirementID
			j.Earlier = &o
		}
	}
	return nil
}

// flattenVersions turns the record's versions snapshot into one level
// of strings for display, with the prompts as prompt.<id>, and returns
// the preferred fraction the record was computed with.
func flattenVersions(raw []byte) (map[string]string, float64) {
	out := map[string]string{}
	var v map[string]any
	if json.Unmarshal(raw, &v) != nil {
		return out, DefaultPreferredFraction
	}
	fraction := DefaultPreferredFraction
	for k, val := range v {
		switch x := val.(type) {
		case string:
			if x != "" {
				out[k] = x
			}
		case float64:
			out[k] = strconv.FormatFloat(x, 'f', -1, 64)
			if k == "preferred_fraction" && x > 0 && x <= 1 {
				fraction = x
			}
		case map[string]any:
			for id, fp := range x {
				if s, ok := fp.(string); ok && s != "" {
					out[strings.TrimSuffix(k, "s")+"."+id] = s
				}
			}
		}
	}
	return out, fraction
}

// RecordCoverage computes a record's numbers under the formula it was
// written with, so an override moves only what the override changed.
// ok is false for the v2 arithmetic, which has no three numbers to
// compute.
func RecordCoverage(formula string, items []Item, preferredFraction float64) (Coverage, bool) {
	switch formula {
	case FormulaV4:
		return CoverageV4(items, preferredFraction), true
	case FormulaV3:
		return CoverageV3(items), true
	}
	return Coverage{}, false
}

// OverrideInput is one override as a reviewer asked for it.
type OverrideInput struct {
	JudgmentID    string
	Determination string
	Basis         string
	Reason        string
	Role          string
	// Withdraw removes the override the judgment carries.
	Withdraw bool
	By       int64
}

// OverrideResult says what an override touched, for the caller's log
// and event.
type OverrideResult struct {
	RunID             string
	SubmissionID      int64
	LocalID           string
	ModelDetermined   string
	ModelBasis        string
	Effective         string
	EffectiveBasis    string
	OverrideCount     int
	DecisionID        int64
	PreviousEffective string
}

// OverrideJudgment sets, replaces or withdraws a person's override of
// one judgment (FR-HF-08) and computes the record's numbers again from
// the effective values, in one transaction with the decision-log row
// that records it. The model's determination and basis are not
// touched: they are what the agreement views compare the override
// against. reg validates the codes; nil validates against the
// compiled-in vocabulary.
func (r *Repo) OverrideJudgment(ctx context.Context, in OverrideInput, reg *Registry) (OverrideResult, error) {
	var res OverrideResult
	in.Reason = strings.TrimSpace(in.Reason)
	in.Determination = strings.TrimSpace(in.Determination)
	in.Basis = strings.TrimSpace(in.Basis)
	in.Role = strings.TrimSpace(in.Role)
	if in.Reason == "" {
		return res, invalidOverride("a reason is required")
	}
	if len([]rune(in.Reason)) > MaxOverrideReason {
		return res, invalidOverride("the reason is longer than %d characters", MaxOverrideReason)
	}
	if in.By <= 0 {
		return res, invalidOverride("an override is made by a person")
	}
	if !in.Withdraw {
		known := func(vocab, code string, fallback []string) bool {
			if reg != nil {
				return reg.Known(vocab, code)
			}
			return slices.Contains(fallback, code)
		}
		defaults := DefaultVocab()
		if !known(VocabDetermination, in.Determination, Codes(defaults.Determination)) {
			return res, invalidOverride("determination %s is not one of the allowed codes", quote(in.Determination))
		}
		if !known(VocabBasis, in.Basis, Codes(defaults.Basis)) {
			return res, invalidOverride("basis %s is not one of the allowed codes", quote(in.Basis))
		}
		if v := OverrideViolation(in.Determination, in.Basis); v != "" {
			return res, invalidOverride("%s", v)
		}
		if in.Role == RoleCandidateDispute {
			return res, invalidOverride("the candidate-dispute role is reserved and cannot be set yet")
		}
		if !known(VocabOverrideRole, in.Role, []string{RoleHiringManager, RoleRecruiter, RoleSystemAdmin}) {
			return res, invalidOverride("role %s is not one a reviewer may act in", quote(in.Role))
		}
	}

	tid := tenant.FromContext(ctx).Int64()
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return res, fmt.Errorf("override: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		requirementID, requirementText, requirementType string
		requirementVersion, attempt, evidenceCount      int
		uncertainty                                     []string
		prevDet, prevBasis, prevRole, prevReason        string
		prevEffective                                   string
	)
	err = tx.QueryRow(ctx, `
    SELECT j.run_id::text, j.requirement_local_id, j.determination, j.basis, j.uncertainty,
           cardinality(j.evidence_ids), j.effective_determination,
           COALESCE(j.override_determination, ''), COALESCE(j.override_basis, ''),
           COALESCE(j.override_role, ''), COALESCE(j.override_reason, ''),
           q.id::text, q.text, q.type, q.version, run.submission_id, run.attempt
    FROM judgments j
    JOIN requirements q ON q.id = j.requirement_id
    JOIN jd_runs run ON run.run_id = j.run_id
    WHERE j.id = $1::uuid AND j.tenant_id = $2
    FOR UPDATE OF j`, in.JudgmentID, tid).Scan(
		&res.RunID, &res.LocalID, &res.ModelDetermined, &res.ModelBasis, &uncertainty,
		&evidenceCount, &prevEffective,
		&prevDet, &prevBasis, &prevRole, &prevReason,
		&requirementID, &requirementText, &requirementType, &requirementVersion, &res.SubmissionID, &attempt)
	if errors.Is(err, pgx.ErrNoRows) {
		return res, ErrNotFound
	}
	if err != nil {
		return res, fmt.Errorf("override: read judgment: %w", err)
	}
	res.PreviousEffective = prevEffective
	if requirementType == TypeBehavioral {
		return res, overrideRefused("a working-style requirement is not judged from documents and takes no override")
	}
	var superseded bool
	if err := tx.QueryRow(ctx, `
    SELECT EXISTS (
      SELECT 1 FROM match_records m JOIN jd_runs x ON x.run_id = m.run_id
      WHERE x.submission_id = $1 AND x.attempt > $2
        AND EXISTS (SELECT 1 FROM judgments j WHERE j.run_id = m.run_id))`,
		res.SubmissionID, attempt).Scan(&superseded); err != nil {
		return res, fmt.Errorf("override: superseded: %w", err)
	}
	if superseded {
		return res, overrideRefused("a later run has replaced this record; override the judgment on the current record")
	}

	action := "override"
	switch {
	case in.Withdraw:
		if prevDet == "" {
			return res, overrideRefused("this judgment carries no override to withdraw")
		}
		action = "withdraw"
		if _, err := tx.Exec(ctx, `
      UPDATE judgments
      SET override_determination = NULL, override_basis = NULL, override_by = NULL,
          override_at = NULL, override_reason = NULL, override_role = NULL
      WHERE id = $1::uuid`, in.JudgmentID); err != nil {
			return res, fmt.Errorf("override: withdraw: %w", err)
		}
		res.Effective, res.EffectiveBasis = res.ModelDetermined, res.ModelBasis
	default:
		if prevDet == in.Determination && prevBasis == in.Basis && prevRole == in.Role && prevReason == in.Reason {
			return res, invalidOverride("this judgment already carries that override")
		}
		if _, err := tx.Exec(ctx, `
      UPDATE judgments
      SET override_determination = $2, override_basis = $3, override_by = $4,
          override_at = now(), override_reason = $5, override_role = $6
      WHERE id = $1::uuid`, in.JudgmentID, in.Determination, in.Basis, in.By, in.Reason, in.Role); err != nil {
			return res, fmt.Errorf("override: write: %w", err)
		}
		res.Effective, res.EffectiveBasis = in.Determination, in.Basis
	}

	// The decision log holds the override with what it was made from,
	// as it holds every decision the pipeline makes. It is entered as
	// reviewed, by the person who made it: a human decision is not put
	// in the queue to be graded by a human.
	var previous any
	if prevDet != "" {
		previous = map[string]any{"determination": prevDet, "basis": prevBasis, "role": prevRole, "reason": prevReason}
	}
	input, _ := json.Marshal(map[string]any{
		"judgment_id": in.JudgmentID, "run_id": res.RunID, "attempt": attempt,
		"requirement_id": requirementID, "requirement_version": requirementVersion, "requirement": requirementText,
		"model":             map[string]any{"determination": res.ModelDetermined, "basis": res.ModelBasis, "uncertainty": uncertainty},
		"evidence_count":    evidenceCount,
		"previous_override": previous,
	})
	output, _ := json.Marshal(map[string]any{
		"action": action, "determination": res.Effective, "basis": res.EffectiveBasis,
		"role": in.Role, "reason": in.Reason,
		"verdict": DeriveVerdict(res.Effective),
	})
	if err := tx.QueryRow(ctx, `
    INSERT INTO decision_log
      (tenant_id, run_id, kind, ref_kind, ref_id, key, model, input, output,
       human_verdict, human_note, reviewed_by, reviewed_at)
    VALUES ($1, $2::uuid, $3, 'jd_submission', $4, $5, 'human', $6, $7, $8, $9, $10, now())
    RETURNING id`,
		tid, res.RunID, DecisionKindOverride, res.SubmissionID, res.LocalID, input, output,
		DeriveVerdict(res.Effective), in.Reason, in.By).Scan(&res.DecisionID); err != nil {
		return res, fmt.Errorf("override: log: %w", err)
	}

	// The record's numbers, again, over effective determinations.
	var formula string
	var versions []byte
	if err := tx.QueryRow(ctx, `
    SELECT formula_version, versions FROM match_records WHERE run_id = $1::uuid FOR UPDATE`,
		res.RunID).Scan(&formula, &versions); err != nil {
		return res, fmt.Errorf("override: read record: %w", err)
	}
	_, fraction := flattenVersions(versions)
	rows, err := tx.Query(ctx, `
    SELECT j.effective_determination, q.weight, q.type, q.category, q.class, q.included,
           COALESCE((j.provenance_summary->>'candidate_response')::int, 0) > 0,
           j.override_determination IS NOT NULL
    FROM judgments j
    JOIN requirements q ON q.id = j.requirement_id
    WHERE j.run_id = $1::uuid`, res.RunID)
	if err != nil {
		return res, fmt.Errorf("override: read judgments: %w", err)
	}
	var items []Item
	for rows.Next() {
		var it Item
		var typ string
		var included, overridden bool
		if err := rows.Scan(&it.Determination, &it.Weight, &typ, &it.Category, &it.Class, &included, &it.SelfAttested, &overridden); err != nil {
			rows.Close()
			return res, fmt.Errorf("override: read judgments: %w", err)
		}
		if typ == TypeBehavioral {
			it.Weight = 0
		}
		it.Excluded = !included
		if overridden {
			res.OverrideCount++
		}
		items = append(items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, fmt.Errorf("override: read judgments: %w", err)
	}
	if c, ok := RecordCoverage(formula, items, fraction); ok {
		if _, err := tx.Exec(ctx, `
      UPDATE match_records
      SET evidenced = $2, unresolved = $3, unmet = $4,
          coverage_equivalent_share = $5, self_attested_share = $6, override_count = $7
      WHERE run_id = $1::uuid`,
			res.RunID, c.Evidenced, c.Unresolved, c.Unmet, c.EquivalentShare, c.SelfAttestedShare, res.OverrideCount); err != nil {
			return res, fmt.Errorf("override: write record: %w", err)
		}
	} else if _, err := tx.Exec(ctx, `
      UPDATE match_records SET override_count = $2 WHERE run_id = $1::uuid`, res.RunID, res.OverrideCount); err != nil {
		return res, fmt.Errorf("override: write record: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return res, fmt.Errorf("override: commit: %w", err)
	}
	return res, nil
}

// DecisionKindOverride is the decision_log kind of a person's override
// of a judgment (14 §3.3).
const DecisionKindOverride = "hf_override"
