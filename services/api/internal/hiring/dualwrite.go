package hiring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

// Writer writes a pipeline run as rows in the framework's tables.
//
// It began as the dual write of Phase 0 steps 3 and 4 (14 §4): the
// same facts the JD pipeline stores as a jsonb assessment, written
// again as roles, requirements, evidence items, judgments and a match
// record, with the jsonb authoritative until the hand-over.
//
// Since step 5 the requirement rows come first. Prepare writes the
// extraction as rows before anything is judged and hands the rows
// back, so what the judge is shown is the row, as extracted or as a
// person edited it, and every judgment names the version it judged
// (FR-HF-03). Record then writes the judgments and the match record
// against those rows.
type Writer struct {
	repo *Repo
	log  *slog.Logger
}

// NewWriter builds a writer over the repository.
func NewWriter(log *slog.Logger, repo *Repo) *Writer {
	return &Writer{repo: repo, log: log}
}

// Repo exposes the repository for the handlers that edit and release
// requirements.
func (w *Writer) Repo() *Repo {
	if w == nil {
		return nil
	}
	return w.repo
}

// Requirement sources, as recorded on a run.
const (
	// SourceExtraction: the run extracted the posting and wrote the
	// result as rows.
	SourceExtraction = "extraction"
	// SourceRows: the run judged the role's released rows without
	// extracting, because a person has edited them or asked for exactly
	// that.
	SourceRows = "rows"
)

// Prepared is what a run judges: the role and the released requirement
// rows, in reading order.
type Prepared struct {
	RoleID string
	Rows   []Requirement
	Source string
}

// Edited reports, for a submission, whether a run must judge the
// released rows instead of extracting: true when the role exists and a
// person has edited any of its current requirements.
func (w *Writer) Edited(ctx context.Context, submissionID int64) (bool, error) {
	if w == nil || w.repo == nil {
		return false, nil
	}
	roleID, ok, err := w.repo.RoleForSubmission(ctx, submissionID)
	if err != nil || !ok {
		return false, err
	}
	return w.repo.HumanEdited(ctx, roleID)
}

// Released returns the role's released rows for a run that does not
// extract (a re-judge, or any run on a role a person has edited).
func (w *Writer) Released(ctx context.Context, submissionID int64) (Prepared, error) {
	if w == nil || w.repo == nil {
		return Prepared{}, errors.New("hiring: no repository")
	}
	roleID, ok, err := w.repo.RoleForSubmission(ctx, submissionID)
	if err != nil {
		return Prepared{}, err
	}
	if !ok {
		return Prepared{}, fmt.Errorf("%w: the posting has no requirement rows yet", ErrNotFound)
	}
	rows, err := w.repo.ActiveRequirements(ctx, roleID)
	if err != nil {
		return Prepared{}, err
	}
	if len(rows) == 0 {
		return Prepared{}, fmt.Errorf("%w: the role has no released requirements", ErrNotFound)
	}
	return Prepared{RoleID: roleID, Rows: rows, Source: SourceRows}, nil
}

// Prepare writes an extraction as released requirement rows and
// returns them (FR-HF-02): each requirement typed, classed, with its
// thresholds, its evidence standard and its equivalence notes. A
// requirement whose judged fields are unchanged since the last run
// keeps its row. Refuses with ErrHumanEdited when a person has edited
// the role's requirements; the caller then judges Released instead.
func (w *Writer) Prepare(ctx context.Context, submissionID int64, title string, in []RequirementInput, prov Provenance) (Prepared, error) {
	if w == nil || w.repo == nil {
		return Prepared{}, errors.New("hiring: no repository")
	}
	roleID, err := w.repo.EnsureRole(ctx, submissionID, trimTitle(title))
	if err != nil {
		return Prepared{}, err
	}
	// The catalog is read per run, so an entry added or retired takes
	// effect on the next run without a restart. A catalog that cannot be
	// read costs the notes, not the run.
	catalog, err := w.repo.LoadCatalog(ctx)
	if err != nil {
		w.log.Warn("hiring: catalog unavailable; requirements drafted without equivalence notes", slog.String("error", err.Error()))
		catalog = nil
	}
	drafts := make([]Requirement, 0, len(in))
	for _, r := range in {
		if r.LocalID == "" {
			continue
		}
		drafts = append(drafts, Draft(r, catalog, prov))
	}
	rows, err := w.repo.SyncRequirementVersions(ctx, roleID, drafts)
	if err != nil {
		return Prepared{}, err
	}
	return Prepared{RoleID: roleID, Rows: rows, Source: SourceExtraction}, nil
}

// Chunk is one corpus chunk as offered to the judge: id and text, the
// two things the evidence items and the keyword baseline need.
type Chunk struct {
	ID   int64
	Text string
}

