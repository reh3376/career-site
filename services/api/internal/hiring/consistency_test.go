package hiring

import "testing"

func TestViolationsNameEveryBrokenRule(t *testing.T) {
	t.Parallel()
	codes := DefaultVocab()
	none := ReportedFacts{AlternativeSatisfied: -1}
	cases := []struct {
		name  string
		v     Verdict
		facts ReportedFacts
		ev    int
		want  int
	}{
		{"consistent met", Verdict{DetMet, BasisDirect, nil}, none, 1, 0},
		{"met via an offered alternative", Verdict{DetMet, BasisExperienceForEducation, nil}, ReportedFacts{AlternativeSatisfied: 1}, 1, 0},
		{"met with an equivalency basis", Verdict{DetMet, BasisVocabulary, nil}, none, 1, 1},
		{"met_equivalent on direct", Verdict{DetMetEquivalent, BasisDirect, nil}, none, 1, 1},
		{"met_equivalent consistent", Verdict{DetMetEquivalent, BasisToolEquivalent, nil}, none, 1, 0},
		{"unresolved without a code", Verdict{DetUnresolved, BasisVocabulary, nil}, none, 1, 1},
		{"unresolved consistent", Verdict{DetUnresolved, BasisVocabulary, []string{UncEquivalenceUnconfirmed}}, none, 1, 0},
		{"not_met with a code", Verdict{DetNotMet, BasisContradicted, []string{UncDepthUnknown}}, none, 1, 1},
		{"not_addressed citing evidence", Verdict{DetNotMet, BasisNotAddressed, nil}, none, 2, 1},
		{"contradicted citing nothing", Verdict{DetNotMet, BasisContradicted, nil}, none, 0, 1},
		{"not_addressed consistent", Verdict{DetNotMet, BasisNotAddressed, nil}, none, 0, 0},
		{"unknown codes", Verdict{"maybe", "vibes", []string{"dunno"}}, none, 1, 3},
	}
	for _, c := range cases {
		got := Violations(c.v, c.facts, c.ev, codes)
		if len(got) != c.want {
			t.Errorf("%s: %d violations %v, want %d", c.name, len(got), got, c.want)
		}
	}
}

func TestRepairKeepsTheFindingAndFixesTheLabel(t *testing.T) {
	t.Parallel()
	codes := DefaultVocab()
	none := ReportedFacts{AlternativeSatisfied: -1}

	// A met with an equivalency basis is a met_equivalent mislabelled.
	out, clear, adj := Repair(Verdict{DetMet, BasisVocabulary, nil}, none, 1, codes)
	if out.Determination != DetMetEquivalent || out.Basis != BasisVocabulary || clear || adj == nil || adj.Rule != RuleConsistency {
		t.Fatalf("met/vocabulary -> %+v clear=%v adj=%+v", out, clear, adj)
	}
	// A met with a shortfall basis is the question that basis describes.
	out, _, _ = Repair(Verdict{DetMet, BasisDurationShortfall, nil}, none, 1, codes)
	if out.Determination != DetUnresolved || out.Uncertainty[0] != UncDurationUnknown {
		t.Fatalf("met/duration_shortfall -> %+v", out)
	}
	// A met with an absence basis is not met.
	out, _, _ = Repair(Verdict{DetMet, BasisNotAddressed, nil}, none, 0, codes)
	if out.Determination != DetNotMet || out.Basis != BasisNotAddressed {
		t.Fatalf("met/not_addressed -> %+v", out)
	}
	// met_equivalent on direct is a plain met.
	out, _, _ = Repair(Verdict{DetMetEquivalent, BasisDirect, nil}, none, 1, codes)
	if out.Determination != DetMet {
		t.Fatalf("met_equivalent/direct -> %+v", out)
	}
	// unresolved with no code gets the one its basis implies, else ambiguity.
	out, _, _ = Repair(Verdict{DetUnresolved, BasisScopeShortfall, nil}, none, 1, codes)
	if len(out.Uncertainty) != 1 || out.Uncertainty[0] != UncScopeUnknown {
		t.Fatalf("unresolved/scope_shortfall -> %+v", out)
	}
	out, _, _ = Repair(Verdict{DetUnresolved, BasisVocabulary, nil}, none, 1, codes)
	if out.Uncertainty[0] != UncEvidenceAmbiguous {
		t.Fatalf("unresolved/vocabulary -> %+v", out)
	}
	// not_met: codes dropped; not_addressed drops its evidence; a basis
	// with no evidence becomes not_addressed.
	out, clear, _ = Repair(Verdict{DetNotMet, BasisNotAddressed, []string{UncDepthUnknown}}, none, 3, codes)
	if len(out.Uncertainty) != 0 || !clear {
		t.Fatalf("not_met/not_addressed with evidence -> %+v clear=%v", out, clear)
	}
	out, _, _ = Repair(Verdict{DetNotMet, BasisContradicted, nil}, none, 0, codes)
	if out.Basis != BasisNotAddressed {
		t.Fatalf("contradicted with no evidence -> %+v", out)
	}
	// Unknown codes fall to the safe values.
	out, _, _ = Repair(Verdict{"maybe", "vibes", []string{"dunno"}}, none, 1, codes)
	if out.Determination != DetUnresolved || out.Basis != BasisNotAddressed || out.Uncertainty[0] != UncEvidenceAmbiguous {
		t.Fatalf("unknown codes -> %+v", out)
	}
	// A consistent judgment is returned unchanged with no adjustment.
	v := Verdict{DetMet, BasisDirect, nil}
	out, clear, adj = Repair(v, none, 1, codes)
	if out.Determination != DetMet || clear || adj != nil {
		t.Fatalf("consistent -> %+v clear=%v adj=%+v", out, clear, adj)
	}
	// Every repaired judgment is itself consistent.
	for _, v := range []Verdict{
		{DetMet, BasisVocabulary, nil}, {DetMet, BasisDurationShortfall, nil}, {DetMet, BasisContradicted, nil},
		{DetMetEquivalent, BasisRecency, nil}, {DetUnresolved, BasisDirect, nil}, {DetNotMet, BasisContradicted, []string{UncDepthUnknown}},
	} {
		out, _, _ := Repair(v, none, 1, codes)
		if viol := Violations(out, none, 1, codes); len(viol) != 0 {
			t.Errorf("repair of %+v left %v", v, viol)
		}
	}
}

// The compiled-in vocabulary and the seeded registry are the same list
// in the same order with the same words, or a prompt rendered from one
// would carry a different fingerprint from one rendered from the other.
func TestDefaultVocabMatchesTheSeededRegistry(t *testing.T) {
	r, ctx := testRepo(t)
	reg, err := r.LoadVocab(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got, want := VocabFromRegistry(reg), DefaultVocab()
	check := func(name string, a, b []CodeDoc) {
		if len(a) != len(b) {
			t.Fatalf("%s: registry has %d codes, the default %d", name, len(a), len(b))
		}
		for i := range a {
			if a[i] != b[i] {
				t.Errorf("%s[%d]: registry %+v, default %+v", name, i, a[i], b[i])
			}
		}
	}
	check("determination", got.Determination, want.Determination)
	check("basis", got.Basis, want.Basis)
	check("uncertainty", got.Uncertainty, want.Uncertainty)
	check("involvement_level", got.InvolvementLevel, want.InvolvementLevel)
}
