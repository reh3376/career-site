package jd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/reh3376/career-site/services/api/internal/llm"
	"github.com/reh3376/career-site/services/api/internal/prompts"
	"github.com/reh3376/career-site/services/api/internal/users"
)

func quietResumeWriter(l llm.Client) *ResumeWriter {
	return &ResumeWriter{llm: l, log: slog.New(slog.NewTextHandler(io.Discard, nil)), numCtx: 16384}
}

func resumeEvidence(n int) []users.CorpusHit {
	out := make([]users.CorpusHit, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, users.CorpusHit{Chunk: users.CorpusChunk{ID: int64(i), Text: strings.Repeat("evidence ", 400)}})
	}
	return out
}

const goodResume = `{"headline":"Controls engineer","summary":"Thirty years.","competencies":[{"text":"PLC programming","sources":["1"]}],"experience":[],"education":[]}`

// Two reviews on the development machine ended "failed" on 2026-11-10
// after their requirements had been judged and scored, because the
// résumé filled its 2,200-token budget and stopped mid-string. The
// writer now asks once more at double the budget, with the evidence
// trimmed from the tail so the wider answer still fits the window.
func TestTruncatedResumeRetriesAtDoubleBudget(t *testing.T) {
	s := &scriptedLLM{responses: []*llm.Response{
		{Text: `{"headline":"Controls engineer","summary":"Thirty ye`, CompletionTokens: resumeTokenBudget, Model: "test"},
		{Text: goodResume, CompletionTokens: 2600, Model: "test"},
	}}
	w := quietResumeWriter(s)
	evidence := resumeEvidence(10)
	render := func(ev []users.CorpusHit) string {
		var b strings.Builder
		for _, h := range ev {
			fmt.Fprintf(&b, "<chunk id=\"%d\">%s</chunk>\n", h.Chunk.ID, h.Chunk.Text)
		}
		return b.String()
	}
	// A budget the full evidence just fits, so the retry has to make
	// room for the 2,200 extra tokens it may now write.
	budget := prompts.EstimateTokens(render(evidence)) + 50

	doc, resp, kept, err := w.generate(context.Background(), 1, prompts.ResumeTailor, render, evidence, budget)
	if err != nil {
		t.Fatalf("a truncated résumé should have been recovered by the retry: %v", err)
	}
	if len(s.requests) != 2 || s.requests[0].MaxTokens != resumeTokenBudget || s.requests[1].MaxTokens != 2*resumeTokenBudget {
		t.Fatalf("calls and budgets: %d calls, first %d, second %d", len(s.requests), s.requests[0].MaxTokens, s.requests[len(s.requests)-1].MaxTokens)
	}
	if doc.Headline != "Controls engineer" || len(doc.Competencies) != 1 || resp.CompletionTokens != 2600 {
		t.Errorf("the retry's answer was not the one decoded: %+v", doc)
	}
	if len(kept) >= len(evidence) || len(kept) < 1 {
		t.Errorf("evidence kept = %d of %d; the wider answer needs room, and the head of the list must survive", len(kept), len(evidence))
	}
	if kept[0].Chunk.ID != 1 {
		t.Errorf("the highest-priority evidence was dropped: first kept id %d", kept[0].Chunk.ID)
	}
	if got := prompts.EstimateTokens(s.requests[1].User); got > budget-resumeTokenBudget {
		t.Errorf("the retry's prompt is %d tokens, over the %d the wider answer leaves", got, budget-resumeTokenBudget)
	}
	if strings.Contains(s.requests[1].User, fmt.Sprintf("<chunk id=\"%d\">", len(evidence))) {
		t.Error("the retry's prompt still carries the evidence that was trimmed")
	}
}

// An answer that fits the budget is decoded as it is, in one call.
func TestResumeThatFitsIsOneCall(t *testing.T) {
	s := &scriptedLLM{responses: []*llm.Response{{Text: goodResume, CompletionTokens: 900, Model: "test"}}}
	evidence := resumeEvidence(3)
	doc, _, kept, err := quietResumeWriter(s).generate(context.Background(), 1, prompts.ResumeTailor,
		func([]users.CorpusHit) string { return "user" }, evidence, 8000)
	if err != nil || len(s.requests) != 1 || len(kept) != 3 || doc.Summary != "Thirty years." {
		t.Errorf("one call expected: err=%v calls=%d kept=%d doc=%+v", err, len(s.requests), len(kept), doc)
	}
}

// Malformed output under budget is a model that cannot follow the
// schema; a wider budget would not help, so there is no second call.
func TestMalformedResumeUnderBudgetIsNotRetried(t *testing.T) {
	s := &scriptedLLM{responses: []*llm.Response{{Text: `{"headline": oops`, CompletionTokens: 40, Model: "test"}}}
	_, _, _, err := quietResumeWriter(s).generate(context.Background(), 1, prompts.ResumeTailor,
		func([]users.CorpusHit) string { return "user" }, resumeEvidence(2), 8000)
	if err == nil || !strings.Contains(err.Error(), "decode resume") || len(s.requests) != 1 {
		t.Errorf("err=%v calls=%d; want a decode error and one call", err, len(s.requests))
	}
}

// When the retry does not decode either, the error says both things
// happened, so the cap can be raised deliberately rather than guessed.
func TestResumeRetryThatAlsoFailsSaysSo(t *testing.T) {
	s := &scriptedLLM{responses: []*llm.Response{
		{Text: `{"headline":"x`, CompletionTokens: resumeTokenBudget, Model: "test"},
		{Text: `{"headline":"x","summary":"y`, CompletionTokens: 2 * resumeTokenBudget, Model: "test"},
	}}
	_, _, _, err := quietResumeWriter(s).generate(context.Background(), 1, prompts.ResumeTailor,
		func([]users.CorpusHit) string { return "user" }, resumeEvidence(2), 8000)
	if err == nil || !strings.Contains(err.Error(), "retry at 4400 did not decode either") || len(s.requests) != 2 {
		t.Errorf("err=%v calls=%d", err, len(s.requests))
	}
}
