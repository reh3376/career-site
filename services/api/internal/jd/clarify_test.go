package jd

import (
	"encoding/json"
	"testing"

	"github.com/reh3376/career-site/services/api/internal/hiring"
)

// A stored judgment as a run carries it: a years requirement the model
// found met and the duration rule set aside because the documents did
// not state the span.
func carriedYears(t *testing.T) (hiring.Requirement, hiring.CarriedJudgment) {
	t.Helper()
	years := 8.0
	q := hiring.Requirement{
		ID: "req-1", LocalID: "r1", Text: "8+ years of experience with SCADA systems", Type: "experience",
		Category: "must", Class: "required", Weight: 3, Included: true,
		Thresholds: hiring.Thresholds{MinYears: &years},
	}
	facts, _ := json.Marshal(hiring.ReportedFacts{AlternativeSatisfied: -1})
	adjs, _ := json.Marshal([]hiring.RuleAdjustment{{
		Rule: hiring.RuleDuration, RuleVersion: "v1",
		FromDetermination: hiring.DetMet, ToDetermination: hiring.DetUnresolved,
		FromBasis: hiring.BasisDirect, ToBasis: hiring.BasisDurationShortfall,
		AddedUncertainty: []string{hiring.UncDurationUnknown},
		Reason:           "the posting asks for 8 years and the evidence states no span",
	}})
	conf := 0.8
	return q, hiring.CarriedJudgment{
		ID: "j-1", RequirementID: "req-1", LocalID: "r1", RequirementType: "experience",
		Determination: hiring.DetUnresolved, Basis: hiring.BasisDurationShortfall,
		Uncertainty: []string{hiring.UncDurationUnknown},
		EvidenceIDs: []string{"ev-1"}, ChunkIDs: []int64{11},
		Rationale: "Ran the SCADA layer at two plants.", ReportedFacts: facts, RuleAdjustments: adjs,
		Confidence: &conf, DecisionID: 42,
		PromptID: "requirement_judge", PromptVersion: 14, PromptFingerprint: "v14:abc", Model: "qwen3:4b-q8_0",
	}
}

func spanAnswer(status string, engagements ...string) hiring.StoredAnswer {
	fields := hiring.QuestionFields(hiring.AnswerSpan, []hiring.Engagement{
		{Ref: "corpus_document:1", Kind: "role", Title: "Director", Organization: "Acme", Start: 2012, End: 2022},
		{Ref: "corpus_document:2", Kind: "role", Title: "Engineer", Organization: "Beta", Start: 2019, End: 2021},
	}, hiring.GenericLists{}, nil)
	a := hiring.Answer{Status: status}
	if status == hiring.ItemAnswered {
		a.Values = map[string]any{"engagements": engagements}
	}
	return hiring.StoredAnswer{
		ItemID: "item-1", RequirementID: "req-1", UncertaintyCode: hiring.UncDurationUnknown, AnswerType: hiring.AnswerSpan,
		Fields: fields, Answer: a, EvidenceItemID: "ev-answer", Statement: "The candidate's own answer.",
	}
}

// A carried judgment is the stored row in the pass's own form, marked
// as carried, with nothing recomputed.
func TestCarriedJudgmentKeepsTheRow(t *testing.T) {
	t.Parallel()
	_, c := carriedYears(t)
	j := carriedJudgment(c)
	if !j.Carried || j.RequirementID != "r1" || j.Determination != hiring.DetUnresolved || j.Verdict != "partial" ||
		j.Basis != hiring.BasisDurationShortfall || len(j.EvidenceIDs) != 1 || j.EvidenceIDs[0] != 11 ||
		len(j.RuleAdjustments) != 1 || j.Adjusted == "" || j.ReportedFacts == nil || j.Confidence == nil || j.NotAssessed {
		t.Fatalf("carried: %+v", j)
	}
	c.RequirementType = hiring.TypeBehavioral
	if !carriedJudgment(c).NotAssessed {
		t.Error("a carried working-style row is still not assessed")
	}
}

