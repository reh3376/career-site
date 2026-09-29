package jd

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/reh3376/career-site/services/api/internal/llm"
	"github.com/reh3376/career-site/services/api/internal/prompts"
)

// scriptedLLM returns a queued response per call, so a test can say
// "truncate first, succeed second".
type scriptedLLM struct {
	responses []*llm.Response
	requests  []llm.Request
}

func (s *scriptedLLM) Generate(_ context.Context, req llm.Request) (*llm.Response, error) {
	s.requests = append(s.requests, req)
	r := s.responses[0]
	s.responses = s.responses[1:]
	return r, nil
}

func quietAssessor(l llm.Client) *Assessor {
	return &Assessor{
		llm:   l,
		log:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		users: nil,
	}
}

// Run 13 lost a posting four hours in because one judgment filled its
// 1200-token budget and the incomplete JSON aborted the assessment. The
// requirement it happened on was the one the run existed to test.
func TestTruncatedJudgeRetriesAtDoubleBudget(t *testing.T) {
	good := `{"judgments":[{"requirement_id":"r11","verdict":"met","evidence_ids":["1"],"rationale":"ok","stated_span_years":3,"parties_evidenced":[]}]}`
	s := &scriptedLLM{responses: []*llm.Response{
		// First: filled the budget, JSON cut off mid-string.
		{Text: `{"judgments":[{"requirement_id":"r11","verdict":"me`, CompletionTokens: 1200, Model: "test"},
		// Second: fits, decodes.
		{Text: good, CompletionTokens: 140, Model: "test"},
	}}
	a := quietAssessor(s)

	var dst struct {
		Judgments []struct {
			RequirementID string `json:"requirement_id"`
			Verdict       string `json:"verdict"`
		} `json:"judgments"`
	}
	_, err := a.call(context.Background(), 1, prompts.RequirementJudge, "user", 1200, &dst)
	if err != nil {
		t.Fatalf("a truncated judgment should have been recovered by the retry: %v", err)
	}
	if len(s.requests) != 2 {
		t.Fatalf("expected one call and one retry, got %d calls", len(s.requests))
	}
	if s.requests[1].MaxTokens != 2400 {
		t.Errorf("retry budget = %d, want double the original 1200", s.requests[1].MaxTokens)
	}
	if len(dst.Judgments) != 1 || dst.Judgments[0].Verdict != "met" {
		t.Errorf("the retry's output was not decoded into dst: %+v", dst.Judgments)
	}
}

// A model that cannot follow the schema is a different fault from a
// budget that is too small, and retrying it twice as expensively helps
// nobody. It must fail on the first call.
func TestMalformedJsonUnderBudgetDoesNotRetry(t *testing.T) {
	s := &scriptedLLM{responses: []*llm.Response{
		{Text: `not json at all`, CompletionTokens: 40, Model: "test"},
	}}
	a := quietAssessor(s)
	var dst map[string]any
	if _, err := a.call(context.Background(), 1, prompts.RequirementJudge, "user", 1200, &dst); err == nil {
		t.Fatal("malformed JSON well under budget should fail, not retry")
	}
	if len(s.requests) != 1 {
		t.Errorf("expected no retry, got %d calls", len(s.requests))
	}
}

// The retry is one attempt, not a loop. A model that fills twice the
// budget is doing something the budget cannot fix.
func TestRetryIsNotALoop(t *testing.T) {
	cut := `{"judgments":[{"requirement_id":"r11","verdict":"me`
	s := &scriptedLLM{responses: []*llm.Response{
		{Text: cut, CompletionTokens: 1200, Model: "test"},
		{Text: cut, CompletionTokens: 2400, Model: "test"},
	}}
	a := quietAssessor(s)
	var dst map[string]any
	if _, err := a.call(context.Background(), 1, prompts.RequirementJudge, "user", 1200, &dst); err == nil {
		t.Fatal("a second truncation should fail rather than retry again")
	}
	if len(s.requests) != 2 {
		t.Errorf("expected exactly two calls, got %d", len(s.requests))
	}
}
