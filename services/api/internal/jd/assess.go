package jd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/reh3376/career-site/services/api/internal/hiring"
	"github.com/reh3376/career-site/services/api/internal/ingest"
	"github.com/reh3376/career-site/services/api/internal/llm"
	"github.com/reh3376/career-site/services/api/internal/prompts"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// EvidencePerRequirement is how many corpus chunks each requirement
// is judged against. Four is the setting the calibration set was
// scored with (0.79 / 0.36 / 0.00 / 0.00); reducing it moved the
// strong JD below the gate. Context pressure is handled by batching
// requirements per call, not by thinning evidence.
const EvidencePerRequirement = 4

// ProfileKind is the source_kind of the owner's career facts sheet,
// offered to every requirement (see Assess). ProfileChunksMax bounds
// how much of it is added per call; the sheet is meant to be one or
// two chunks.
const (
	ProfileKind      = prompts.ProfileSourceKind
	ProfileChunksMax = 2
)

// withProfile puts the profile chunks first in a requirement's
// evidence, skipping any the open search already returned.
func withProfile(profile, hits []users.CorpusHit) []users.CorpusHit {
	if len(profile) == 0 {
		return hits
	}
	seen := map[int64]bool{}
	for _, p := range profile {
		seen[p.Chunk.ID] = true
	}
	out := make([]users.CorpusHit, 0, len(profile)+len(hits))
	out = append(out, profile...)
	for _, h := range hits {
		if !seen[h.Chunk.ID] {
			out = append(out, h)
		}
	}
	return out
}

// ScoreFormula identifies the current score arithmetic in stored
// assessments. v1 was a plain weighted mean over every requirement;
// v2 keeps unmet "nice" requirements out of the denominator.
const ScoreFormula = "v2-nice-bonus"

// ErrMonthlyCap is returned when the LLM call cap for the current
// calendar month has been reached (Phase 4 guardrail #8).
var ErrMonthlyCap = errors.New("llm monthly call cap reached")

// rawJudgment is the model's judgment as decoded, before validation:
// the v13 output schema (docs/framework-research/04 §2.3).
type rawJudgment struct {
	RequirementID string   `json:"requirement_id"`
	Determination string   `json:"determination"`
	Basis         string   `json:"basis"`
	Uncertainty   []string `json:"uncertainty"`
	EvidenceIDs   []string `json:"evidence_ids"`
	// ReportedFacts are what the model found for the code rules to
	// compare: the span, the latest year, the scale, the credential, the
	// organisations named. Reported by the model, decided in hiring/rules.go.
	ReportedFacts hiring.ReportedFacts `json:"reported_facts"`
	Rationale     string               `json:"rationale"`
	// Confidence is the model's self-report, logged for calibration and
	// never used in arithmetic (FR-HF-05).
	Confidence *float64 `json:"confidence,omitempty"`
}

// callResult is what one gateway call returned, kept for the ledger
// and the decision log.
type callResult struct {
	Model            string
	Text             string
	PromptTokens     int32
	CompletionTokens int32
	LatencyMs        int64
}

// Judgment is the settled judgment of one requirement: the model's
// determination as the code rules left it, the three-way verdict
// derived from it (FR-HF-07), and everything a reader needs to see how
// it got there.
type Judgment struct {
	RequirementID string `json:"requirement_id"`
	// Verdict is met | partial | unmet, derived in code from the
	// determination so the owner's graded series and every reader of the
	// stored assessment keep working across the vocabulary change.
	Verdict string `json:"verdict"`
	// Determination, Basis and Uncertainty are the v1 vocabulary (03 §2),
	// after the code rules. Type is the requirement's type as judged.
	Determination string   `json:"determination,omitempty"`
	Basis         string   `json:"basis,omitempty"`
	Uncertainty   []string `json:"uncertainty,omitempty"`
	Type          string   `json:"type,omitempty"`
	EvidenceIDs   []int64  `json:"evidence_ids"`
	Rationale     string   `json:"rationale"`
	// ReportedFacts are kept whole so a reader can see what every
	// comparison was made against.
	ReportedFacts *hiring.ReportedFacts `json:"reported_facts,omitempty"`
	Confidence    *float64              `json:"confidence,omitempty"`
	// StatedSpanYears and PartiesEvidenced are copied out of
	// ReportedFacts for the admin pages that read them by name.
	StatedSpanYears  float64  `json:"stated_span_years,omitempty"`
	PartiesEvidenced []string `json:"parties_evidenced,omitempty"`
	// RuleAdjustments are the changes code made to the model's answer,
	// each with its rule and version (FR-HF-05). Empty when the model's
	// output stood.
	RuleAdjustments []hiring.RuleAdjustment `json:"rule_adjustments,omitempty"`
	// Adjusted is the adjustments' reasons joined, for readers that show
	// one line. An adjustment that cannot be read is indistinguishable
	// from the model having said so itself.
	Adjusted string `json:"adjusted,omitempty"`
	// NotAssessed marks a behavioral requirement (FR-HF-18): recorded,
	// shown as not assessed, weight 0 in every number, never judged.
	NotAssessed bool `json:"not_assessed,omitempty"`
	// RationaleIssues are the ways the rationale disagrees with the
	// record it was written from. They never change the verdict. See
	// rationale.go.
	RationaleIssues []rationaleIssue `json:"rationale_issues,omitempty"`
}

// Coverage is the match record's arithmetic (FR-HF-06): three numbers
// over effective determinations that sum to one over the denominator,
// and two shares of the evidenced part. Formula v4 since step 5
// (FR-HF-23): required items at full weight, preferred at a fraction,
// ideal as bonus only, and a years or credential item counted only
// once its equivalency is defined. The résumé gate still reads the v2
// score until D-32 is settled by the golden run.
type Coverage struct {
	Evidenced         float64 `json:"evidenced"`
	Unresolved        float64 `json:"unresolved"`
	Unmet             float64 `json:"unmet"`
	EquivalentShare   float64 `json:"coverage_equivalent_share"`
	SelfAttestedShare float64 `json:"self_attested_share"`
	// Denominator is the weight the three numbers are fractions of;
	// fractional under v4, where a preferred item counts at a fraction.
	Denominator float64 `json:"denominator"`
	Formula     string  `json:"formula"`
	// PreferredFraction is the fraction this record was computed with,
	// and Excluded the requirement ids left out for want of a defined
	// equivalency, so the three numbers can be read without the rows.
	PreferredFraction float64  `json:"preferred_fraction,omitempty"`
	Excluded          []string `json:"excluded,omitempty"`
}