// The candidate states ten years across two roles: the duration rule
// that had set the judgment aside is run again over the model's own
// verdict and now passes, with no model call. The row is this run's,
// not a carried one, and rests on the answer.
func TestSettleOnAnswerRunsTheRuleAgain(t *testing.T) {
	t.Parallel()
	q, c := carriedYears(t)
	j, ok := settleOnAnswer(q, c, spanAnswer(hiring.ItemAnswered, "corpus_document:1", "corpus_document:2"), settleNow)
	if !ok {
		t.Fatal("a span answer to a duration question settles in code")
	}
	if j.Carried || j.Determination != hiring.DetMet || j.Basis != hiring.BasisDirect || j.Verdict != "met" || len(j.Uncertainty) != 0 {
		t.Fatalf("settled: %+v", j)
	}
	// Overlapping roles count once: 2012 to 2022 is ten years.
	if j.ReportedFacts == nil || j.ReportedFacts.StatedSpanYears != 10 || j.StatedSpanYears != 10 || j.ReportedFacts.MostRecentYear != 2022 {
		t.Fatalf("facts: %+v", j.ReportedFacts)
	}
	last := j.RuleAdjustments[len(j.RuleAdjustments)-1]
	if last.Rule != hiring.RuleClarification || last.FromDetermination != hiring.DetUnresolved || last.ToDetermination != hiring.DetMet {
		t.Fatalf("adjustments: %+v", j.RuleAdjustments)
	}
	if len(j.AnswerEvidence) != 1 || j.AnswerEvidence[0] != "ev-answer" || len(j.ClarificationItems) != 1 || j.ClarificationItems[0] != "item-1" {
		t.Fatalf("provenance: %+v %+v", j.AnswerEvidence, j.ClarificationItems)
	}
	if len(j.EvidenceIDs) != 1 || j.EvidenceIDs[0] != 11 {
		t.Fatalf("the passages the model cited stay cited: %+v", j.EvidenceIDs)
	}
}

// A stated span that falls short is a shortfall, not a pass: the
// answer is put to the rule, not taken as a yes.
func TestSettleOnAnswerCanFallShort(t *testing.T) {
	t.Parallel()
	q, c := carriedYears(t)
	j, ok := settleOnAnswer(q, c, spanAnswer(hiring.ItemAnswered, "corpus_document:2"), settleNow)
	if !ok {
		t.Fatal("settles in code")
	}
	if j.Determination == hiring.DetMet || j.ReportedFacts.StatedSpanYears != 2 {
		t.Fatalf("two years against eight must not be met: %+v facts %+v", j, j.ReportedFacts)
	}
}

// "I have not done this" settles the requirement as not met, not
// addressed, on the candidate's own statement, and cites no passage.
func TestSettleOnAnswerNotApplicable(t *testing.T) {
	t.Parallel()
	q, c := carriedYears(t)
	j, ok := settleOnAnswer(q, c, spanAnswer(hiring.ItemNotApplicable), settleNow)
	if !ok || j.Determination != hiring.DetNotMet || j.Basis != hiring.BasisNotAddressed || j.Verdict != "unmet" || len(j.EvidenceIDs) != 0 {
		t.Fatalf("not applicable: ok=%v %+v", ok, j)
	}
	if len(j.AnswerEvidence) != 1 || j.RuleAdjustments[len(j.RuleAdjustments)-1].Rule != hiring.RuleClarification {
		t.Fatalf("provenance: %+v %+v", j.AnswerEvidence, j.RuleAdjustments)
	}
}

// An answer only a judge can weigh, and a judgment no code rule had set
// aside, go to the judge.
func TestSettleOnAnswerLeavesTheRestToTheJudge(t *testing.T) {
	t.Parallel()
	q, c := carriedYears(t)
	pointer := spanAnswer(hiring.ItemAnswered, "corpus_document:1")
	pointer.AnswerType = hiring.AnswerEvidencePointer
	if _, ok := settleOnAnswer(q, c, pointer, settleNow); ok {
		t.Error("a pointer to one's own work is for the judge")
	}
	c.RuleAdjustments = nil
	if _, ok := settleOnAnswer(q, c, spanAnswer(hiring.ItemAnswered, "corpus_document:1"), settleNow); ok {
		t.Error("a judgment the model itself left unresolved is for the judge")
	}
	c.Determination = hiring.DetMet
	if _, ok := settleOnAnswer(q, c, spanAnswer(hiring.ItemNotApplicable), settleNow); ok {
		t.Error("only an unresolved judgment is settled on an answer")
	}
}

