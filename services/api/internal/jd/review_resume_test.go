package jd

import (
	"testing"

	"github.com/reh3376/career-site/services/api/internal/hiring"
)

// The résumé writer is handed the record as it stands after review: a
// person's verdict where one was set, and only the passages the judge
// cited, so an override can change what is emphasised and cannot add
// evidence.
func TestAssessmentFromRecord(t *testing.T) {
	rec := hiring.Record{
		Requirements: []hiring.Requirement{
			{ID: "q1", LocalID: "r1", Text: "Ignition SCADA", Category: "must", Class: hiring.ClassRequired, Weight: 2, Included: true},
			{ID: "q2", LocalID: "r2", Text: "Front-end loading", Category: "must", Class: hiring.ClassRequired, Weight: 3, Included: true},
			{ID: "q3", LocalID: "r3", Text: "Thrives in ambiguity", Category: "must", Class: hiring.ClassRequired, Weight: 1, Included: true, Type: hiring.TypeBehavioral},
		},
		Judgments: []hiring.RecordJudgment{
			{RequirementID: "q1", LocalID: "r1", Determination: hiring.DetMet, EffectiveDetermination: hiring.DetMet, EffectiveBasis: hiring.BasisDirect,
				Evidence: []hiring.RecordEvidence{{ID: "e1", ChunkID: 41}, {ID: "e2", ChunkID: 42, Withdrawn: true}, {ID: "e3"}}},
			{RequirementID: "q2", LocalID: "r2", Determination: hiring.DetNotMet, Basis: hiring.BasisNotAddressed,
				EffectiveDetermination: hiring.DetMetEquivalent, EffectiveBasis: hiring.BasisAdjacentDomain,
				Override: &hiring.RecordOverride{Determination: hiring.DetMetEquivalent, Basis: hiring.BasisAdjacentDomain}},
			{RequirementID: "q3", LocalID: "r3", Determination: hiring.DetUnresolved, EffectiveDetermination: hiring.DetUnresolved, NotAssessed: true},
		},
	}
	a := assessmentFromRecord(rec)
	if len(a.Requirements) != 3 || a.Requirements[1].ID != "r2" || a.Requirements[1].Text != "Front-end loading" {
		t.Fatalf("requirements: %+v", a.Requirements)
	}
	if len(a.Judgments) != 3 {
		t.Fatalf("judgments: %+v", a.Judgments)
	}
	j1, j2, j3 := a.Judgments[0], a.Judgments[1], a.Judgments[2]
	if j1.RequirementID != "r1" || j1.Verdict != "met" || len(j1.EvidenceIDs) != 1 || j1.EvidenceIDs[0] != 41 {
		t.Errorf("r1: the cited passage, without the withdrawn one or one that is not a passage: %+v", j1)
	}
	if j2.Verdict != "met" || j2.Determination != hiring.DetMetEquivalent || len(j2.EvidenceIDs) != 0 {
		t.Errorf("r2: a person's verdict is the one handed over, and it brings no evidence of its own: %+v", j2)
	}
	if j3.Verdict != "partial" || !j3.NotAssessed {
		t.Errorf("r3: a working-style row is handed over as it was recorded: %+v", j3)
	}
}

func TestCanWriteResumeNeedsTheWriterAndTheRows(t *testing.T) {
	var none *Scorer
	if none.CanWriteResume() || (&Scorer{}).CanWriteResume() {
		t.Error("a scorer without a résumé writer or the hiring tables cannot write a résumé after review")
	}
}
