package hiring

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

// Writer is the dual write of Phase 0 steps 3 and 4 (14 §4): after the
// JD pipeline has stored its jsonb assessment, the same facts are
// written as rows in the framework's tables, and the jsonb stays
// authoritative until the hand-over. Since step 4 the judge emits a
// determination, a basis and the facts, so judgment rows are written
// too and the match record carries formula v3's three numbers; a run
// from before that (no determinations) still writes the v2 record.
type Writer struct {
	repo *Repo
	log  *slog.Logger
}

// NewWriter builds a writer over the repository.
func NewWriter(log *slog.Logger, repo *Repo) *Writer {
	return &Writer{repo: repo, log: log}
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
	// it ("v2:9f1c2e7a"); the extractor's is copied into each
	// requirement's provenance.
	Prompts           map[string]string
	ExtractorPromptID string
	// JudgePromptID and JudgePromptVersion name the judge whose output
	// the judgment rows hold; its fingerprint is read from Prompts.
	JudgePromptID      string
	JudgePromptVersion int
	Embedder           string
	CorpusFingerprint  string
	Requirements       []RequirementInput
	Judgments          []JudgmentInput
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
	roleID, err := w.repo.EnsureRole(ctx, in.SubmissionID, trimTitle(in.Title))
	if err != nil {
		return err
	}

	// Requirements, typed by the heuristic until the extractor types
	// them (step 5), with the provenance saying so.
	extractorFP := in.Prompts[in.ExtractorPromptID]
	reqs := make([]RequirementInput, 0, len(in.Requirements))
	for _, r := range in.Requirements {
		if r.LocalID == "" {
			continue
		}
		if r.Type == "" {
			r.Type = InferType(r.Text, r.NamedParties)
		}
		if len(r.Extraction) == 0 {
			prov, _ := json.Marshal(map[string]any{
				"prompt_id":          in.ExtractorPromptID,
				"prompt_fingerprint": extractorFP,
				"model":              in.Model,
				"type_rule":          TypeRule,
			})
			r.Extraction = prov
		}
		reqs = append(reqs, r)
	}
	rows, err := w.repo.SyncRequirementVersions(ctx, roleID, reqs)
	if err != nil {
		return err
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
	rowByLocal := map[string]RequirementRow{}
	for _, q := range rows {
		rowByLocal[q.LocalID] = q
	}
	weightByLocal := map[string]int{}
	categoryByLocal := map[string]string{}
	for _, r := range reqs {
		weightByLocal[r.LocalID] = r.Weight
		categoryByLocal[r.LocalID] = r.Category
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
		weight := weightByLocal[j.LocalID]
		if j.NotAssessed {
			weight = 0
		}
		coverageItems = append(coverageItems, Item{Weight: weight, Category: categoryByLocal[j.LocalID], Determination: j.Determination})
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
		for _, r := range reqs {
			var texts []string
			for _, c := range in.Offered[r.LocalID] {
				texts = append(texts, c.Text)
			}
			found, terms := KeywordBaseline(r.Text, texts)
			if terms == nil {
				terms = []string{}
			}
			items[r.LocalID] = map[string]any{"found": found, "terms": terms}
		}
	}
	kb, _ := json.Marshal(baseline)

	prev, err := w.repo.PreviousRunID(ctx, in.SubmissionID, in.Attempt)
	if err != nil {
		return err
	}
	// The record: formula v3's three numbers over the judgment rows
	// (FR-HF-06) when there are any, the v2 score otherwise. The gate
	// still reads jd_runs.match_score (v2) until D-32 is settled.
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
		formula, rules = FormulaV3, RulesVersion
		c := CoverageV3(coverageItems)
		record.Evidenced, record.Unresolved, record.Unmet = &c.Evidenced, &c.Unresolved, &c.Unmet
		record.EquivalentShare, record.SelfAttestedShare = &c.EquivalentShare, &c.SelfAttestedShare
	}
	record.FormulaVersion = formula
	record.Versions, _ = json.Marshal(map[string]any{
		"requirement_schema": RequirementSchema,
		"judgment_schema":    JudgmentSchema,
		"vocab":              VocabVersion,
		"formula":            formula,
		"rules":              rules,
		"keyword_rule":       KeywordRule,
		"type_rule":          TypeRule,
		"prompts":            sortedPrompts(in.Prompts),
		"model":              in.Model,
		"embedder":           in.Embedder,
		"corpus_fingerprint": in.CorpusFingerprint,
	})
	return w.repo.UpsertMatchRecord(ctx, record)
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