// Assessment is the auditable record behind a match score: what the
// posting asked for, what evidence was retrieved for each ask, what
// the model concluded, and the arithmetic that produced the score.
type Assessment struct {
	Requirements []prompts.Requirement `json:"requirements"`
	Judgments    []Judgment            `json:"judgments"`
	// EvidenceIDs lists, per requirement id, the chunk ids that were
	// offered to the judge (not just the ones it cited).
	EvidenceIDs map[string][]int64 `json:"evidence_ids"`
	Score       float64            `json:"score"`
	WeightTotal int                `json:"weight_total"`
	// ScoreFormula names the arithmetic (see ScoreFormula const) so a
	// stored assessment can be re-read after the formula changes.
	ScoreFormula string         `json:"score_formula,omitempty"`
	Model        string         `json:"model"`
	Prompts      map[string]int `json:"prompts"` // prompt id → version
	// JudgeBatches is how many model calls the judgment took (context
	// budgeting on a small box splits it).
	JudgeBatches int `json:"judge_batches,omitempty"`
	// Coverage is formula v3's three numbers (FR-HF-06), beside the v2
	// Score the gate reads. Nil when the assessor did not run.
	Coverage *Coverage `json:"coverage,omitempty"`
	// ConsistencyRetries counts judge calls sent back once for breaking
	// a consistency rule (04 §2.3); a repair after a second failure is
	// recorded on the judgment itself.
	ConsistencyRetries int `json:"consistency_retries,omitempty"`
	// RequirementsSource says whether the run extracted the posting
	// ("extraction") or judged the role's released rows without
	// extracting ("rows"), which is every run after a person edits them.
	RequirementsSource string `json:"requirements_source,omitempty"`
	// Error is set when the pipeline could not complete and the caller
	// fell back to the retrieval score.
	Error string `json:"error,omitempty"`
	// DecisionIDs maps requirement id to the decision_log row that holds
	// the judgment's prompt and response, for the judgment rows the
	// hiring writer creates. In-process only; not part of the stored
	// assessment.
	DecisionIDs map[string]int64 `json:"-"`
}

// Assessor runs the requirements → retrieval → judgment pipeline.
type Assessor struct {
	log        *slog.Logger
	users      *users.Repo
	embed      ingest.EmbedClient
	llm        llm.Client
	monthlyCap int64 // 0 = unlimited
	// numCtx is the model context window the sidecar requests. The
	// judgment prompt is split into batches that fit it, so the same
	// pipeline runs on an 8 GB box (8k) and a workstation (16k+).
	numCtx int
	// judge is the requirement judge as rendered for the vocabulary in
	// use (prompts.RequirementJudgeFor); vocab is that vocabulary, which
	// the consistency rules validate against. Defaults to the compiled-in
	// vocabulary 1.0; main swaps in the registry's rendering at boot.
	judge prompts.Prompt
	vocab hiring.VocabLists
	// extractor is the requirements extractor as rendered for the same
	// vocabulary (prompts.JDRequirementsFor): its type codes come from
	// the registry too.
	extractor prompts.Prompt
	// settings holds the preferred fraction formula v4 counts a
	// preferred item at; nil reads the default.
	settings *hiring.SettingsStore
	// now is the clock the recency rule reads; tests pin it.
	now func() time.Time
}

// SetVocab renders the judge for a vocabulary, usually the one loaded
// from vocab_codes at boot, so a registry change is a new prompt
// fingerprint rather than a code change (14 §5).
func (a *Assessor) SetVocab(v hiring.VocabLists) {
	if a == nil {
		return
	}
	a.vocab = v
	a.judge = prompts.RequirementJudgeFor(v)
	a.extractor = prompts.JDRequirementsFor(v)
}

// SetSettings wires the store formula v4 reads its fraction from.
func (a *Assessor) SetSettings(s *hiring.SettingsStore) {
	if a != nil {
		a.settings = s
	}
}

// ExtractorPrompt is the extractor as this assessor sends it, for run
// records and requirement provenance.
func (a *Assessor) ExtractorPrompt() prompts.Prompt {
	if a == nil || a.extractor.ID == "" {
		return prompts.JDRequirements
	}
	return a.extractor
}

// JudgePrompt is the judge as this assessor sends it, for run records
// and the decision log.
func (a *Assessor) JudgePrompt() prompts.Prompt {
	if a == nil {
		return prompts.RequirementJudge
	}
	return a.judge
}

// NumCtx is the context window this assessor asks the sidecar for. The
// run record keeps it because the same model at a different context
// size is, in practice, a different judge: truncation changed a score
// by 0.6 once already (docs/llm-tuning-log.md, 2026-09-21).
func (a *Assessor) NumCtx() int {
	if a == nil {
		return 0
	}
	return a.numCtx
}

// judgeReserve is the context kept free for the system prompt and the
// JSON verdicts when sizing a judgment batch.
const judgeReserve = 1800

// judgeBatchMax caps how many requirements share one judge call. The
// verdicts qwen3:14b reaches for a requirement change with how many
// other requirements are in the same prompt (the calibration JD moved
// from 0.79 in one 12-requirement call to 0.625 in three or four
// smaller ones with identical evidence). One requirement per call
// makes every verdict independent of grouping, so an 8k box and a
// 16k workstation reach the same score. See docs/llm-tuning-log.md.
const judgeBatchMax = 1

// NewAssessor wires the deps. numCtx <= 0 falls back to 16384.
func NewAssessor(log *slog.Logger, repo *users.Repo, embed ingest.EmbedClient, client llm.Client, monthlyCap int64, numCtx int) *Assessor {
	if numCtx <= 0 {
		numCtx = 16384
	}
	return &Assessor{
		log: log, users: repo, embed: embed, llm: client, monthlyCap: monthlyCap, numCtx: numCtx,
		judge: prompts.RequirementJudge, extractor: prompts.JDRequirements, vocab: hiring.DefaultVocab(), now: time.Now,
	}
}

