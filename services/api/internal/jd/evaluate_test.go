package jd

import (
	"testing"

	"github.com/reh3376/career-site/services/api/internal/users"
)

func f(v float64) *float64 { return &v }

func item(name, expected string, score *float64, err string) users.EvalItem {
	return users.EvalItem{GoldenName: name, ExpectedGate: expected, MatchScore: score, Error: err}
}

// separation is the part of an evaluation that says something gate
// accuracy cannot. These cases are the reasons it exists.
func TestSeparation(t *testing.T) {
	t.Parallel()

	t.Run("clean split has no violations and a positive margin", func(t *testing.T) {
		t.Parallel()
		v, m := separation([]users.EvalItem{
			item("bosch", "above", f(0.91), ""),
			item("strong", "above", f(0.86), ""),
			item("mid", "below", f(0.59), ""),
			item("unrelated", "below", f(0.02), ""),
		})
		if v != 0 {
			t.Fatalf("violations = %d, want 0", v)
		}
		if m == nil || *m < 0.26 || *m > 0.28 {
			t.Fatalf("margin = %v, want about 0.27", m)
		}
	})

	// The case the whole measurement exists for: every posting is still
	// on the correct side of a 0.70 gate, so gate accuracy is perfect,
	// but the two groups have nearly converged. The next model change
	// crosses the line and looks sudden. A shrinking margin is the
	// warning that arrives first.
	t.Run("perfect gate accuracy can still hide a collapse", func(t *testing.T) {
		t.Parallel()
		v, m := separation([]users.EvalItem{
			item("strong", "above", f(0.71), ""),
			item("mid", "below", f(0.69), ""),
		})
		if v != 0 {
			t.Fatalf("violations = %d, want 0", v)
		}
		if m == nil || *m > 0.03 {
			t.Fatalf("margin = %v, want a small positive number", m)
		}
	})

	// An inversion is worse than a gate miss: moving the gate cannot fix
	// it, because there is no threshold that separates these two.
	t.Run("an inversion is counted and the margin goes negative", func(t *testing.T) {
		t.Parallel()
		v, m := separation([]users.EvalItem{
			item("strong", "above", f(0.55), ""),
			item("mid", "below", f(0.72), ""),
		})
		if v != 1 {
			t.Fatalf("violations = %d, want 1", v)
		}
		if m == nil || *m >= 0 {
			t.Fatalf("margin = %v, want negative", m)
		}
	})

	t.Run("counts every crossing pair, not just the worst", func(t *testing.T) {
		t.Parallel()
		v, _ := separation([]users.EvalItem{
			item("a", "above", f(0.40), ""),
			item("b", "above", f(0.45), ""),
			item("x", "below", f(0.50), ""),
			item("y", "below", f(0.42), ""),
		})
		// a is beaten by both; b is beaten by x only.
		if v != 3 {
			t.Fatalf("violations = %d, want 3", v)
		}
	})

	t.Run("failed and unscored postings are ignored, not counted as zero", func(t *testing.T) {
		t.Parallel()
		v, m := separation([]users.EvalItem{
			item("strong", "above", f(0.86), ""),
			item("broken", "above", nil, "the model host went away"),
			item("mid", "below", f(0.59), ""),
		})
		if v != 0 {
			t.Fatalf("violations = %d, want 0", v)
		}
		// Treating the failure as 0.0 would have made the margin
		// negative and reported a regression that did not happen.
		if m == nil || *m < 0.26 {
			t.Fatalf("margin = %v, want about 0.27", m)
		}
	})

	t.Run("one-sided sets have no margin to report", func(t *testing.T) {
		t.Parallel()
		if v, m := separation([]users.EvalItem{item("a", "above", f(0.9), "")}); v != 0 || m != nil {
			t.Fatalf("got (%d, %v), want (0, nil)", v, m)
		}
	})
}
