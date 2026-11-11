package jd

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/reh3376/career-site/services/api/internal/hiring"
	"github.com/reh3376/career-site/services/api/internal/llm"
	"github.com/reh3376/career-site/services/api/internal/prompts"
)

const extractPosting = `Senior Controls Engineer.
Requirements: Bachelor's degree in Electrical Engineering or equivalent experience.
8+ years of PLC programming with Rockwell platforms.
Lead the front-end development of future major investments, including scope definition.
A collaborative team player.`

// Extractor v5 (FR-HF-02): the type and the alternatives come back
// with each requirement, validated, and nothing the model wrote can
// set what the row is drafted with in code.
func TestExtractKeepsTypeAndAlternatives(t *testing.T) {
	answer := `{"requirements":[
	 {"id":"r1","text":"Bachelor's degree in Electrical Engineering or equivalent experience","source_quote":"Bachelor's degree in Electrical Engineering or equivalent experience","category":"must","weight":2,"named_parties":[],"type":"credential","alternatives":["a Bachelor's degree in Electrical Engineering"," equivalent experience ","Equivalent experience",""],
	  "class":"ideal","included":false,"evidence_standard":"anything goes","thresholds":{"min_years":1},"row_id":"x","version":7,"edited":true},
	 {"id":"r2","text":"8+ years of PLC programming with Rockwell platforms","source_quote":"8+ years of PLC programming with Rockwell platforms","category":"must","weight":3,"named_parties":[],"type":"wizardry","alternatives":["PLC programming"]},
	 {"id":"r3","text":"Lead front-end development of major investments","source_quote":"a quote the posting does not contain","category":"must","weight":3,"named_parties":[],"type":"outcome","alternatives":[]}
	]}`
	s := &scriptedLLM{responses: []*llm.Response{{Text: answer, CompletionTokens: 300, Model: "test-model"}}}
	a := quietAssessor(s)
	a.SetVocab(hiring.DefaultVocab())

	reqs, model, err := a.Extract(context.Background(), 1, extractPosting, prompts.Hints{}, nil)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if model != "test-model" || len(reqs) != 3 {
		t.Fatalf("model=%q, %d requirements", model, len(reqs))
	}
	// The call that was made is the extractor as rendered, at v5's
	// budget.
	if got := s.requests[0]; got.System != prompts.JDRequirements.System || got.JSONSchema != prompts.JDRequirements.Schema || got.MaxTokens != requirementsTokenBudget {
		t.Errorf("the extraction call did not use the rendered extractor at its budget (max tokens %d)", got.MaxTokens)
	}

	r1 := reqs[0]
	if r1.Type != "credential" || len(r1.Alternatives) != 2 || r1.Alternatives[1] != "equivalent experience" {
		t.Errorf("r1 type=%q alternatives=%q; want credential and two trimmed, distinct options", r1.Type, r1.Alternatives)
	}
	if r1.Class != "" || r1.Included != nil || r1.EvidenceStandard != "" || r1.Thresholds != nil || r1.RowID != "" || r1.Version != 0 || r1.Edited {
		t.Errorf("the model set fields that belong to the row: %+v", r1)
	}
	if reqs[1].Type != "" || reqs[1].Alternatives != nil {
		t.Errorf("r2: an unknown type must be dropped and one option is no choice: type=%q alternatives=%q", reqs[1].Type, reqs[1].Alternatives)
	}
	if reqs[2].SourceQuote != "" {
		t.Errorf("r3: an invented quote was kept: %q", reqs[2].SourceQuote)
	}

	// Drafted in code from that: the row the judge is shown.
	drafted := DraftRequirements(reqs, nil, hiring.Provenance{})
	d1, d2, d3 := drafted[0], drafted[1], drafted[2]
	if d1.Class != hiring.ClassRequired || d1.Thresholds == nil || d1.Thresholds.CredentialLevel != "bachelor" || !d1.Counts() {
		t.Errorf("r1 drafted: class=%s thresholds=%+v counts=%v; a credential with a posting alternative counts", d1.Class, d1.Thresholds, d1.Counts())
	}
	if !strings.Contains(d1.EvidenceStandard, "a bachelor's degree in Electrical Engineering") {
		t.Errorf("r1 standard = %q", d1.EvidenceStandard)
	}
	if d2.Type != "experience" || d2.Thresholds.MinYears == nil || *d2.Thresholds.MinYears != 8 || d2.Counts() {
		t.Errorf("r2 drafted: type=%s thresholds=%+v counts=%v; the heuristic types it, and a bare years item does not count", d2.Type, d2.Thresholds, d2.Counts())
	}
	if d3.Type != "outcome" || !d3.Counts() || !d3.Required() {
		t.Errorf("r3 drafted: %+v", d3)
	}
	// The drafts the writer would store are the same requirements.
	rows := Drafts(reqs, nil, hiring.Provenance{})
	for i, q := range rows {
		back := FromRow(q)
		if back.Text != drafted[i].Text || back.Type != drafted[i].Type || back.EvidenceStandard != drafted[i].EvidenceStandard {
			t.Errorf("%s: the stored draft and the judged draft differ", q.LocalID)
		}
	}
}

// What settles a judgment reads the row's thresholds: the hiring
// manager's number, not the text's.
func TestSettleReadsTheRowsThresholds(t *testing.T) {
	years := func(n float64) *hiring.Thresholds { return &hiring.Thresholds{MinYears: &n} }
	raw := rawJudgment{
		RequirementID: "r1", Determination: hiring.DetMet, Basis: hiring.BasisDirect, Uncertainty: []string{},
		EvidenceIDs:   []string{"7"},
		ReportedFacts: hiring.ReportedFacts{StatedSpanYears: 6, AlternativeSatisfied: -1, InvolvementLevel: "owned"},
		Rationale:     "Six years of PLC programming are stated.",
	}
	in := settleInput{
		req:     prompts.Requirement{ID: "r1", Text: "10+ years of PLC programming", Category: "must", Weight: 3, Type: "experience"},
		raw:     raw,
		offered: map[int64]bool{7: true},
		now:     time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC),
		vocab:   hiring.DefaultVocab(),
	}
	// No row: the text's ten years decide, and six is well short.
	if j := settleJudgment(in); j.Determination != hiring.DetNotMet || len(j.RuleAdjustments) != 1 {
		t.Errorf("text threshold: %s with %d adjustments", j.Determination, len(j.RuleAdjustments))
	}
	// The row says six.
	in.req.Thresholds = years(6)
	if j := settleJudgment(in); j.Determination != hiring.DetMet || len(j.RuleAdjustments) != 0 || j.Verdict != "met" {
		t.Errorf("row threshold of 6: %s with %d adjustments", j.Determination, len(j.RuleAdjustments))
	}
	// The row says seven: six is within a quarter, so it is a question.
	in.req.Thresholds = years(7)
	if j := settleJudgment(in); j.Determination != hiring.DetUnresolved || j.Basis != hiring.BasisDurationShortfall || j.RuleAdjustments[0].RuleVersion != hiring.RuleDurationVersion {
		t.Errorf("row threshold of 7: %s / %s", j.Determination, j.Basis)
	}
}