// batchRequirements groups requirements so each rendered judgment
// prompt stays under the context budget. A single oversized
// requirement still gets its own batch (the renderer caps chunk text).
func (a *Assessor) batchRequirements(reqs []prompts.Requirement, evidence map[string][]users.CorpusHit, profile []users.CorpusHit) [][]prompts.Requirement {
	budget := a.numCtx - judgeReserve - prompts.EstimateTokens(a.judgeOrDefault().System)
	var batches [][]prompts.Requirement
	var cur []prompts.Requirement
	for _, r := range reqs {
		trial := append(append([]prompts.Requirement{}, cur...), r)
		if len(cur) >= judgeBatchMax || (len(cur) > 0 && prompts.EstimateTokens(prompts.RenderJudgeUser(trial, evidence, profile)) > budget) {
			batches = append(batches, cur)
			cur = []prompts.Requirement{r}
			continue
		}
		cur = trial
	}
	if len(cur) > 0 {
		batches = append(batches, cur)
	}
	return batches
}

// Progress receives pipeline progress (0 to 100) with a short stage
// label. Nil is allowed.
type Progress func(pct int32, stage string)

func (p Progress) report(pct int32, stage string) {
	if p != nil {
		p(pct, stage)
	}
}

// Assess produces the assessment for one JD without the requirement
// rows: extract, draft each requirement in code (type, thresholds,
// evidence standard; no catalog, so no equivalence notes), and judge.
// The scorer does the same with the rows written in between (Extract,
// then hiring.Writer.Prepare, then Judge); this is the path for a run
// whose rows could not be written, and for tests.
func (a *Assessor) Assess(ctx context.Context, submissionID int64, jdText string, hints prompts.Hints, progress Progress) (*Assessment, error) {
	extracted, model, err := a.Extract(ctx, submissionID, jdText, hints, progress)
	if err != nil {
		return nil, err
	}
	return a.Judge(ctx, submissionID, DraftRequirements(extracted, nil, hiring.Provenance{}), model, hiring.SourceExtraction, progress)
}

// requirementsTokenBudget is the extractor's output cap. The budget is
// arithmetic, not a guess. The schema allows 20 requirements, each
// carrying text up to 200 characters and a source_quote up to 300,
// plus id, category, weight, named_parties and, since v5, a type and
// up to six alternatives: about 640 characters when a requirement
// offers a choice and 580 when it does not, call it 180 tokens. Twenty
// of those is 3,600, and the JSON wrapper rounds it up.
//
// It was 1,200, which was ample until source_quote was added on
// 2026-09-25 and roughly tripled the output. Evaluation run 7 then
// lost two of eight postings to "unexpected end of JSON input",
// marginally rather than universally: a 4,682-character posting fit
// and a 4,903-character one did not. A cap costs nothing when it is
// not reached, so it is set where a full-length answer fits rather
// than where a typical one does. 4,000 until v5; 4,800 with the two
// fields v5 adds.
const requirementsTokenBudget = 4800

// Extract runs the requirements call and validates what came back: the
// posting structured into requirements, each with its verified source
// quote, the type the extractor gave it and the alternatives the
// posting offers (FR-HF-02). Nothing here is a requirement row yet;
// the caller drafts the rows from this. Returns the model that
// answered, for the run record.
func (a *Assessor) Extract(ctx context.Context, submissionID int64, jdText string, hints prompts.Hints, progress Progress) ([]prompts.Requirement, string, error) {
	if err := a.checkCap(ctx); err != nil {
		return nil, "", err
	}
	progress.report(5, "reading the posting")
	var reqDoc struct {
		Requirements []prompts.Requirement `json:"requirements"`
	}
	reqCall, err := a.call(ctx, submissionID, a.ExtractorPrompt(),
		prompts.RenderRequirementsUser(jdText, hints), requirementsTokenBudget, &reqDoc)
	if err != nil {
		return nil, "", fmt.Errorf("requirements: %w", err)
	}
	reqs, err := validateRequirements(reqDoc.Requirements, jdText)
	if err != nil {
		return nil, "", fmt.Errorf("requirements: %w", err)
	}
	return reqs, reqCall.Model, nil
}

// ToInput is an extracted requirement as the hiring package drafts a
// row from it.
func ToInput(r prompts.Requirement) hiring.RequirementInput {
	return hiring.RequirementInput{
		LocalID: r.ID, Text: r.Text, SourceQuote: r.SourceQuote, Category: r.Category, Weight: r.Weight,
		NamedParties: r.NamedParties, Type: r.Type, Alternatives: r.Alternatives,
	}
}

// FromRow is a requirement row as the judge is shown it and the
// assessment stores it.
func FromRow(q hiring.Requirement) prompts.Requirement {
	thresholds := q.Thresholds
	included := q.Included
	return prompts.Requirement{
		ID: q.LocalID, Text: q.Text, SourceQuote: q.SourceQuote, Category: q.Category, Weight: q.Weight,
		NamedParties: q.NamedParties, Type: q.Type, Alternatives: q.Alternatives,
		RowID: q.ID, Version: q.Version, Class: q.Class, Thresholds: &thresholds,
		EvidenceStandard: q.EvidenceStandard, EquivalenceNotes: q.EquivalenceNotes, Equivalency: q.Equivalency,
		StandardEdited: strings.HasPrefix(q.EvidenceStandardTemplate, hiring.EditedTemplatePrefix),
		Included:       &included, Edited: q.EditedBy > 0,
	}
}

// FromRows converts a run's rows in order.
func FromRows(rows []hiring.Requirement) []prompts.Requirement {
	out := make([]prompts.Requirement, 0, len(rows))
	for _, q := range rows {
		out = append(out, FromRow(q))
	}
	return out
}

