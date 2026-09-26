package jd

import (
	"strings"
	"testing"
)

// Both cases below are verbatim from evaluation run 9 on 2026-09-26,
// which passed 9 of 9 with zero inversions while carrying them.

const (
	// Blue Origin r10. The model answered "met", reported
	// stated_span_years 0, and wrote a rationale describing thirty
	// years. applyDurationRule then downgraded it for want of a span
	// the model had just described.
	r10Requirement = "10+ Years of Software and systems engineering in fast paced environments"
	r10Rationale   = "The candidate has over 30 years of engineering experience, including " +
		"leadership roles in engineering teams at multiple companies, and has led development " +
		"teams in fast-paced environments."

	// Blue Origin r9. The quoted phrase is real, copied out of the
	// candidate's own facts sheet, but the requirement asks for a
	// Master's and the sentence calls the quote the requirement.
	r9Requirement = "Masters or equivalent experience in Computer Science, Physics, Statistics or other STEM degree."
	r9Rationale   = "The candidate holds a bachelor's degree in Applied Mathematics and Electrical " +
		"Engineering Technology, which satisfies the 'bachelor's degree in engineering or a related " +
		"field' requirement. Additionally, the evidence shows extensive experience in controls, " +
		"manufacturing systems, and applied AI, which supports the 'or equivalent experience' " +
		"alternative specified in the requirement."
	r9FactsSheet = "Degree requirements: holds a bachelor's degree (Applied Mathematics) plus an " +
		"engineering degree (Electrical Engineering Technology). This satisfies \"bachelor's degree " +
		"in engineering or a related field\", \"or equivalent experience\" and \"combination of " +
		"education and experience\" requirements for engineering, controls and manufacturing roles."
)

func kinds(issues []rationaleIssue) []string {
	out := make([]string, 0, len(issues))
	for _, i := range issues {
		out = append(out, i.Kind)
	}
	return out
}

func TestSpanMismatchIsReported(t *testing.T) {
	issues := checkRationale(r10Rationale, r10Requirement, "", nil, 0)
	var found *rationaleIssue
	for i := range issues {
		if issues[i].Kind == "span_mismatch" {
			found = &issues[i]
		}
	}
	if found == nil {
		t.Fatalf("a rationale saying 30 years with stated_span_years 0 was not reported: %v", kinds(issues))
	}
	if !strings.Contains(found.Detail, "30") {
		t.Errorf("detail does not name the span it found: %q", found.Detail)
	}
}

func TestSpanAgreementIsNotReported(t *testing.T) {
	// The ordinary case: the model wrote about thirty years and said so.
	if issues := checkRationale(r10Rationale, r10Requirement, "", nil, 30); len(issues) != 0 {
		t.Errorf("a consistent rationale was reported: %v", issues)
	}
}

func TestRationaleWithNoSpanIsNotReported(t *testing.T) {
	// Most rationales say nothing about duration, and a check that
	// fires on them is noise that gets ignored.
	r := "The evidence shows the candidate designed and commissioned medium-voltage distribution."
	if issues := checkRationale(r, r10Requirement, "", nil, 0); len(issues) != 0 {
		t.Errorf("a rationale with no span was reported: %v", issues)
	}
}

func TestQuoteFromEvidenceCalledARequirementIsReported(t *testing.T) {
	issues := checkRationale(r9Rationale, r9Requirement, "", []string{r9FactsSheet}, 0)
	var detail string
	for _, i := range issues {
		if i.Kind == "quote_unsupported" {
			detail = i.Detail
		}
	}
	if detail == "" {
		t.Fatalf("misattributed quote was not reported: %v", kinds(issues))
	}
	if !strings.Contains(detail, "cited evidence") {
		t.Errorf("detail should say where the text actually came from: %q", detail)
	}
	// "or equivalent experience" IS in the requirement and must not be
	// reported; flagging a correct quote would train the reader to
	// ignore the whole check.
	if strings.Contains(detail, "equivalent experience") {
		t.Errorf("a quote that is genuinely in the requirement was reported: %q", detail)
	}
}

func TestInventedQuoteIsReported(t *testing.T) {
	r := `The evidence shows the candidate meets the "ten years of Kubernetes administration" requirement.`
	issues := checkRationale(r, r10Requirement, "", []string{"Nothing about containers here."}, 12)
	if len(issues) == 0 {
		t.Fatal("a quote present in neither requirement nor evidence was not reported")
	}
	if !strings.Contains(issues[0].Detail, "neither") {
		t.Errorf("detail should say it is in neither source: %q", issues[0].Detail)
	}
}

func TestQuoteFromTheRequirementIsNotReported(t *testing.T) {
	r := `The posting asks for "10+ Years of Software and systems engineering" and the evidence shows it.`
	if issues := checkRationale(r, r10Requirement, "", nil, 12); len(issues) != 0 {
		t.Errorf("a quote taken from the requirement was reported: %v", issues)
	}
}

func TestQuoteFromTheSourceQuoteIsNotReported(t *testing.T) {
	// source_quote is the posting's own words, so a rationale quoting
	// it is quoting the posting.
	src := "candidates should bring 10+ Years of Software and systems engineering in fast paced environments, ideally in aerospace"
	r := `The posting says "ideally in aerospace" and the evidence does not show that.`
	if issues := checkRationale(r, r10Requirement, src, nil, 12); len(issues) != 0 {
		t.Errorf("a quote taken from the source quote was reported: %v", issues)
	}
}

func TestQuotingEvidenceWithoutCallingItARequirementIsFine(t *testing.T) {
	// Quoting the evidence is ordinary and correct. Only calling it the
	// requirement misleads a reader about what the posting asked for.
	r := `The profile states "30 years of engineering practice", which covers the ask.`
	ev := []string{"backed by 30 years of engineering practice, across mining and spirits."}
	if issues := checkRationale(r, r10Requirement, "", ev, 30); len(issues) != 0 {
		t.Errorf("quoting the evidence was reported: %v", issues)
	}
}

func TestEmptyRationaleIsNotReported(t *testing.T) {
	if issues := checkRationale("  ", r10Requirement, "", nil, 0); issues != nil {
		t.Errorf("an empty rationale produced issues: %v", issues)
	}
}

func TestProseSpanTakesTheLargest(t *testing.T) {
	// A rationale describing a career and a detail inside it is not
	// disagreeing with itself; taking the smaller number would
	// manufacture a mismatch.
	n, ok := proseSpanYears("30 years of engineering, including 3 years leading teams")
	if !ok || n != 30 {
		t.Errorf("proseSpanYears = %d, %v; want 30, true", n, ok)
	}
}
