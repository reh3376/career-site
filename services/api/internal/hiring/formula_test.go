package hiring

import (
	"math"
	"testing"
)

func TestDeriveVerdict(t *testing.T) {
	cases := map[string]string{
		DetMet: VerdictMet, DetMetEquivalent: VerdictMet,
		DetUnresolved: VerdictPartial, DetNotMet: VerdictUnmet,
		"": VerdictUnmet, "other": VerdictUnmet,
	}
	for det, want := range cases {
		if got := DeriveVerdict(det); got != want {
			t.Errorf("DeriveVerdict(%q) = %q, want %q", det, got, want)
		}
	}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestCoverageV3(t *testing.T) {
	cases := []struct {
		name  string
		items []Item
		want  Coverage
	}{
		{
			name: "all must, one of each",
			items: []Item{
				{Weight: 2, Category: "must", Determination: DetMet},
				{Weight: 2, Category: "must", Determination: DetMetEquivalent},
				{Weight: 2, Category: "must", Determination: DetUnresolved},
				{Weight: 2, Category: "must", Determination: DetNotMet},
			},
			want: Coverage{Evidenced: 0.5, Unresolved: 0.25, Unmet: 0.25, EquivalentShare: 0.5, Denominator: 8},
		},
		{
			name: "nice counts only when evidenced",
			items: []Item{
				{Weight: 3, Category: "must", Determination: DetMet},
				{Weight: 1, Category: "nice", Determination: DetNotMet},
				{Weight: 1, Category: "nice", Determination: DetUnresolved},
				{Weight: 1, Category: "nice", Determination: DetMet},
			},
			want: Coverage{Evidenced: 1, Denominator: 4},
		},
		{
			name: "unresolved is its own number, never half",
			items: []Item{
				{Weight: 1, Category: "must", Determination: DetUnresolved},
				{Weight: 1, Category: "must", Determination: DetUnresolved},
			},
			want: Coverage{Unresolved: 1, Denominator: 2},
		},
		{
			name: "behavioral weight 0 is excluded",
			items: []Item{
				{Weight: 0, Category: "must", Determination: DetUnresolved},
				{Weight: 2, Category: "must", Determination: DetMet, SelfAttested: true},
			},
			want: Coverage{Evidenced: 1, SelfAttestedShare: 1, Denominator: 2},
		},
		{
			name:  "nothing judged",
			items: nil,
			want:  Coverage{},
		},
		{
			name: "self-attested share is of the evidenced weight",
			items: []Item{
				{Weight: 3, Category: "must", Determination: DetMet},
				{Weight: 1, Category: "must", Determination: DetMetEquivalent, SelfAttested: true},
				{Weight: 4, Category: "must", Determination: DetNotMet, SelfAttested: true},
			},
			want: Coverage{Evidenced: 0.5, Unmet: 0.5, EquivalentShare: 0.25, SelfAttestedShare: 0.25, Denominator: 8},
		},
	}
	for _, c := range cases {
		got := CoverageV3(c.items)
		if !near(got.Evidenced, c.want.Evidenced) || !near(got.Unresolved, c.want.Unresolved) || !near(got.Unmet, c.want.Unmet) ||
			!near(got.EquivalentShare, c.want.EquivalentShare) || !near(got.SelfAttestedShare, c.want.SelfAttestedShare) ||
			got.Denominator != c.want.Denominator {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
		if got.Denominator > 0 && !near(got.Evidenced+got.Unresolved+got.Unmet, 1) {
			t.Errorf("%s: the three numbers sum to %v, not 1", c.name, got.Evidenced+got.Unresolved+got.Unmet)
		}
	}
}