// DraftRequirements drafts extracted requirements in code without
// writing rows, for a run that has none: the same type, thresholds,
// evidence standard and inclusion a row would carry, so the judge is
// shown the same thing either way.
func DraftRequirements(extracted []prompts.Requirement, catalog *hiring.Catalog, prov hiring.Provenance) []prompts.Requirement {
	out := make([]prompts.Requirement, 0, len(extracted))
	for _, r := range extracted {
		out = append(out, FromRow(hiring.Draft(ToInput(r), catalog, prov)))
	}
	return out
}

// Drafts is DraftRequirements as the rows the writer would store, for
// a run that judged drafts and writes them afterwards.
func Drafts(extracted []prompts.Requirement, catalog *hiring.Catalog, prov hiring.Provenance) []hiring.Requirement {
	out := make([]hiring.Requirement, 0, len(extracted))
	for _, r := range extracted {
		out = append(out, hiring.Draft(ToInput(r), catalog, prov))
	}
	return out
}

// Judge produces the assessment for requirements that are already
// settled: retrieval for each, one judge call per requirement, the code
// rules, and the arithmetic. reqs are the released requirement rows
// (FromRows), as extracted or as a person edited them, so what is
// judged is what the row says (FR-HF-03). Every call lands in
// llm_usage. Progress is reported from 12% to 75%; the scorer owns the
// rest (résumé, PDF).
func (a *Assessor) Judge(ctx context.Context, submissionID int64, reqs []prompts.Requirement, model, source string, progress Progress) (*Assessment, error) {
	if len(reqs) == 0 {
		return nil, errors.New("judge: no requirements")
	}
	if err := a.checkCap(ctx); err != nil {
		return nil, err
	}
	out := &Assessment{
		EvidenceIDs: map[string][]int64{},
		Prompts: map[string]int{
			a.ExtractorPrompt().ID: a.ExtractorPrompt().Version,
			a.judgeOrDefault().ID:  a.judgeOrDefault().Version,
		},
		DecisionIDs:        map[string]int64{},
		Model:              model,
		RequirementsSource: source,
	}
	// A requirement that reached here untyped (a provider that ignored
	// the schema, a row from before the extractor typed them) is typed
	// by the heuristic: the judge sees the type, the rules read it, and
	// behavioral rows are never sent to the judge (FR-HF-18).
	reqs = append([]prompts.Requirement{}, reqs...)
	for i := range reqs {
		if reqs[i].Type == "" {
			reqs[i].Type = hiring.InferType(reqs[i].Text, reqs[i].NamedParties)
		}
	}
	out.Requirements = reqs

	progress.report(12, fmt.Sprintf("%d requirements found; gathering evidence", len(reqs)))

	// 2. Retrieval per requirement (one batched embed call).
	texts := make([]string, len(reqs))
	for i, r := range reqs {
		texts[i] = r.Text
	}
	vectors, _, err := a.embed.Embed(ctx, texts, ingest.PurposeQuery)
	if err != nil {
		return nil, fmt.Errorf("embed requirements: %w", err)
	}
	if len(vectors) != len(reqs) {
		return nil, errors.New("embed requirements: vector count mismatch")
	}
	// The career facts sheet (source_kind "profile") is offered to every
	// requirement. Embedding retrieval is unreliable for tenure, title,
	// degree and certification facts: a "2+ years leadership"
	// requirement once drew dev-doc notes about session "resume"
	// scoring, and the closest résumé chunk by cosine was the
	// publications section. The sheet is short and owner-maintained
	// (docs/decision-log.md, docs/llm-tuning-log.md 2026-09-22).
	profile, err := a.users.ListChunksByKind(ctx, ProfileKind, ProfileChunksMax)
	if err != nil {
		a.log.Warn("profile chunks unavailable", slog.String("error", err.Error()))
	}
	evidence := make(map[string][]users.CorpusHit, len(reqs))
	for i, r := range reqs {
		hits, err := a.users.SearchCorpus(ctx, vectors[i], EvidencePerRequirement)
		if err != nil {
			return nil, fmt.Errorf("retrieve %s: %w", r.ID, err)
		}
		hits = withProfile(profile, hits)
		evidence[r.ID] = hits
		for _, h := range hits {
			out.EvidenceIDs[r.ID] = append(out.EvidenceIDs[r.ID], h.Chunk.ID)
		}
	}

	// 3. Judgments, in batches sized to the context window.
	//
	// The career facts sheet is fetched once and passed to every judge
	// call unchanged. It has to be the same chunks in the same order
	// every time or the shared prefix is not shared and Ollama
	// re-evaluates the whole prompt on each call. Two chunks today.
	//
	// This path deliberately ignores internal/corpusscope, so a member's
	// submission sees the facts sheet even though the document is
	// corpus_only. Decided by the owner on 2026-09-24, and the reason
	// matters: the scope exists to stop a submitted posting steering
	// retrieval across client and NDA material. The facts sheet is about
	// the candidate himself, it is the same for every posting, and it
	// cannot be steered because it is not retrieved. A review without it
	// would be worse and no safer. The same exemption covers the master
	// résumé in resume.go.
	profile, pErr := a.users.ListChunksByKind(ctx, prompts.ProfileSourceKind, 8)
	if pErr != nil {
		// Not fatal: the judge still has the retrieved evidence, it just
		// pays full prompt evaluation on every call.
		a.log.Warn("judge: profile chunks unavailable, prefix caching lost",
			slog.Int64("jd_id", submissionID), slog.String("error", pErr.Error()))
		profile = nil
	}
	// Behavioral requirements are recorded, never judged (FR-HF-18):
	// scoring personality adjectives from documents is the opaque
	// culture-fit judgment the framework names as a root cause.
	var judgeable []prompts.Requirement
	for _, r := range reqs {
		if r.Type != "behavioral" {
			judgeable = append(judgeable, r)
		}
	}
	judge := a.judgeOrDefault()
	var all []rawJudgment
	// repairs names the requirements whose judgment broke a consistency
	// rule twice and is repaired in code rather than lost.
	repairs := map[string]bool{}
	batches := a.batchRequirements(judgeable, evidence, profile)
	if err := a.checkCapN(ctx, int64(len(batches))); err != nil {
		return nil, err
	}
	// Per requirement: which call produced its judgment, so the decision
	// log can carry the exact prompt and raw response for each one.
	calls := make([]callResult, 0, len(batches))
	userPrompts := make([]string, 0, len(batches))
	batchOf := map[string]int{}
	for bi, batch := range batches {
		progress.report(int32(15+60*bi/len(batches)), fmt.Sprintf("judging requirement %d of %d", bi+1, len(batches)))
		var judgeDoc struct {
			Judgments []rawJudgment `json:"judgments"`
		}
		user := prompts.RenderJudgeUser(batch, evidence, profile)
		res, err := a.call(ctx, submissionID, judge, user, 1200, &judgeDoc)
		if err != nil {
			return nil, fmt.Errorf("judge batch %d/%d: %w", bi+1, len(batches), err)
		}
		// The consistency rules of 04 §2.3, which the schema cannot
		// express. An answer that breaks one is sent back once with the
		// rule named; one that breaks it again is repaired in code and
		// the repair recorded, because losing a four-hour run to one
		// mislabelled field is worse than correcting it in the open.
		if broken := a.consistencyViolations(judgeDoc.Judgments, out.EvidenceIDs); len(broken) > 0 {
			out.ConsistencyRetries++
			a.log.Info("judge: answer broke a consistency rule, asking once more",
				slog.Int64("jd_id", submissionID), slog.Int("batch", bi+1), slog.Any("rules", broken))
			var again struct {
				Judgments []rawJudgment `json:"judgments"`
			}
			correction := user + "\n\n<correction>Your previous answer broke these rules: " + strings.Join(broken, "; ") + ". Answer again, keeping to the rules.</correction>"
			res2, err2 := a.call(ctx, submissionID, judge, correction, 1200, &again)
			if err2 == nil && len(again.Judgments) > 0 {
				res, user, judgeDoc.Judgments = res2, correction, again.Judgments
				if stillBroken := a.consistencyViolations(judgeDoc.Judgments, out.EvidenceIDs); len(stillBroken) > 0 {
					for _, j := range judgeDoc.Judgments {
						repairs[j.RequirementID] = true
					}
				}
			} else {
				// The retry failed to produce an answer; the first one is
				// repaired rather than the posting lost.
				for _, j := range judgeDoc.Judgments {
					repairs[j.RequirementID] = true
				}
			}
		}
		if out.Model == "" {
			// A run on released rows made no extraction call; the judge
			// is then the model of record.
			out.Model = res.Model
		}
		calls = append(calls, res)
		userPrompts = append(userPrompts, user)
		for _, r := range batch {
			batchOf[r.ID] = bi
		}
		all = append(all, judgeDoc.Judgments...)
	}
	out.JudgeBatches = len(batches)
	progress.report(76, "computing the score")
	rawByReq := map[string]rawJudgment{}
	byReq := map[string]Judgment{}
	reqByID := make(map[string]prompts.Requirement, len(reqs))
	for _, r := range reqs {
		reqByID[r.ID] = r
	}
	now := time.Now()
	if a.now != nil {
		now = a.now()
	}
	for _, j := range all {
		if _, dup := rawByReq[j.RequirementID]; !dup {
			rawByReq[j.RequirementID] = j
		}
		if _, dup := byReq[j.RequirementID]; dup {
			continue
		}
		req, ok := reqByID[j.RequirementID]
		if !ok {
			// A judgment for an id the posting does not have is a
			// malfunction, not an answer; it is logged and dropped.
			a.log.Warn("judge: judgment for an unknown requirement id dropped",
				slog.Int64("jd_id", submissionID), slog.String("id", j.RequirementID))
			continue
		}
		offered := map[int64]bool{}
		for _, id := range out.EvidenceIDs[j.RequirementID] {
			offered[id] = true
		}
		byReq[j.RequirementID] = settleJudgment(settleInput{
			req: req, raw: j, offered: offered, evidence: evidence[j.RequirementID],
			now: now, vocab: a.vocab, repair: repairs[j.RequirementID],
		})
	}
	for _, r := range reqs {
		if r.Type == "behavioral" {
			byReq[r.ID] = notAssessedJudgment(r)
		}
	}

	// 4. Score in code. A requirement the model skipped counts as unmet.
	//
	// Two formulas, from the derived three-way verdict and from the
	// determinations respectively. The v2 score (2026-09-22) is what the
	// résumé gate reads until D-32 is settled by the golden run: every
	// required requirement is in the denominator; any other joins it
	// only when it earned something, so a wish list can raise a score
	// and never sink it; verdict values met 1, partial 0.5, unmet 0;
	// floor: a posting with no required lines counts every requirement.
	// "Required" is the class the hiring manager set (FR-HF-23), which
	// for a requirement nobody has edited is the posting's must, so an
	// unedited posting scores exactly as it did. Formula v4 (FR-HF-06,
	// FR-HF-23) reports three numbers over the determinations, counts a
	// preferred item at a fraction and an ideal one as bonus only,
	// leaves out a years or credential item whose equivalency nobody has
	// defined, and never scores unresolved as half of anything.
	// Behavioral rows weigh nothing in either (FR-HF-18).
	anyRequired := false
	for _, r := range reqs {
		if r.Required() && r.Type != "behavioral" {
			anyRequired = true
			break
		}
	}
	var weighted float64
	var items []hiring.Item
	var excluded []string
	for _, r := range reqs {
		j, ok := byReq[r.ID]
		if !ok {
			j = Judgment{RequirementID: r.ID, Verdict: "unmet", Determination: hiring.DetNotMet, Basis: hiring.BasisNotAddressed,
				Uncertainty: []string{}, Type: r.Type, Rationale: "no judgment returned"}
		}
		out.Judgments = append(out.Judgments, j)
		w := r.Weight
		if j.NotAssessed {
			w = 0
		}
		v := verdictValue(j.Verdict)
		if w > 0 && (!anyRequired || r.Required() || v > 0) {
			out.WeightTotal += w
		}
		weighted += float64(w) * v
		items = append(items, hiring.Item{
			Weight: w, Category: r.Category, Class: r.Class, Excluded: !r.Counts(),
			Determination: j.Determination,
		})
		if !r.Counts() && w > 0 {
			excluded = append(excluded, r.ID)
		}
	}
	if out.WeightTotal > 0 {
		out.Score = weighted / float64(out.WeightTotal)
	}
	out.ScoreFormula = ScoreFormula
	fraction := a.settings.Get(ctx).PreferredFraction
	c := hiring.CoverageV4(items, fraction)
	out.Coverage = &Coverage{
		Evidenced: c.Evidenced, Unresolved: c.Unresolved, Unmet: c.Unmet,
		EquivalentShare: c.EquivalentShare, SelfAttestedShare: c.SelfAttestedShare,
		Denominator: c.Denominator, Formula: hiring.FormulaV4,
		PreferredFraction: fraction, Excluded: excluded,
	}

	// 5. Decision log: one row per verdict with everything the owner
	// needs to judge it himself (docs/decision-log.md). Best-effort.
	a.logVerdicts(ctx, submissionID, out, evidence, rawByReq, calls, userPrompts, batchOf)
	return out, nil
}