// JudgmentInput is one settled judgment as the pipeline hands it over:
// which requirement, the three-way verdict (kept for the v2 record and
// checked against the derived verdict), the chunks it cited, and since
// step 4 the determination, basis, uncertainty, facts and adjustments
// that make a judgment row. A judgment with no Determination is a
// pre-v13 one and writes no row.
type JudgmentInput struct {
	LocalID          string
	Verdict          string
	EvidenceChunkIDs []int64

	Determination   string
	Basis           string
	Uncertainty     []string
	Rationale       string
	ReportedFacts   json.RawMessage
	Confidence      *float64
	RuleAdjustments json.RawMessage
	RationaleIssues json.RawMessage
	// DecisionID is the decision_log row holding the prompt and the raw
	// response; 0 when the log write failed or the row was not judged.
	DecisionID int64
	// NotAssessed marks a behavioral requirement (FR-HF-18): weight 0 in
	// the numbers, no model call.
	NotAssessed bool
}

// RunInput is one finished pipeline run as the scorer saw it.
type RunInput struct {
	SubmissionID int64
	Title        string
	RunID        string
	Attempt      int
	Score        float64
	Model        string
	// Prompts is the fingerprint per prompt id, as jd_runs.prompts holds
	// it ("v2:9f1c2e7a").
	Prompts map[string]string
	// JudgePromptID and JudgePromptVersion name the judge whose output
	// the judgment rows hold; its fingerprint is read from Prompts.
	JudgePromptID      string
	JudgePromptVersion int
	Embedder           string
	CorpusFingerprint  string
	// Rows are the requirement rows the run judged, as Prepare or
	// Released returned them. When empty (a run whose rows could not be
	// written before judging), Drafts, the requirements exactly as they
	// were judged, are written here instead, after the fact, as the dual
	// write always did.
	Rows   []Requirement
	Drafts []Requirement
	// RequirementsSource says whether the run extracted or judged the
	// released rows (SourceExtraction, SourceRows).
	RequirementsSource string
	// PreferredFraction is the share of its weight a preferred item
	// counts at (FR-HF-23); 0 means the default.
	PreferredFraction float64
	Judgments         []JudgmentInput
	// Offered lists, per requirement local id, the chunks the judge was
	// shown, cited or not; the keyword baseline runs over all of them,
	// because the comparison is "would a filter over the same evidence
	// have found it", not "over the evidence the judge chose".
	Offered map[string][]Chunk
	// KeywordSkipped, when set, says why the baseline could not be
	// computed (the chunk texts could not be read back). The record is
	// still written, with no baseline items and this reason, rather
	// than an all-false baseline that would read as a complete keyword
	// miss.
	KeywordSkipped string
}

