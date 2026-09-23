package jd

import "testing"

// How many years a requirement asks for is read from the posting's own
// words, so these are the words postings actually use.
func TestRequiredYears(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		text string
		want int
		asks bool
	}{
		{"plus form", "5+ years in building products that apply Machine Learning Models", 5, true},
		{"spaced plus", "2 + years working with Large Language Models", 2, true},
		{"capitalised", "10+ Years of Software and systems engineering", 10, true},
		{"or more", "7 or more years in process manufacturing", 7, true},
		{"spelled out", "at least three years leading development teams", 3, true},
		{"abbreviated", "8 yrs of controls experience", 8, true},

		// When a requirement carries two spans, the larger one is its
		// real ask. Taking the smaller would let the easier half of a
		// compound requirement satisfy the whole of it.
		{
			"compound takes the larger",
			"10+ Years of Software and systems engineering in fast paced environments with 3+ years leading development teams",
			10, true,
		},

		// Requirements with no duration must not acquire one, or every
		// capability requirement would start being downgraded.
		{"no duration", "Strong understanding of software architecture and design patterns", 0, false},
		{"degree not a duration", "Masters or equivalent experience in Computer Science", 0, false},

		// Numbers that are not years. A posting is full of them.
		{"travel percentage", "Willingness to travel up to 10% of the time", 0, false},
		{"team size", "Lead a team of 12 engineers", 0, false},
		{"a standard number", "Experience with NFPA 70E and UL 508A", 0, false},
		{"a year of grace", "Founded in 1996", 0, false},

		// A span longer than a working life is a parse gone wrong.
		{"implausible span ignored", "500 years of experience", 0, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, asks := RequiredYears(c.text)
			if got != c.want || asks != c.asks {
				t.Fatalf("got (%d, %v), want (%d, %v) for %q", got, asks, c.want, c.asks, c.text)
			}
		})
	}
}

// The rule only ever weakens a verdict, and only when the requirement
// asks for a span the evidence does not support.
func TestApplyDurationRule(t *testing.T) {
	t.Parallel()

	t.Run("span covers the ask, verdict stands", func(t *testing.T) {
		t.Parallel()
		v, note := applyDurationRule("2+ Years building products that use Large Language Models", "met", 2)
		if v != "met" || note != "" {
			t.Fatalf("got (%q, %q), want met with no note", v, note)
		}
	})

	// The failure this whole file exists for: a five-year requirement
	// answered by citing a two-year project.
	t.Run("span falls short, met becomes partial", func(t *testing.T) {
		t.Parallel()
		v, note := applyDurationRule("5+ years in building products that apply Machine Learning Models", "met", 2)
		if v != "partial" {
			t.Fatalf("verdict = %q, want partial", v)
		}
		if note == "" {
			t.Fatal("an adjustment the reader cannot see is indistinguishable from the model saying so itself")
		}
	})

	t.Run("no span stated at all, met becomes partial", func(t *testing.T) {
		t.Parallel()
		v, note := applyDurationRule("5+ years applying Machine Learning Models", "met", 0)
		if v != "partial" || note == "" {
			t.Fatalf("got (%q, %q), want partial with a note", v, note)
		}
	})

	t.Run("requirement asks for no span, nothing is touched", func(t *testing.T) {
		t.Parallel()
		v, note := applyDurationRule("Strong understanding of software architecture", "met", 0)
		if v != "met" || note != "" {
			t.Fatalf("got (%q, %q), want met untouched", v, note)
		}
	})

	// It must never argue a candidate up. A judge that said unmet or
	// partial knows something this rule does not.
	t.Run("never strengthens a verdict", func(t *testing.T) {
		t.Parallel()
		for _, start := range []string{"unmet", "partial"} {
			if v, _ := applyDurationRule("5+ years of X", start, 30); v != start {
				t.Fatalf("a %q verdict with a 30 year span became %q; the rule must only weaken", start, v)
			}
		}
	})

	t.Run("a longer span than asked for is fine", func(t *testing.T) {
		t.Parallel()
		if v, _ := applyDurationRule("5+ years of process control", "met", 15); v != "met" {
			t.Fatalf("verdict = %q, want met", v)
		}
	})
}
