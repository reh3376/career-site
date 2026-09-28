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
	issues := checkRationale(r10Rationale, r10Requirement, "", "met", nil, 0)
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
	if issues := checkRationale(r10Rationale, r10Requirement, "", "met", nil, 30); len(issues) != 0 {
		t.Errorf("a consistent rationale was reported: %v", issues)
	}
}

func TestRationaleWithNoSpanIsNotReported(t *testing.T) {
	// Most rationales say nothing about duration, and a check that
	// fires on them is noise that gets ignored.
	r := "The evidence shows the candidate designed and commissioned medium-voltage distribution."
	if issues := checkRationale(r, r10Requirement, "", "met", nil, 0); len(issues) != 0 {
		t.Errorf("a rationale with no span was reported: %v", issues)
	}
}

func TestQuoteFromEvidenceCalledARequirementIsReported(t *testing.T) {
	issues := checkRationale(r9Rationale, r9Requirement, "", "met", []string{r9FactsSheet}, 0)
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
	issues := checkRationale(r, r10Requirement, "", "met", []string{"Nothing about containers here."}, 12)
	if len(issues) == 0 {
		t.Fatal("a quote present in neither requirement nor evidence was not reported")
	}
	if !strings.Contains(issues[0].Detail, "neither") {
		t.Errorf("detail should say it is in neither source: %q", issues[0].Detail)
	}
}

func TestQuoteFromTheRequirementIsNotReported(t *testing.T) {
	r := `The posting asks for "10+ Years of Software and systems engineering" and the evidence shows it.`
	if issues := checkRationale(r, r10Requirement, "", "met", nil, 12); len(issues) != 0 {
		t.Errorf("a quote taken from the requirement was reported: %v", issues)
	}
}

func TestQuoteFromTheSourceQuoteIsNotReported(t *testing.T) {
	// source_quote is the posting's own words, so a rationale quoting
	// it is quoting the posting.
	src := "candidates should bring 10+ Years of Software and systems engineering in fast paced environments, ideally in aerospace"
	r := `The posting says "ideally in aerospace" and the evidence does not show that.`
	if issues := checkRationale(r, r10Requirement, src, "met", nil, 12); len(issues) != 0 {
		t.Errorf("a quote taken from the source quote was reported: %v", issues)
	}
}

func TestQuotingEvidenceWithoutCallingItARequirementIsFine(t *testing.T) {
	// Quoting the evidence is ordinary and correct. Only calling it the
	// requirement misleads a reader about what the posting asked for.
	r := `The profile states "30 years of engineering practice", which covers the ask.`
	ev := []string{"backed by 30 years of engineering practice, across mining and spirits."}
	if issues := checkRationale(r, r10Requirement, "", "met", ev, 30); len(issues) != 0 {
		t.Errorf("quoting the evidence was reported: %v", issues)
	}
}