// What the judging pass is given for a standing answer: the statement,
// and for a structured answer the way its figure enters the facts.
func TestAnswerEvidence(t *testing.T) {
	t.Parallel()
	ae := answerEvidence(spanAnswer(hiring.ItemAnswered, "corpus_document:1"), settleNow)
	if ae.ItemID != "item-1" || ae.EvidenceItemID != "ev-answer" || ae.Statement == "" || ae.NotApplicable || ae.FactsFix == nil {
		t.Fatalf("answer evidence: %+v", ae)
	}
	if got := ae.FactsFix(hiring.ReportedFacts{}); got.StatedSpanYears != 10 {
		t.Errorf("the span enters the facts: %+v", got)
	}
	na := answerEvidence(spanAnswer(hiring.ItemNotApplicable), settleNow)
	if !na.NotApplicable || na.FactsFix != nil {
		t.Errorf("not applicable: %+v", na)
	}
	pointer := spanAnswer(hiring.ItemAnswered, "corpus_document:1")
	pointer.AnswerType = hiring.AnswerEvidencePointer
	if answerEvidence(pointer, settleNow).FactsFix != nil {
		t.Error("a pointer carries no figure for the rules")
	}
}

// The candidate's figure is what the rule compares when the judge is
// asked again: the question existed because the documents did not
// state it.
func TestSettleUsesTheCandidatesFigure(t *testing.T) {
	t.Parallel()
	off, hits := offered(11)
	raw := rawMet("11")
	req := settleReq("8+ years of experience with SCADA systems", "experience")
	without := settleJudgment(settleInput{req: req, raw: raw, offered: off, evidence: hits, now: settleNow})
	if without.Determination != hiring.DetUnresolved {
		t.Fatalf("with no span stated the rule sets it aside: %+v", without)
	}
	ae := answerEvidence(spanAnswer(hiring.ItemAnswered, "corpus_document:1"), settleNow)
	with := settleJudgment(settleInput{req: req, raw: raw, offered: off, evidence: hits, now: settleNow, factsFix: ae.FactsFix})
	if with.Determination != hiring.DetMet || with.StatedSpanYears != 10 {
		t.Fatalf("with the stated span it passes: %+v", with)
	}
}

// A judgment made with an answer on the table: the answer's citation
// leaves the chunk ids, and the judgment names the item it was made
// with and counts the statement among what it rests on.
func TestAnswerSettled(t *testing.T) {
	t.Parallel()
	isAnswer := func(id int64) bool { return id < 0 }
	base := Judgment{RequirementID: "r1", Determination: hiring.DetMetEquivalent, Basis: hiring.BasisAdjacentDomain, Verdict: "met", EvidenceIDs: []int64{-1, 11}}
	pointer := AnswerEvidence{ItemID: "item-1", EvidenceItemID: "ev-answer", Statement: "s"}

	cited := answerSettled(base, pointer, true, isAnswer)
	if len(cited.EvidenceIDs) != 1 || cited.EvidenceIDs[0] != 11 || len(cited.AnswerEvidence) != 1 || len(cited.ClarificationItems) != 1 {
		t.Fatalf("cited: %+v", cited)
	}
	uncited := base
	uncited.EvidenceIDs = []int64{11}
	// Shown and not cited: it was still made with the answer among its
	// evidence, and the record counts it as resting on it.
	shown := answerSettled(uncited, pointer, true, isAnswer)
	if len(shown.AnswerEvidence) != 1 || len(shown.ClarificationItems) != 1 || len(shown.EvidenceIDs) != 1 {
		t.Fatalf("shown and not cited: %+v", shown)
	}
	none := answerSettled(uncited, AnswerEvidence{}, false, isAnswer)
	if len(none.ClarificationItems) != 0 || len(none.AnswerEvidence) != 0 {
		t.Fatalf("no answer: %+v", none)
	}
	// "I have not done this" is not overruled by a reading of the
	// documents.
	na := answerSettled(base, AnswerEvidence{ItemID: "item-1", EvidenceItemID: "ev-answer", NotApplicable: true}, true, isAnswer)
	if na.Determination != hiring.DetNotMet || na.Basis != hiring.BasisNotAddressed || na.Verdict != "unmet" || len(na.EvidenceIDs) != 0 ||
		len(na.AnswerEvidence) != 1 || len(na.RuleAdjustments) != 1 || na.RuleAdjustments[0].FromDetermination != hiring.DetMetEquivalent {
		t.Fatalf("not applicable: %+v", na)
	}
}

// One closing run per request: a second claim while the first holds is
// refused, and the request can be claimed again once released.
func TestClaimClosing(t *testing.T) {
	t.Parallel()
	s := &Scorer{}
	if !s.claimClosing("req-1") {
		t.Fatal("the first claim is granted")
	}
	if s.claimClosing("req-1") {
		t.Error("a second claim on the same request is refused while the first holds")
	}
	if !s.claimClosing("req-2") {
		t.Error("another request is not affected")
	}
	s.releaseClosing("req-1")
	if !s.claimClosing("req-1") {
		t.Error("a released request can be claimed again")
	}
}
