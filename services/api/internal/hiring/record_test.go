package hiring

import (
	"slices"
	"testing"
)

// The gap class is derived from the basis (FR-HF-17), and every basis
// of the vocabulary has a stated place: a new basis code that lands
// without one fails here rather than rendering as "no gap".
func TestGapClassCoversTheVocabulary(t *testing.T) {
	want := map[string]string{
		BasisDirect:                 "",
		BasisVocabulary:             GapLexical,
		BasisToolEquivalent:         GapLexical,
		BasisExperienceForEducation: GapLexical,
		BasisEducationForExperience: GapLexical,
		BasisAdjacentDomain:         GapLexical,
		BasisScopeShortfall:         GapSubstantive,
		BasisDurationShortfall:      GapSubstantive,
		BasisRecency:                GapSubstantive,
		BasisRelationshipUnproven:   GapSubstantive,
		BasisContradicted:           GapSubstantive,
		BasisNotAddressed:           GapUnaddressed,
	}
	bases := Codes(DefaultVocab().Basis)
	if len(bases) != len(want) {
		t.Fatalf("the vocabulary has %d basis codes and this test places %d; give the new one a gap class", len(bases), len(want))
	}
	for _, b := range bases {
		w, ok := want[b]
		if !ok {
			t.Errorf("basis %q has no stated gap class", b)
			continue
		}
		if got := GapClass(b); got != w {
			t.Errorf("GapClass(%q) = %q, want %q", b, got, w)
		}
	}
	if got := GapClass("a_code_from_a_later_vocabulary"); got != "" {
		t.Errorf("an unknown basis must have no gap class, got %q", got)
	}
}

// What a reviewer may set: each determination with the bases that do
// not contradict it. The override form is built from this list, so the
// list and the validation are the same function.
func TestOverrideBases(t *testing.T) {
	bases := Codes(DefaultVocab().Basis)
	for det, want := range map[string][]string{
		DetMet:           {BasisDirect},
		DetMetEquivalent: {BasisVocabulary, BasisToolEquivalent, BasisExperienceForEducation, BasisEducationForExperience, BasisAdjacentDomain},
		DetUnresolved: {BasisVocabulary, BasisToolEquivalent, BasisExperienceForEducation, BasisEducationForExperience, BasisAdjacentDomain,
			BasisScopeShortfall, BasisDurationShortfall, BasisRecency, BasisRelationshipUnproven},
		DetNotMet: {BasisVocabulary, BasisToolEquivalent, BasisExperienceForEducation, BasisEducationForExperience, BasisAdjacentDomain,
			BasisScopeShortfall, BasisDurationShortfall, BasisRecency, BasisRelationshipUnproven, BasisNotAddressed, BasisContradicted},
	} {
		got := OverrideBases(det, bases)
		if !slices.Equal(got, want) {
			t.Errorf("OverrideBases(%s) = %v, want %v", det, got, want)
		}
		for _, b := range bases {
			allowed := slices.Contains(want, b)
			if v := OverrideViolation(det, b); (v == "") != allowed {
				t.Errorf("OverrideViolation(%s, %s) = %q; allowed should be %v", det, b, v, allowed)
			}
		}
	}
	if len(OverrideBases("partial", bases)) != 0 || OverrideViolation("partial", BasisDirect) == "" {
		t.Error("a determination outside the vocabulary must take no basis")
	}
}

// An override moves the numbers under the formula the record was
// written with, at the fraction it was written with.
func TestRecordCoverage(t *testing.T) {
	items := []Item{
		{Weight: 2, Class: ClassRequired, Determination: DetMet},
		{Weight: 3, Class: ClassRequired, Determination: DetMetEquivalent},
		{Weight: 2, Class: ClassPreferred, Determination: DetNotMet},
		{Weight: 0, Class: ClassRequired, Determination: DetUnresolved}, // behavioral
	}
	c, ok := RecordCoverage(FormulaV4, items, 0.5)
	if !ok || !near(c.Evidenced, 5.0/6) || !near(c.Unmet, 1.0/6) || !near(c.EquivalentShare, 0.6) || c.Unresolved != 0 {
		t.Errorf("v4 at 0.5: %+v ok=%v", c, ok)
	}
	if _, ok := RecordCoverage(FormulaV3, items, 0.5); !ok {
		t.Error("a v3 record has three numbers to compute")
	}
	if _, ok := RecordCoverage(FormulaV2, items, 0.5); ok {
		t.Error("a v2 record holds a score, not three numbers; nothing to compute")
	}
}

func TestFlattenVersions(t *testing.T) {
	got, fraction := flattenVersions([]byte(`{"formula":"v4-classes","preferred_fraction":0.4,"rules":"","prompts":{"requirement_judge":"v14:42984a17"},"model":"qwen3:4b-q8_0"}`))
	if fraction != 0.4 {
		t.Errorf("fraction = %v, want the record's own 0.4", fraction)
	}
	for k, want := range map[string]string{
		"formula": "v4-classes", "preferred_fraction": "0.4", "prompt.requirement_judge": "v14:42984a17", "model": "qwen3:4b-q8_0",
	} {
		if got[k] != want {
			t.Errorf("versions[%s] = %q, want %q", k, got[k], want)
		}
	}
	if _, ok := got["rules"]; ok {
		t.Error("an empty version is left out rather than shown blank")
	}
	if _, f := flattenVersions(nil); f != DefaultPreferredFraction {
		t.Errorf("a record with no snapshot falls back to the default fraction, got %v", f)
	}
}

