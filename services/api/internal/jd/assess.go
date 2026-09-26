package jd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

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

// rawJudgment is the model's verdict as decoded, before validation.
type rawJudgment struct {
	RequirementID string   `json:"requirement_id"`
	Verdict       string   `json:"verdict"`
	EvidenceIDs   []string `json:"evidence_ids"`
	Rationale     string   `json:"rationale"`
	// StatedSpanYears is what the evidence says about duration, reported
	// by the model rather than judged by it. The comparison against what
	// the requirement asks for happens in duration.go.
	StatedSpanYears float64 `json:"stated_span_years"`
	// PartiesEvidenced are the organisations the evidence names as ones
	// the candidate worked with. Compared against the requirement's own
	// named parties in relationship.go.
	PartiesEvidenced []string `json:"parties_evidenced"`
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

// Judgment is the model's verdict on one requirement, validated.
type Judgment struct {
	RequirementID string  `json:"requirement_id"`
	Verdict       string  `json:"verdict"` // met | partial | unmet
	EvidenceIDs   []int64 `json:"evidence_ids"`
	Rationale     string  `json:"rationale"`
	// StatedSpanYears is the duration the model found in the evidence,
	// kept whether or not it mattered, so a reader can see what the
	// comparison was made against.
	StatedSpanYears float64 `json:"stated_span_years,omitempty"`
	// PartiesEvidenced are the organisations the evidence named, kept so
	// a reader can see what the comparison was made against.
	PartiesEvidenced []string `json:"parties_evidenced,omitempty"`
	// Adjusted explains a verdict that code weakened, and is empty when
	// the model's verdict stood. An adjustment that cannot be read is
	// indistinguishable from the model having said so itself.
	Adjusted string `json:"adjusted,omitempty"`
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
	// Error is set when the pipeline could not complete and the caller
	// fell back to the retrieval score.
	Error string `json:"error,omitempty"`
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
	return &Assessor{log: log, users: repo, embed: embed, llm: client, monthlyCap: monthlyCap, numCtx: numCtx}
}

// batchRequirements groups requirements so each rendered judgment
// prompt stays under the context budget. A single oversized
// requirement still gets its own batch (the renderer caps chunk text).
func (a *Assessor) batchRequirements(reqs []prompts.Requirement, evidence map[string][]users.CorpusHit, profile []users.CorpusHit) [][]prompts.Requirement {
	budget := a.numCtx - judgeReserve - prompts.EstimateTokens(prompts.RequirementJudge.System)
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

// Assess produces the assessment for one JD: one requirements call,
// one embed call, one judge call per requirement. Every call lands in
// llm_usage. Progress is reported from 5% to 75%; the scorer owns the
// rest (résumé, PDF).
func (a *Assessor) Assess(ctx context.Context, submissionID int64, jdText string, hints prompts.Hints, progress Progress) (*Assessment, error) {
	if err := a.checkCap(ctx); err != nil {
		return nil, err
	}
	progress.report(5, "reading the posting")
	out := &Assessment{
		EvidenceIDs: map[string][]int64{},
		Prompts: map[string]int{
			prompts.JDRequirements.ID:   prompts.JDRequirements.Version,
			prompts.RequirementJudge.ID: prompts.RequirementJudge.Version,
		},
	}

	// 1. Requirements.
	var reqDoc struct {
		Requirements []prompts.Requirement `json:"requirements"`
	}
	// The budget is arithmetic, not a guess. The schema allows 20
	// requirements, each carrying text up to 200 characters and a
	// source_quote up to 300, plus id, category, weight and
	// named_parties: about 560 characters, call it 160 tokens. Twenty
	// of those is 3,200, and the JSON wrapper rounds it up.
	//
	// It was 1,200, which was ample until source_quote was added on
	// 2026-09-25 and roughly tripled the output. Evaluation run 7 then
	// lost two of eight postings to "unexpected end of JSON input",
	// marginally rather than universally: a 4,682-character posting fit
	// and a 4,903-character one did not. A cap costs nothing when it is
	// not reached, so it is set where a full-length answer fits rather
	// than where a typical one does.
	const requirementsTokenBudget = 4000
	reqCall, err := a.call(ctx, submissionID, prompts.JDRequirements,
		prompts.RenderRequirementsUser(jdText, hints), requirementsTokenBudget, &reqDoc)
	if err != nil {
		return nil, fmt.Errorf("requirements: %w", err)
	}
	out.Model = reqCall.Model
	reqs, err := validateRequirements(reqDoc.Requirements, jdText)
	if err != nil {
		return nil, fmt.Errorf("requirements: %w", err)
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
	var all []rawJudgment
	batches := a.batchRequirements(reqs, evidence, profile)
	if err := a.checkCapN(ctx, int64(len(batches))); err != nil {
		return nil, err
	}
	// Per requirement: which call produced its verdict, so the decision
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
		res, err := a.call(ctx, submissionID, prompts.RequirementJudge, user, 1200, &judgeDoc)
		if err != nil {
			return nil, fmt.Errorf("judge batch %d/%d: %w", bi+1, len(batches), err)
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
	// The requirement's own text is what says how many years it asks
	// for, so the duration rule needs it alongside the verdict.
	reqByID := make(map[string]prompts.Requirement, len(reqs))
	for _, r := range reqs {
		reqByID[r.ID] = r
	}
	for _, j := range all {
		if _, dup := rawByReq[j.RequirementID]; !dup {
			rawByReq[j.RequirementID] = j
		}
		if _, dup := byReq[j.RequirementID]; dup {
			continue
		}
		allowed := map[int64]bool{}
		for _, id := range out.EvidenceIDs[j.RequirementID] {
			allowed[id] = true
		}
		var ids []int64
		for _, s := range j.EvidenceIDs {
			id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
			if err == nil && allowed[id] {
				ids = append(ids, id)
			}
		}
		verdict := j.Verdict
		// A verdict that cites nothing it was given is downgraded: the
		// score must be traceable to evidence, not to the model's mood.
		if verdict != "unmet" && len(ids) == 0 {
			verdict = "unmet"
		}
		if verdict == "unmet" {
			ids = nil
		}
		// Duration is decided here, not by the judge. Three prompt
		// versions failed to teach a small model to check a span before
		// answering "met"; it reports the span it found and this applies
		// the comparison. See duration.go.
		adjusted := ""
		if req, ok := reqByID[j.RequirementID]; ok {
			verdict, adjusted = applyDurationRule(req.Text, verdict, j.StatedSpanYears)
			// A requirement can ask for both a span and a named party.
			// The duration rule runs first and may already have weakened
			// the verdict, in which case this one has nothing to do.
			if v, note := applyRelationshipRule(req, verdict, j.PartiesEvidenced); note != "" {
				verdict = v
				if adjusted == "" {
					adjusted = note
				} else {
					adjusted += "; " + note
				}
			}
		}
		byReq[j.RequirementID] = Judgment{
			RequirementID:    j.RequirementID,
			Verdict:          verdict,
			EvidenceIDs:      ids,
			Rationale:        truncErr(strings.TrimSpace(j.Rationale)),
			StatedSpanYears:  j.StatedSpanYears,
			PartiesEvidenced: j.PartiesEvidenced,
			Adjusted:         adjusted,
		}
	}

	// 4. Score in code. A requirement the model skipped counts as unmet.
	//
	// Formula (v2, 2026-09-22): every "must" requirement is in the
	// denominator; a "nice" requirement joins it only when it earned
	// something. Preferred items can raise a score, never sink it: an
	// ATS that subtracts for a missing PMP or a preferred Master's is
	// exactly the filter this reviewer is meant not to be. Verdict
	// values are unchanged (met 1, partial 0.5, unmet 0).
	// Floor: a posting with no "must" lines would otherwise score 1.0
	// from a single evidenced preference; then every requirement counts.
	anyMust := false
	for _, r := range reqs {
		if r.Category == "must" {
			anyMust = true
			break
		}
	}
	var weighted float64
	for _, r := range reqs {
		j, ok := byReq[r.ID]
		if !ok {
			j = Judgment{RequirementID: r.ID, Verdict: "unmet", Rationale: "no judgment returned"}
		}
		out.Judgments = append(out.Judgments, j)
		v := verdictValue(j.Verdict)
		if !anyMust || r.Category == "must" || v > 0 {
			out.WeightTotal += r.Weight
		}
		weighted += float64(r.Weight) * v
	}
	if out.WeightTotal > 0 {
		out.Score = weighted / float64(out.WeightTotal)
	}
	out.ScoreFormula = ScoreFormula

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
			"verdict":      j.Verdict,
			"evidence_ids": j.EvidenceIDs,
			"rationale":    j.Rationale,
		}
		if rj, ok := raw[r.ID]; ok {
			outDoc["raw_verdict"] = rj.Verdict
			outDoc["raw_evidence_ids"] = rj.EvidenceIDs
		} else {
			outDoc["raw_verdict"] = ""
		}
		output, _ := json.Marshal(outDoc)
		d := users.Decision{
			Kind: "jd_requirement_verdict", RefKind: "jd_submission", RefID: submissionID, Key: r.ID,
			Model: out.Model, PromptID: prompts.RequirementJudge.ID, PromptVersion: prompts.RequirementJudge.Version,
			NumCtx: a.numCtx, Input: input, Output: output,
		}
		if bi, ok := batchOf[r.ID]; ok && bi < len(calls) {
			c := calls[bi]
			d.PromptText = prompts.RequirementJudge.System + "\n\n---\n\n" + userPrompts[bi]
			d.ResponseText = c.Text
			d.PromptTokens, d.CompletionTok, d.LatencyMs = c.PromptTokens, c.CompletionTokens, c.LatencyMs
		}
		rows = append(rows, d)
	}
	if err := a.users.InsertDecisions(context.WithoutCancel(ctx), rows); err != nil {
		a.log.Warn("decision log write failed", slog.Int64("jd_id", submissionID), slog.String("error", err.Error()))
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
		out = append(out, r)
	}
	if len(out) == 0 {
		return nil, errors.New("no usable requirements")
	}
	return out, nil
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
	if rErr := a.users.RecordLLMUsage(context.WithoutCancel(ctx), usage); rErr != nil {
		a.log.Warn("llm usage record failed", slog.Int64("jd_id", submissionID), slog.String("error", rErr.Error()))
	}
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
		if resp.CompletionTokens >= int32(maxTokens) && maxTokens > 0 {
			return res, fmt.Errorf(
				"decode %s output: the model filled its %d-token budget and the JSON is incomplete, so raise the budget for this call: %w",
				p.ID, maxTokens, err)
		}
		return res, fmt.Errorf("decode %s output: %w", p.ID, err)
	}
	return res, nil
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