func TestEmptyRationaleIsNotReported(t *testing.T) {
	if issues := checkRationale("  ", r10Requirement, "", "met", nil, 0); issues != nil {
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

// Run 10 reported this as an unsupported quote. Nothing is quoted: the
// apostrophes in "Bachelor's" and "Associate's" were read as a matched
// single-quote pair. A false positive of this kind is how a check gets
// ignored, which costs more than the detections it would have made.
const r10CaiRationale = "The candidate holds a Bachelor's degree in Applied Mathematics and an " +
	"Associate's degree in Electrical Engineering Technology, which are relevant to the " +
	"requirement for a Bachelor's degree in a related discipline."

func TestPossessivesAreNotQuotes(t *testing.T) {
	if got := quotedSpans(r10CaiRationale); len(got) != 0 {
		t.Errorf("possessive apostrophes were read as quotes: %q", got)
	}
	issues := checkRationale(r10CaiRationale,
		"Bachelor's degree in Engineering or a related discipline", "", "met", nil, 0)
	for _, i := range issues {
		if i.Kind == "quote_unsupported" {
			t.Errorf("false positive survived: %s", i.Detail)
		}
	}
}

func TestSingleQuotesStillWorkWhenProperlyDelimited(t *testing.T) {
	// Opens after a space, closes before a space. This is what a real
	// single-quoted span looks like when it carries no possessive.
	r := "The posting asks for 'hyperscale data centre experience' and the evidence shows none."
	got := quotedSpans(r)
	if len(got) != 1 || got[0] != "hyperscale data centre experience" {
		t.Errorf("a properly delimited single-quoted span was missed: %q", got)
	}
}

func TestCurlyQuotesAreMatched(t *testing.T) {
	r := "It cites “ten years of Kubernetes administration” as the ask."
	got := quotedSpans(r)
	if len(got) != 1 || got[0] != "ten years of Kubernetes administration" {
		t.Errorf("curly-quoted span missed: %q", got)
	}
}

func TestApostropheInsideADoubleQuotedSpanIsFine(t *testing.T) {
	// Double quotes carry no ambiguity, so a possessive inside one must
	// not break the match.
	r := `The requirement says "a Bachelor's degree in engineering" and it is not met.`
	got := quotedSpans(r)
	if len(got) != 1 || got[0] != "a Bachelor's degree in engineering" {
		t.Errorf("possessive inside double quotes broke the match: %q", got)
	}
}

// The run 11 CAI case: a requirement offering three industries,
// answered "unmet" by a rationale that grants the third one.
const r11CaiRequirement = "3+ years' experience in automation engineering within a " +
	"pharmaceutical, biotechnology, or regulated manufacturing environment"

const r11CaiRationale = "The evidence does not mention pharmaceutical, biotechnology, or " +
	"regulated manufacturing environments. The candidate's experience is in distilleries " +
	"and mining, which are regulated but not in the specified industries."

func TestDisjunctionIgnoredIsReported(t *testing.T) {
	issues := checkRationale(r11CaiRationale, r11CaiRequirement, "", "unmet", nil, 0)
	var found *rationaleIssue
	for i := range issues {
		if issues[i].Kind == "disjunction_ignored" {
			found = &issues[i]
		}
	}
	if found == nil {
		t.Fatalf("the run 11 CAI contradiction was not reported: %+v", issues)
	}
	if !strings.Contains(found.Detail, "regulated") {
		t.Errorf("the report should name the branch that was granted: %s", found.Detail)
	}
}

func TestRestatingTheRequirementToRefuseItIsNotAContradiction(t *testing.T) {
	// The first sentence of the CAI rationale on its own. Every branch
	// word is present and every one is denied. This is what an honest
	// "unmet" looks like, and reporting it would make the check noise.
	r := "The evidence does not mention pharmaceutical, biotechnology, or regulated " +
		"manufacturing environments."
	for _, i := range checkRationale(r, r11CaiRequirement, "", "unmet", nil, 0) {
		if i.Kind == "disjunction_ignored" {
			t.Errorf("a plain refusal was read as a contradiction: %s", i.Detail)
		}
	}
}

func TestDisjunctionArmIgnoresVerdictsTheCodeWeakened(t *testing.T) {
	// applyDurationRule can turn the model's "met" into "unmet". That
	// is the code disagreeing with the model, not the model with
	// itself, so the model's own verdict is what this arm reads.
	for _, i := range checkRationale(r11CaiRationale, r11CaiRequirement, "", "met", nil, 0) {
		if i.Kind == "disjunction_ignored" {
			t.Errorf("reported against a verdict the model did not give: %s", i.Detail)
		}
	}
}

func TestRequirementWithNoAlternativesIsNotChecked(t *testing.T) {
	req := "5 years of hands-on PLC programming"
	r := "The evidence shows PLC programming across several plants."
	for _, i := range checkRationale(r, req, "", "unmet", nil, 0) {
		if i.Kind == "disjunction_ignored" {
			t.Errorf("reported on a requirement offering no alternatives: %s", i.Detail)
		}
	}
}

func TestThePostingsOwnWordingIsPreferredOverTheSummary(t *testing.T) {
	// Extraction compresses, and a list is exactly what a summary
	// drops. The quote is what the branches are read from.
	summary := "automation engineering in a regulated industry"
	quote := "3+ years in a pharmaceutical, biotechnology, or regulated manufacturing environment"
	r := "His work is in distilleries, which are regulated but not the named sectors."
	var kinds []string
	for _, i := range checkRationale(r, summary, quote, "unmet", nil, 0) {
		kinds = append(kinds, i.Kind)
	}
	if len(kinds) == 0 {
		t.Errorf("the branches should have come from the quote; got no issues")
	}
}

// The three false positives the first version of the disjunction arm
// produced against real runs. Each is a rationale reciting a
// qualification the candidate does hold, which shares a word with the
// requirement and satisfies none of its branches. They are pinned
// because every one of them looked like a contradiction at a glance,
// which is exactly how a check like this becomes noise nobody reads.
func TestDisjunctionFalsePositivesFromRuns9To11(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  string
		why  string
	}{
		{
			// Branch words came from "using tools like ...", which is
			// prose introducing the list rather than an item in it.
			name: "a tool list the evidence does not answer",
			req: "Develop and review structural models, calculations, drawings, and " +
				"specifications using tools like Tekla Structures, ETABS, RISA, STAAD, or equivalent",
			why: "The evidence does not mention any use of structural modeling tools like " +
				"Tekla Structures, ETABS, RISA, STAAD, or equivalent. The candidate's evidence " +
				"focuses on control systems, AI, and manufacturing software rather than " +
				"structural engineering tools.",
		},
		{
			// "Bachelor" and "Science" are the frame a degree is stated
			// in. The branches are civil and structural.
			name: "a different degree in the same frame",
			req:  "Bachelor of Science in Civil or Structural Engineering from an accredited university",
			why: "The evidence does not show the candidate has a Bachelor of Science in Civil " +
				"or Structural Engineering from an accredited university. The candidate has a " +
				"Bachelor of Science in Applied Mathematics and an Associate of Science in " +
				"Electrical Engineering Technology, but no degree in Civil or Structural Engineering.",
		},
		{
			// "Applied" matched inside "Applied Mathematics", the name
			// of a degree the candidate holds at a lower level than the
			// one asked for.
			name: "a branch word absorbed into a proper noun",
			req: "PhD (or equivalent industry experience) in Computer Science, Machine Learning, " +
				"Natural Language Processing, Applied Math, Computational Biology, Statistics, " +
				"or a related field",
			why: "The evidence does not mention a PhD or equivalent industry experience in " +
				"Computer Science, Machine Learning, Natural Language Processing, Applied Math, " +
				"Computational Biology, Statistics, or a related field. The candidate holds a " +
				"B.S. in Applied Mathematics and an A.S. in Electrical Engineering Technology, " +
				"but no graduate degree is mentioned.",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, i := range checkRationale(tc.why, tc.req, "", "unmet", nil, 0) {
				if i.Kind == "disjunction_ignored" {
					t.Errorf("false positive: %s", i.Detail)
				}
			}
		})
	}
}

func TestBranchMatchesAreWholeWords(t *testing.T) {
	// "deregulated" is not "regulated", and reading it as one reports
	// the opposite of what the rationale said.
	req := "experience in a pharmaceutical, biotechnology, or regulated manufacturing environment"
	why := "The candidate's markets are deregulated and the evidence names no regulated producer."
	for _, i := range checkRationale(why, req, "", "unmet", nil, 0) {
		if i.Kind == "disjunction_ignored" {
			t.Errorf("a substring match was read as a branch: %s", i.Detail)
		}
	}
}