// The gate's score, written once more for the fit after review, against
// the cases the scorer's own arithmetic is documented with: required
// items always in the denominator, any other only when it earned
// something, unresolved worth half, working-style rows left out, and
// every item counted when nothing is required.
func TestScoreV2(t *testing.T) {
	for name, c := range map[string]struct {
		items []Item
		want  float64
	}{
		"required only": {[]Item{
			{Weight: 2, Class: ClassRequired, Determination: DetMet},
			{Weight: 3, Class: ClassRequired, Determination: DetUnresolved},
		}, 3.5 / 5},
		"an unmet preference costs nothing": {[]Item{
			{Weight: 2, Class: ClassRequired, Determination: DetMet},
			{Weight: 3, Class: ClassRequired, Determination: DetUnresolved},
			{Weight: 2, Class: ClassPreferred, Determination: DetNotMet},
		}, 3.5 / 5},
		"an evidenced preference joins the denominator": {[]Item{
			{Weight: 2, Class: ClassRequired, Determination: DetNotMet},
			{Weight: 2, Class: ClassPreferred, Determination: DetMetEquivalent},
		}, 2.0 / 4},
		"a working-style row weighs nothing": {[]Item{
			{Weight: 2, Class: ClassRequired, Determination: DetMet},
			{Weight: 0, Class: ClassRequired, Determination: DetUnresolved},
		}, 1},
		"nothing required counts everything": {[]Item{
			{Weight: 1, Class: ClassPreferred, Determination: DetMet},
			{Weight: 3, Class: ClassPreferred, Determination: DetNotMet},
		}, 1.0 / 4},
		"the class comes from the category when the row has none": {[]Item{
			{Weight: 2, Category: "must", Determination: DetNotMet},
			{Weight: 2, Category: "nice", Determination: DetNotMet},
		}, 0},
		"no items": {nil, 0},
	} {
		if got := ScoreV2(c.items); !near(got, c.want) {
			t.Errorf("%s: ScoreV2 = %v, want %v", name, got, c.want)
		}
	}
}

// What a reader other than the hiring side is sent: the record as the
// model left it, with no trace of an override and the numbers computed
// over the model's own verdicts.
func TestModelOnly(t *testing.T) {
	rec := Record{
		FormulaVersion: FormulaV4, PreferredFraction: 0.25, HasNumbers: true, OverrideCount: 1,
		Coverage: Coverage{Evidenced: 1},
		Requirements: []Requirement{
			{ID: "q1", Weight: 2, Class: ClassRequired, Included: true},
			{ID: "q2", Weight: 3, Class: ClassRequired, Included: true},
			{ID: "q3", Weight: 1, Class: ClassRequired, Included: true, Type: TypeBehavioral},
		},
		Judgments: []RecordJudgment{
			{RequirementID: "q1", Determination: DetMet, Basis: BasisDirect, EffectiveDetermination: DetMet, EffectiveBasis: BasisDirect},
			{RequirementID: "q2", Determination: DetNotMet, Basis: BasisNotAddressed,
				EffectiveDetermination: DetMetEquivalent, EffectiveBasis: BasisAdjacentDomain, GapClass: GapLexical,
				Override: &RecordOverride{Determination: DetMetEquivalent, Basis: BasisAdjacentDomain, Reason: "the reviewer's note"}},
			{RequirementID: "q3", Determination: DetUnresolved, Basis: BasisNotAddressed, EffectiveDetermination: DetUnresolved,
				EffectiveBasis: BasisNotAddressed, NotAssessed: true, Earlier: &EarlierOverride{Attempt: 1}},
		},
	}
	if score, ok := rec.ReviewedScore(); !ok || !near(score, 1) {
		t.Errorf("ReviewedScore = %v, %v; with the override both required rows are met", score, ok)
	}
	if got := rec.ModelScore(); !near(got, 2.0/5) {
		t.Errorf("ModelScore = %v, want the run's own 0.4", got)
	}
	m := rec.ModelOnly()
	if m.OverrideCount != 0 || !near(m.Coverage.Evidenced, 2.0/5) || !near(m.Coverage.Unmet, 3.0/5) {
		t.Errorf("model-only numbers: count=%d coverage=%+v", m.OverrideCount, m.Coverage)
	}
	j := m.Judgments[1]
	if j.Override != nil || j.EffectiveDetermination != DetNotMet || j.EffectiveBasis != BasisNotAddressed || j.GapClass != GapUnaddressed {
		t.Errorf("a model-only judgment must read as the model left it: %+v", j)
	}
	if m.Judgments[2].Earlier != nil || m.Judgments[2].GapClass != "" {
		t.Errorf("no earlier override and no gap on a working-style row: %+v", m.Judgments[2])
	}
	if _, ok := m.ReviewedScore(); ok {
		t.Error("a model-only record has nothing reviewed to score")
	}
	// The record it was made from is untouched.
	if rec.Judgments[1].Override == nil || rec.OverrideCount != 1 || rec.Coverage.Evidenced != 1 {
		t.Error("ModelOnly must not change the record it reads")
	}
}
