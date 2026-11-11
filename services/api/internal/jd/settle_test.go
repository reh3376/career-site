package jd

import (
	"testing"
	"time"

	"github.com/reh3376/career-site/services/api/internal/hiring"
	"github.com/reh3376/career-site/services/api/internal/prompts"
	"github.com/reh3376/career-site/services/api/internal/users"
)

var settleNow = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

func settleReq(text, typ string, parties ...string) prompts.Requirement {
	return prompts.Requirement{ID: "r1", Text: text, Category: "must", Weight: 3, Type: typ, NamedParties: parties}
}

func rawMet(ids ...string) rawJudgment {
	return rawJudgment{
		RequirementID: "r1", Determination: hiring.DetMet, Basis: hiring.BasisDirect, Uncertainty: []string{},
		EvidenceIDs: ids, Rationale: "Designed and ran the SCADA layer.",
		ReportedFacts: hiring.ReportedFacts{AlternativeSatisfied: -1, InvolvementLevel: "designed"},
	}
}

func offered(ids ...int64) (map[int64]bool, []users.CorpusHit) {
	m := map[int64]bool{}
	var hits []users.CorpusHit
	for _, id := range ids {
		m[id] = true
		hits = append(hits, users.CorpusHit{Chunk: users.CorpusChunk{ID: id, Text: "chunk"}})
	}
	return m, hits
}

// A consistent, evidenced answer passes through untouched, with the
// three-way verdict derived from it and the facts kept whole.
func TestSettleKeepsAConsistentAnswer(t *testing.T) {
	t.Parallel()
	off, hits := offered(11, 12)
	j := settleJudgment(settleInput{req: settleReq("Experience with Ignition SCADA", "tool"), raw: rawMet("11"), offered: off, evidence: hits, now: settleNow})
	if j.Determination != hiring.DetMet || j.Basis != hiring.BasisDirect || j.Verdict != "met" || len(j.RuleAdjustments) != 0 || j.Adjusted != "" {
		t.Fatalf("got %+v", j)
	}
	if len(j.EvidenceIDs) != 1 || j.EvidenceIDs[0] != 11 || j.ReportedFacts == nil || j.ReportedFacts.InvolvementLevel != "designed" || j.Type != "tool" {
		t.Fatalf("evidence/facts/type: %+v", j)
	}
}

// An id the judge was never shown is dropped; when nothing offered
// remains, the evidence rule makes it not_met / not_addressed.
func TestSettleDropsUnofferedEvidence(t *testing.T) {
	t.Parallel()
	off, hits := offered(11)
	j := settleJudgment(settleInput{req: settleReq("Experience with Ignition SCADA", "tool"), raw: rawMet("99", "banana"), offered: off, evidence: hits, now: settleNow})
	if j.Determination != hiring.DetNotMet || j.Basis != hiring.BasisNotAddressed || j.Verdict != "unmet" || len(j.EvidenceIDs) != 0 {
		t.Fatalf("got %+v", j)
	}
	if len(j.RuleAdjustments) != 1 || j.RuleAdjustments[0].Rule != hiring.RuleEvidence {
		t.Fatalf("adjustments: %+v", j.RuleAdjustments)
	}
}

// The duration rule reaches the settled judgment with its adjustment
// visible, and the derived verdict follows the determination.
func TestSettleAppliesTheCodeRules(t *testing.T) {
	t.Parallel()
	off, hits := offered(11)
	raw := rawMet("11")
	raw.ReportedFacts.StatedSpanYears = 0
	j := settleJudgment(settleInput{req: settleReq("5+ years of PLC programming", "experience"), raw: raw, offered: off, evidence: hits, now: settleNow})
	if j.Determination != hiring.DetUnresolved || j.Verdict != "partial" || len(j.Uncertainty) != 1 || j.Uncertainty[0] != hiring.UncDurationUnknown {
		t.Fatalf("got %+v", j)
	}
	if j.Adjusted == "" || j.RuleAdjustments[0].Rule != hiring.RuleDuration {
		t.Fatalf("adjustment not visible: %+v", j)
	}
	// The relationship rule too.
	raw = rawMet("11")
	j = settleJudgment(settleInput{req: settleReq("Partner with Anthropic", "relationship", "Anthropic"), raw: raw, offered: off, evidence: hits, now: settleNow})
	if j.Determination != hiring.DetUnresolved || j.Basis != hiring.BasisRelationshipUnproven {
		t.Fatalf("relationship: %+v", j)
	}
}

// A second consistency failure is repaired in code and recorded.
func TestSettleRepairsAfterTheRetry(t *testing.T) {
	t.Parallel()
	off, hits := offered(11)
	raw := rawMet("11")
	raw.Basis = hiring.BasisVocabulary // met with an equivalency basis
	j := settleJudgment(settleInput{req: settleReq("Experience with Ignition SCADA", "tool"), raw: raw, offered: off, evidence: hits, now: settleNow, repair: true})
	if j.Determination != hiring.DetMetEquivalent || j.Verdict != "met" || j.RuleAdjustments[0].Rule != hiring.RuleConsistency {
		t.Fatalf("got %+v", j)
	}
	// Even without the repair flag an inconsistent row is never stored.
	j = settleJudgment(settleInput{req: settleReq("Experience with Ignition SCADA", "tool"), raw: raw, offered: off, evidence: hits, now: settleNow})
	if len(hiring.Violations(hiring.Verdict{Determination: j.Determination, Basis: j.Basis, Uncertainty: j.Uncertainty}, *j.ReportedFacts, len(j.EvidenceIDs), hiring.DefaultVocab())) != 0 {
		t.Fatalf("an inconsistent judgment was stored: %+v", j)
	}
}

// Behavioral requirements are recorded, not judged.
func TestNotAssessedJudgment(t *testing.T) {
	t.Parallel()
	j := notAssessedJudgment(settleReq("Self-starter with a passion for excellence", "behavioral"))
	if !j.NotAssessed || j.Determination != hiring.DetUnresolved || j.Verdict != "partial" || len(j.EvidenceIDs) != 0 || j.Uncertainty[0] != hiring.UncEvidenceAmbiguous {
		t.Fatalf("got %+v", j)
	}
	if j.RuleAdjustments[0].Rule != hiring.RuleBehavioral {
		t.Fatalf("adjustment: %+v", j.RuleAdjustments)
	}
}

// consistencyViolations counts evidence as the model cited it against
// what was offered, so a retry message names the real problem.
func TestConsistencyViolationsReadTheOfferedIds(t *testing.T) {
	t.Parallel()
	a := quietAssessor(nil)
	raws := []rawJudgment{{RequirementID: "r1", Determination: hiring.DetNotMet, Basis: hiring.BasisNotAddressed, EvidenceIDs: []string{"11"},
		ReportedFacts: hiring.ReportedFacts{AlternativeSatisfied: -1}}}
	if v := a.consistencyViolations(raws, map[string][]int64{"r1": {11}}); len(v) != 1 {
		t.Fatalf("not_addressed citing an offered chunk should be one violation: %v", v)
	}
	if v := a.consistencyViolations(raws, map[string][]int64{"r1": {12}}); len(v) != 0 {
		t.Fatalf("a cited id that was not offered counts as no evidence: %v", v)
	}
}
