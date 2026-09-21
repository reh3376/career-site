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
// is judged against.
const EvidencePerRequirement = 4

// ErrMonthlyCap is returned when the LLM call cap for the current
// calendar month has been reached (Phase 4 guardrail #8).
var ErrMonthlyCap = errors.New("llm monthly call cap reached")

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
}

// NewAssessor wires the deps.
func NewAssessor(log *slog.Logger, repo *users.Repo, embed ingest.EmbedClient, client llm.Client, monthlyCap int64) *Assessor {
	return &Assessor{log: log, users: repo, embed: embed, llm: client, monthlyCap: monthlyCap}
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
	model, err := a.call(ctx, submissionID, prompts.JDRequirements,
		prompts.RenderRequirementsUser(jdText, hints), 1200, &reqDoc)
	if err != nil {
		return nil, fmt.Errorf("requirements: %w", err)
	}
	out.Model = model
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

	// 3. Judgments.
	var judgeDoc struct {
		Judgments []struct {
			RequirementID string   `json:"requirement_id"`
			Verdict       string   `json:"verdict"`
			EvidenceIDs   []string `json:"evidence_ids"`
			Rationale     string   `json:"rationale"`
		} `json:"judgments"`
	}
	if _, err := a.call(ctx, submissionID, prompts.RequirementJudge,
		prompts.RenderJudgeUser(reqs, evidence), 2000, &judgeDoc); err != nil {
		return nil, fmt.Errorf("judge: %w", err)
	}
	byReq := map[string]Judgment{}
	for _, j := range judgeDoc.Judgments {
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
	return out, nil
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
// decodes the JSON into dst. Returns the model name.
func (a *Assessor) call(ctx context.Context, submissionID int64, p prompts.Prompt, user string, maxTokens int, dst any) (string, error) {
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
		return "", err
	}
	if err := json.Unmarshal([]byte(resp.Text), dst); err != nil {
		return resp.Model, fmt.Errorf("decode %s output: %w", p.ID, err)
	}
	return resp.Model, nil
}

func (a *Assessor) checkCap(ctx context.Context) error {
	if a.monthlyCap <= 0 {
		return nil
	}
	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	n, err := a.users.CountLLMCallsSince(ctx, monthStart)
	if err != nil {
		return err
	}
	// Two calls per assessment.
	if n+2 > a.monthlyCap {
		return ErrMonthlyCap
	}
	return nil
}
