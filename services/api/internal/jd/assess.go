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

// ErrMonthlyCap is returned when the LLM call cap for the current
// calendar month has been reached (Phase 4 guardrail #8).
var ErrMonthlyCap = errors.New("llm monthly call cap reached")

// rawJudgment is the model's verdict as decoded, before validation.
type rawJudgment struct {
	RequirementID string   `json:"requirement_id"`
	Verdict       string   `json:"verdict"`
	EvidenceIDs   []string `json:"evidence_ids"`
	Rationale     string   `json:"rationale"`
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
	Model       string             `json:"model"`
	Prompts     map[string]int     `json:"prompts"` // prompt id → version
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
func (a *Assessor) batchRequirements(reqs []prompts.Requirement, evidence map[string][]users.CorpusHit) [][]prompts.Requirement {
	budget := a.numCtx - judgeReserve - prompts.EstimateTokens(prompts.RequirementJudge.System)
	var batches [][]prompts.Requirement
	var cur []prompts.Requirement
	for _, r := range reqs {
		trial := append(append([]prompts.Requirement{}, cur...), r)
		if len(cur) >= judgeBatchMax || (len(cur) > 0 && prompts.EstimateTokens(prompts.RenderJudgeUser(trial, evidence)) > budget) {
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

// Assess produces the assessment for one JD. Two LLM calls, N
// retrievals. Every call lands in llm_usage.
func (a *Assessor) Assess(ctx context.Context, submissionID int64, jdText string, hints prompts.Hints) (*Assessment, error) {
	if err := a.checkCap(ctx); err != nil {
		return nil, err
	}
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
	reqCall, err := a.call(ctx, submissionID, prompts.JDRequirements,
		prompts.RenderRequirementsUser(jdText, hints), 1200, &reqDoc)
	if err != nil {
		return nil, fmt.Errorf("requirements: %w", err)
	}
	out.Model = reqCall.Model
	reqs, err := validateRequirements(reqDoc.Requirements)
	if err != nil {
		return nil, fmt.Errorf("requirements: %w", err)
	}
	out.Requirements = reqs

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
	evidence := make(map[string][]users.CorpusHit, len(reqs))
	for i, r := range reqs {
		hits, err := a.users.SearchCorpus(ctx, vectors[i], EvidencePerRequirement)
		if err != nil {
			return nil, fmt.Errorf("retrieve %s: %w", r.ID, err)
		}
		evidence[r.ID] = hits
		for _, h := range hits {
			out.EvidenceIDs[r.ID] = append(out.EvidenceIDs[r.ID], h.Chunk.ID)
		}
	}

	// 3. Judgments, in batches sized to the context window.
	var all []rawJudgment
	batches := a.batchRequirements(reqs, evidence)
	if err := a.checkCapN(ctx, int64(len(batches))); err != nil {
		return nil, err
	}
	// Per requirement: which call produced its verdict, so the decision
	// log can carry the exact prompt and raw response for each one.
	calls := make([]callResult, 0, len(batches))
	userPrompts := make([]string, 0, len(batches))
	batchOf := map[string]int{}
	for bi, batch := range batches {
		var judgeDoc struct {
			Judgments []rawJudgment `json:"judgments"`
		}
		user := prompts.RenderJudgeUser(batch, evidence)
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
	rawByReq := map[string]rawJudgment{}
	byReq := map[string]Judgment{}
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
		byReq[j.RequirementID] = Judgment{
			RequirementID: j.RequirementID,
			Verdict:       verdict,
			EvidenceIDs:   ids,
			Rationale:     truncErr(strings.TrimSpace(j.Rationale)),
		}
	}

	// 4. Score in code. A requirement the model skipped counts as unmet.
	var weighted float64
	for _, r := range reqs {
		j, ok := byReq[r.ID]
		if !ok {
			j = Judgment{RequirementID: r.ID, Verdict: "unmet", Rationale: "no judgment returned"}
		}
		out.Judgments = append(out.Judgments, j)
		out.WeightTotal += r.Weight
		weighted += float64(r.Weight) * verdictValue(j.Verdict)
	}
	if out.WeightTotal > 0 {
		out.Score = weighted / float64(out.WeightTotal)
	}

	// 5. Decision log: one row per verdict with everything the owner
	// needs to judge it himself (docs/decision-log.md). Best-effort.
	a.logVerdicts(ctx, submissionID, out, evidence, rawByReq, calls, userPrompts, batchOf)
	return out, nil
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
				Similarity: h.Similarity, Text: prompts.CapRunes(h.Chunk.Text, prompts.JudgeChunkRunes),
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

func validateRequirements(in []prompts.Requirement) ([]prompts.Requirement, error) {
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
	resp, err := a.llm.Generate(ctx, llm.Request{
		System:      p.System,
		User:        user,
		MaxTokens:   maxTokens,
		Temperature: 0,
		JSONSchema:  p.Schema,
		TraceID:     fmt.Sprintf("jd:%d:%s", submissionID, p.ID),
	})
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
