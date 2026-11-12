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