// judgeEvidenceText mirrors what RenderJudgeUser shows the model for a
// chunk: retrieved evidence capped, the facts sheet whole.
func judgeEvidenceText(h users.CorpusHit) string {
	if h.SourceKind == prompts.ProfileSourceKind {
		return h.Chunk.Text
	}
	return prompts.CapRunes(h.Chunk.Text, prompts.JudgeChunkRunes)
}

// logVerdicts writes one decision_log row per requirement.
func (a *Assessor) logVerdicts(
	ctx context.Context, submissionID int64, out *Assessment,
	evidence map[string][]users.CorpusHit, raw map[string]rawJudgment,
	calls []callResult, userPrompts []string, batchOf map[string]int,
) {
	type evidenceRow struct {
		ChunkID    int64   `json:"chunk_id"`
		SourceKind string  `json:"source_kind"`
		Access     string  `json:"access"`
		Title      string  `json:"title,omitempty"`
		Similarity float32 `json:"similarity"`
		Text       string  `json:"text"`
	}
	rows := make([]users.Decision, 0, len(out.Requirements))
	for _, r := range out.Requirements {
		var ev []evidenceRow
		for _, h := range evidence[r.ID] {
			access, title := "public", h.Title
			if h.Visibility == users.VisibilityCorpusOnly {
				access, title = "private", ""
			}
			ev = append(ev, evidenceRow{
				ChunkID: h.Chunk.ID, SourceKind: h.SourceKind, Access: access, Title: title,
				Similarity: h.Similarity, Text: judgeEvidenceText(h),
			})
		}
		var j Judgment
		for _, cand := range out.Judgments {
			if cand.RequirementID == r.ID {
				j = cand
				break
			}
		}
		input, _ := json.Marshal(map[string]any{
			"requirement": r,
			"evidence":    ev,
		})
		outDoc := map[string]any{
			"verdict":       j.Verdict,
			"determination": j.Determination,
			"basis":         j.Basis,
			"uncertainty":   j.Uncertainty,
			"evidence_ids":  j.EvidenceIDs,
			"rationale":     j.Rationale,
		}
		if j.ReportedFacts != nil {
			outDoc["reported_facts"] = j.ReportedFacts
		}
		if j.Confidence != nil {
			outDoc["confidence"] = *j.Confidence
		}
		if len(j.RuleAdjustments) > 0 {
			outDoc["rule_adjustments"] = j.RuleAdjustments
		}
		if j.NotAssessed {
			outDoc["not_assessed"] = true
		}
		// Rationale issues ride on the decision row rather than getting
		// a surface of their own, because this row is already the thing
		// a person grades on /admin/decisions and already carries
		// human_verdict, human_note and reviewed_by. A separate list
		// would be a second place to look and a second thing to
		// remember, and the judgment being questioned is right here.
		//
		// They never change the verdict, so a reader grading the verdict
		// can disagree with the check as easily as with the model, which
		// is the point of having a person in the loop at all.
		if len(j.RationaleIssues) > 0 {
			outDoc["rationale_issues"] = j.RationaleIssues
		}
		if j.Adjusted != "" {
			outDoc["adjusted"] = j.Adjusted
		}
		if rj, ok := raw[r.ID]; ok {
			// raw_verdict stays the derived three-way reading of the
			// model's own determination, so the agreement history keeps
			// one meaning across the change (FR-HF-07).
			outDoc["raw_verdict"] = hiring.DeriveVerdict(rj.Determination)
			outDoc["raw_determination"] = rj.Determination
			outDoc["raw_basis"] = rj.Basis
			outDoc["raw_evidence_ids"] = rj.EvidenceIDs
		} else {
			outDoc["raw_verdict"] = ""
		}
		output, _ := json.Marshal(outDoc)
		judge := a.judgeOrDefault()
		d := users.Decision{
			Kind: "jd_requirement_verdict", RefKind: "jd_submission", RefID: submissionID, Key: r.ID,
			Model: out.Model, PromptID: judge.ID, PromptVersion: judge.Version,
			NumCtx: a.numCtx, Input: input, Output: output,
		}
		if j.NotAssessed {
			d.Model = "code"
		}
		if bi, ok := batchOf[r.ID]; ok && bi < len(calls) {
			c := calls[bi]
			d.PromptText = judge.System + "\n\n---\n\n" + userPrompts[bi]
			d.ResponseText = c.Text
			d.PromptTokens, d.CompletionTok, d.LatencyMs = c.PromptTokens, c.CompletionTokens, c.LatencyMs
		}
		rows = append(rows, d)
	}
	ids, err := a.users.InsertDecisionsReturning(context.WithoutCancel(ctx), rows)
	if err != nil {
		a.log.Warn("decision log write failed", slog.Int64("jd_id", submissionID), slog.String("error", err.Error()))
		return
	}
	// The judgment rows the hiring writer creates cite these (FR-HF-04).
	for i, d := range rows {
		if i < len(ids) && out.DecisionIDs != nil {
			out.DecisionIDs[d.Key] = ids[i]
		}
	}
}