// Record writes the run as rows. Errors are returned for the caller to
// log; nothing here can fail the member's review, which was persisted
// before this was called.
func (w *Writer) Record(ctx context.Context, in RunInput) error {
	if w == nil || w.repo == nil {
		return nil
	}
	if in.RunID == "" {
		return fmt.Errorf("hiring: record: run id is empty")
	}
	rows, roleID := in.Rows, ""
	if len(rows) > 0 {
		roleID = rows[0].RoleID
	} else {
		// The rows were not written before judging: write the drafts as
		// they were judged now, so the judgments still have versions to
		// name.
		var err error
		roleID, err = w.repo.EnsureRole(ctx, in.SubmissionID, trimTitle(in.Title))
		if err != nil {
			return err
		}
		rows, err = w.repo.SyncRequirementVersions(ctx, roleID, in.Drafts)
		if errors.Is(err, ErrHumanEdited) {
			// A person edited the role while this run was judging its own
			// extraction. The edit stands and this run's judgments have no
			// rows to name; the next run judges the edited rows.
			return fmt.Errorf("hiring: record: %w; this run judged its own extraction and is not written as rows", err)
		}
		if err != nil {
			return err
		}
	}
	reqIDs := make([]string, 0, len(rows))
	for _, q := range rows {
		reqIDs = append(reqIDs, q.ID)
	}

	// Evidence items for every chunk a judgment cited, lazily.
	var cited []int64
	for _, j := range in.Judgments {
		for _, id := range j.EvidenceChunkIDs {
			if !slices.Contains(cited, id) {
				cited = append(cited, id)
			}
		}
	}
	evidence, err := w.repo.EnsureChunkEvidence(ctx, cited)
	if err != nil {
		return err
	}

	// Judgment rows (FR-HF-04), when the judge emitted the vocabulary.
	rowByLocal := map[string]Requirement{}
	for _, q := range rows {
		rowByLocal[q.LocalID] = q
	}
	var judgmentRows []JudgmentRow
	var coverageItems []Item
	for _, j := range in.Judgments {
		if j.Determination == "" {
			continue
		}
		q, ok := rowByLocal[j.LocalID]
		if !ok {
			continue
		}
		var evIDs []string
		kinds := map[string]int{}
		for _, id := range j.EvidenceChunkIDs {
			if ref, ok := evidence[id]; ok {
				evIDs = append(evIDs, ref.ID)
				kinds[ref.SourceKind]++
			}
		}
		summary, _ := json.Marshal(kinds)
		row := JudgmentRow{
			RequirementID: q.ID, LocalID: j.LocalID,
			Determination: j.Determination, Basis: j.Basis, Uncertainty: j.Uncertainty,
			EvidenceIDs: evIDs, Rationale: j.Rationale,
			ReportedFacts: j.ReportedFacts, Confidence: j.Confidence,
			RuleAdjustments: j.RuleAdjustments, RationaleIssues: j.RationaleIssues,
			ProvenanceSummary: summary,
			DerivedVerdict:    DeriveVerdict(j.Determination),
			PromptID:          in.JudgePromptID, PromptVersion: in.JudgePromptVersion,
			PromptFingerprint: in.Prompts[in.JudgePromptID], Model: in.Model,
		}
		if j.DecisionID > 0 {
			id := j.DecisionID
			row.DecisionID = &id
		}
		judgmentRows = append(judgmentRows, row)
		weight := q.Weight
		if j.NotAssessed {
			weight = 0
		}
		coverageItems = append(coverageItems, Item{
			Weight: weight, Category: q.Category, Class: q.Class, Excluded: !q.Included,
			Determination: j.Determination,
		})
	}
	if len(judgmentRows) > 0 {
		if err := w.repo.ReplaceJudgments(ctx, in.RunID, judgmentRows); err != nil {
			return err
		}
	}

	// The keyword baseline over the offered evidence (FR-HF-12).
	items := map[string]any{}
	baseline := map[string]any{"rule": KeywordRule, "items": items}
	if in.KeywordSkipped != "" {
		baseline["skipped"] = in.KeywordSkipped
	} else {
		for _, q := range rows {
			var texts []string
			for _, c := range in.Offered[q.LocalID] {
				texts = append(texts, c.Text)
			}
			found, terms := KeywordBaseline(q.Text, texts)
			if terms == nil {
				terms = []string{}
			}
			items[q.LocalID] = map[string]any{"found": found, "terms": terms}
		}
	}
	kb, _ := json.Marshal(baseline)

	prev, err := w.repo.PreviousRunID(ctx, in.SubmissionID, in.Attempt)
	if err != nil {
		return err
	}
	// The record: formula v4's three numbers over the judgment rows
	// (FR-HF-06, FR-HF-23) when there are any, the v2 score otherwise.
	// The gate still reads jd_runs.match_score (v2) until D-32 is
	// settled.
	formula, rules := FormulaV2, ""
	score := in.Score
	record := MatchRecord{
		RunID:           in.RunID,
		RoleID:          roleID,
		CandidateID:     OwnerCandidateID,
		Attempt:         in.Attempt,
		SupersedesRunID: prev,
		RequirementIDs:  reqIDs,
		KeywordBaseline: kb,
		Evidenced:       &score,
	}
	if len(judgmentRows) > 0 {
		formula, rules = FormulaV4, RulesVersion
		c := CoverageV4(coverageItems, in.PreferredFraction)
		record.Evidenced, record.Unresolved, record.Unmet = &c.Evidenced, &c.Unresolved, &c.Unmet
		record.EquivalentShare, record.SelfAttestedShare = &c.EquivalentShare, &c.SelfAttestedShare
	}
	record.FormulaVersion = formula
	record.Versions, _ = json.Marshal(map[string]any{
		"requirement_schema":  RequirementSchema,
		"judgment_schema":     JudgmentSchema,
		"vocab":               VocabVersion,
		"formula":             formula,
		"rules":               rules,
		"keyword_rule":        KeywordRule,
		"type_rule":           TypeRule,
		"thresholds_rule":     ThresholdsRule,
		"inclusion_rule":      InclusionRule,
		"templates":           TemplateVersion,
		"preferred_fraction":  preferredFraction(in.PreferredFraction),
		"requirements_source": in.RequirementsSource,
		"prompts":             sortedPrompts(in.Prompts),
		"model":               in.Model,
		"embedder":            in.Embedder,
		"corpus_fingerprint":  in.CorpusFingerprint,
	})
	if err := w.repo.UpsertMatchRecord(ctx, record); err != nil {
		return err
	}
	// A person's verdicts on the run before this one stand on the
	// requirements that have not changed (FR-HF-08). The run's own
	// numbers were written first, so a failure here leaves a record that
	// is right about the model and only lacks the carry.
	if len(judgmentRows) > 0 {
		carried, err := w.repo.CarryOverrides(ctx, in.RunID, in.SubmissionID, in.Attempt)
		if err != nil {
			return err
		}
		if carried > 0 && w.log != nil {
			w.log.Info("hiring: overrides carried to the new run", slog.String("run", in.RunID), slog.Int("carried", carried))
		}
	}
	return nil
}

// preferredFraction is the fraction the formula used, for the record.
func preferredFraction(f float64) float64 {
	if f <= 0 || f > 1 {
		return DefaultPreferredFraction
	}
	return f
}

func trimTitle(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	// Cut on a rune boundary: a posting's first line is full of dashes
	// and curly quotes, and a string cut inside one of them is invalid
	// UTF-8 that Postgres refuses, which would cost the run every row.
	if len(s) > 160 {
		cut := 160
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		s = s[:cut]
	}
	return s
}

func sortedPrompts(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]string, len(m))
	for _, k := range keys {
		out[k] = m[k]
	}
	return out
}