func verdictValue(v string) float64 {
	switch v {
	case "met":
		return 1
	case "partial":
		return 0.5
	default:
		return 0
	}
}

// normaliseForQuote collapses whitespace and lowercases, so a quote
// the model re-wrapped still matches the posting it came from. Only
// whitespace and case are forgiven; a changed word is a changed quote.
func normaliseForQuote(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// quoteAppearsIn reports whether the quote is genuinely a span of the
// posting. A source quote exists to give the judge the posting's own
// words, so one the model composed rather than copied is worse than
// none at all: it looks like evidence and is not.
func quoteAppearsIn(quote, jd string) bool {
	q := normaliseForQuote(quote)
	if q == "" {
		return false
	}
	return strings.Contains(normaliseForQuote(jd), q)
}

func validateRequirements(in []prompts.Requirement, jdText string) ([]prompts.Requirement, error) {
	if len(in) == 0 {
		return nil, errors.New("no requirements extracted")
	}
	seen := map[string]bool{}
	var out []prompts.Requirement
	for i, r := range in {
		if i >= 20 {
			break
		}
		r.Text = strings.TrimSpace(r.Text)
		if r.Text == "" {
			continue
		}
		if r.ID == "" || seen[r.ID] {
			r.ID = fmt.Sprintf("r%d", i+1)
		}
		seen[r.ID] = true
		if r.Category != "must" && r.Category != "nice" {
			r.Category = "nice"
		}
		if r.Weight < 1 || r.Weight > 3 {
			r.Weight = 2
		}
		r.SourceQuote = strings.TrimSpace(r.SourceQuote)
		if r.SourceQuote != "" && !quoteAppearsIn(r.SourceQuote, jdText) {
			// Dropped rather than kept with a caveat. The judge cannot
			// tell a copied quote from a composed one, so anything that
			// reaches it must have been checked here.
			r.SourceQuote = ""
		}
		// A type outside the vocabulary is no type; the heuristic stands
		// in when the row is drafted. Alternatives are trimmed and
		// bounded, and one alone is the requirement restated, not a
		// choice.
		if !slices.Contains(hiring.RequirementTypes, r.Type) {
			r.Type = ""
		}
		r.Alternatives = cleanAlternatives(r.Alternatives)
		// Everything else on a requirement is the row's, written in code
		// after this. A provider that does not enforce the schema must
		// not be able to set a class, a threshold or an evidence
		// standard by adding a field to its answer.
		r.RowID, r.Version, r.Class, r.Thresholds = "", 0, "", nil
		r.EvidenceStandard, r.EquivalenceNotes, r.Equivalency = "", "", ""
		r.Included, r.Edited, r.StandardEdited = nil, false, false
		out = append(out, r)
	}
	if len(out) == 0 {
		return nil, errors.New("no usable requirements")
	}
	return out, nil
}

// cleanAlternatives trims the extractor's alternatives, drops blanks
// and repeats, and holds the list to the schema's bounds. Fewer than
// two is no choice, and returns nil.
func cleanAlternatives(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, a := range in {
		a = strings.TrimSpace(a)
		key := strings.ToLower(a)
		if a == "" || seen[key] || len(out) >= hiring.MaxAlternatives {
			continue
		}
		if len(a) > hiring.MaxAlternativeText {
			a = a[:hiring.MaxAlternativeText]
		}
		seen[key] = true
		out = append(out, a)
	}
	if len(out) < 2 {
		return nil
	}
	return out
}

// call runs one schema-constrained generation, records usage, and
// decodes the JSON into dst. Returns what the call produced.
func (a *Assessor) call(ctx context.Context, submissionID int64, p prompts.Prompt, user string, maxTokens int, dst any) (callResult, error) {
	started := time.Now()
	req := llm.Request{
		System:      p.System,
		User:        user,
		MaxTokens:   maxTokens,
		Temperature: 0,
		JSONSchema:  p.Schema,
		TraceID:     fmt.Sprintf("jd:%d:%s", submissionID, p.ID),
	}
	resp, err := a.llm.Generate(ctx, req)
	if err != nil && isTransient(err) {
		// One retry per call, not per assessment: a judge pass is a dozen
		// calls on a CPU box and a model-runner restart mid-way should
		// cost one call's worth of time, not the whole pass.
		a.log.Warn("llm call transient failure, retrying once",
			slog.Int64("jd_id", submissionID), slog.String("prompt", p.ID), slog.String("error", err.Error()))
		if sleepCtx(ctx, retryDelay) {
			resp, err = a.llm.Generate(ctx, req)
		}
	}
	usage := users.LLMUsage{
		Kind: "jd_assess", RefID: submissionID,
		PromptID: p.ID, PromptVersion: p.Version,
		LatencyMs: time.Since(started).Milliseconds(),
		Model:     "unknown",
	}
	if err == nil {
		usage.OK = true
		usage.Model = resp.Model
		usage.PromptTokens = resp.PromptTokens
		usage.CompletionTokens = resp.CompletionTokens
		if resp.LatencyMs > 0 {
			usage.LatencyMs = resp.LatencyMs
		}
	} else {
		usage.Error = truncErr(err.Error())
	}
	a.recordUsage(ctx, submissionID, usage)
	if err != nil {
		return callResult{}, err
	}
	res := callResult{
		Model: resp.Model, Text: resp.Text,
		PromptTokens: resp.PromptTokens, CompletionTokens: resp.CompletionTokens,
		LatencyMs: usage.LatencyMs,
	}
	if err := json.Unmarshal([]byte(resp.Text), dst); err != nil {
		// Truncation and malformed JSON fail the same way here, and
		// they need opposite fixes: one is a budget that is too small,
		// the other is a model that cannot follow the schema. The
		// completion hitting the cap distinguishes them, and saying so
		// turns "unexpected end of JSON input" into an instruction.
		// Evaluation run 7 lost two postings to this and the message
		// pointed at neither cause.
		truncated := resp.CompletionTokens >= int32(maxTokens) && maxTokens > 0
		if !truncated {
			a.recordFailure(ctx, submissionID, p, user, resp,
				fmt.Sprintf("decode failed under budget (%d of %d tokens): %v", resp.CompletionTokens, maxTokens, err))
			return res, fmt.Errorf("decode %s output: %w", p.ID, err)
		}

		// One retry at double the budget.
		//
		// Run 13 lost the Blue Origin posting to exactly this, four
		// hours into a five-hour evaluation, on the one requirement the
		// run existed to test. The judge averages 123 completion tokens
		// and one call in 1,343 has ever passed 500; this one filled
		// 1,200 because the corpus had finally been given something
		// substantial to say about that requirement. The evidence
		// arriving is what overflowed the budget.
		//
		// Retrying rather than only raising the ceiling, because a
		// ceiling is a guess about the longest answer the model will
		// ever want to give and it will be wrong again. A retry costs
		// one call when the guess is wrong instead of a whole run, and
		// the log line says it happened so the ceiling can be raised
		// deliberately rather than by accident.
		a.log.Warn("judge output hit its token budget, retrying once at double",
			slog.Int64("jd_id", submissionID), slog.String("prompt", p.ID),
			slog.Int("budget", maxTokens))

		wider := req
		wider.MaxTokens = maxTokens * 2
		retryStarted := time.Now()
		resp2, err2 := a.llm.Generate(ctx, wider)
		usage2 := users.LLMUsage{
			Kind: "jd_assess", RefID: submissionID,
			PromptID: p.ID, PromptVersion: p.Version,
			LatencyMs: time.Since(retryStarted).Milliseconds(),
			Model:     "unknown",
		}
		if err2 == nil {
			usage2.OK = true
			usage2.Model = resp2.Model
			usage2.PromptTokens = resp2.PromptTokens
			usage2.CompletionTokens = resp2.CompletionTokens
			if resp2.LatencyMs > 0 {
				usage2.LatencyMs = resp2.LatencyMs
			}
		} else {
			usage2.Error = truncErr(err2.Error())
		}
		a.recordUsage(ctx, submissionID, usage2)
		if err2 != nil {
			// The retry never produced a response, so the truncated
			// first one is the only output there is to keep.
			a.recordFailure(ctx, submissionID, p, user, resp,
				fmt.Sprintf("filled its %d-token budget; the retry at %d failed to generate: %v",
					maxTokens, wider.MaxTokens, err2))
			return res, fmt.Errorf(
				"decode %s output: the model filled its %d-token budget and the retry at %d failed: %w",
				p.ID, maxTokens, wider.MaxTokens, err2)
		}
		res = callResult{
			Model: resp2.Model, Text: resp2.Text,
			PromptTokens: resp2.PromptTokens, CompletionTokens: resp2.CompletionTokens,
			LatencyMs: usage2.LatencyMs,
		}
		if err3 := json.Unmarshal([]byte(resp2.Text), dst); err3 != nil {
			a.recordFailure(ctx, submissionID, p, user, resp2,
				fmt.Sprintf("retry at %d tokens did not decode either (%d used): %v",
					wider.MaxTokens, resp2.CompletionTokens, err3))
			return res, fmt.Errorf(
				"decode %s output: the model filled its %d-token budget, and the retry at %d did not decode either (%d tokens): %w",
				p.ID, maxTokens, wider.MaxTokens, resp2.CompletionTokens, err3)
		}
		a.log.Info("judge retry at double budget decoded",
			slog.Int64("jd_id", submissionID), slog.String("prompt", p.ID),
			slog.Int("completion_tokens", int(resp2.CompletionTokens)))
		return res, nil
	}
	return res, nil
}

// recordUsage writes the usage ledger row, and is never a reason a call
// fails. The repository is absent in tests that exercise the call path
// with a scripted model, and a missing ledger row is not worth a panic
// in production either: the row is evidence about a call, not part of
// making it.
func (a *Assessor) recordUsage(ctx context.Context, submissionID int64, u users.LLMUsage) {
	if a.users == nil {
		return
	}
	if err := a.users.RecordLLMUsage(context.WithoutCancel(ctx), u); err != nil {
		a.log.Warn("llm usage record failed",
			slog.Int64("jd_id", submissionID), slog.String("error", err.Error()))
	}
}

// recordFailure keeps the prompt and the whole response of a call that
// produced no usable verdict.
//
// decision_log carries prompt_text and response_text for every
// judgment, and was written only after a successful decode, so the one
// response nobody could examine was the one that broke. Run 13 lost a
// posting to a judgment that filled its budget, and the response that
// did it is gone: all that survives is a token count in llm_usage, from
// which the cause had to be inferred rather than read.
//
// Behaviour at and near a failure is the behaviour least understood and
// most worth keeping, so the whole response goes in, untruncated. These
// rows are rare by construction; one call in 1,343 has ever hit this.
//
// Best-effort, and deliberately so: a failure to record a failure must
// not replace the error the caller is about to return with a different
// one.
func (a *Assessor) recordFailure(ctx context.Context, submissionID int64, p prompts.Prompt, user string, resp *llm.Response, why string) {
	if a.users == nil || resp == nil {
		return
	}
	row := users.Decision{
		Kind: "jd_call_failed", RefKind: "jd_submission", RefID: submissionID,
		Model: resp.Model, PromptID: p.ID, PromptVersion: p.Version, NumCtx: a.numCtx,
		PromptText:   p.System + "\n\n---\n\n" + user,
		ResponseText: resp.Text,
		PromptTokens: resp.PromptTokens, CompletionTok: resp.CompletionTokens,
		LatencyMs: resp.LatencyMs,
		Error:     why,
	}
	if err := a.users.InsertDecisions(context.WithoutCancel(ctx), []users.Decision{row}); err != nil {
		a.log.Warn("could not record the failing call",
			slog.Int64("jd_id", submissionID), slog.String("prompt", p.ID),
			slog.String("error", err.Error()))
	}
}

func (a *Assessor) checkCap(ctx context.Context) error {
	// One requirements call plus at least one judgment call.
	return a.checkCapN(ctx, 2)
}

func (a *Assessor) checkCapN(ctx context.Context, calls int64) error {
	if a.monthlyCap <= 0 {
		return nil
	}
	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	n, err := a.users.CountLLMCallsSince(ctx, monthStart)
	if err != nil {
		return err
	}
	if n+calls > a.monthlyCap {
		return ErrMonthlyCap
	}
	return nil
}
