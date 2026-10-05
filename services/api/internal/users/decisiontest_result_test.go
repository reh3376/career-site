package users

import "testing"

// The result email tells a participant something about themselves, once,
// with no way to correct it afterwards. The arithmetic behind the
// sentence has to be right, and the branch it picks has to match the
// numbers it just printed.

func TestResultArithmetic(t *testing.T) {
	// The effect the test exists to demonstrate: accuracy collapses,
	// confidence barely moves.
	r := DTResult{EarlyAccuracy: 83, EarlyConfidence: 79, HardAccuracy: 50, HardConfidence: 76}
	if got := r.AccuracyDrop(); got != 33 {
		t.Errorf("AccuracyDrop() = %d, want 33", got)
	}
	if got := r.ConfidenceDrop(); got != 3 {
		t.Errorf("ConfidenceDrop() = %d, want 3", got)
	}
	// The gap is what the email leads with, so it is the number most
	// worth pinning.
	if got := r.Gap(); got != 30 {
		t.Errorf("Gap() = %d, want 30", got)
	}
}

// Three different things can happen to a person, and the email says a
// different true thing about each. Getting the branch wrong would tell
// somebody their calibration was good when it collapsed.
func TestWhichSentenceTheEmailPicks(t *testing.T) {
	tests := []struct {
		name string
		r    DTResult
		want string // "effect" | "overcorrected" | "calibrated"
	}{
		{
			name: "the effect: accuracy falls, confidence does not",
			r:    DTResult{EarlyAccuracy: 83, EarlyConfidence: 79, HardAccuracy: 50, HardConfidence: 76},
			want: "effect",
		},
		{
			name: "calibrated: both move together",
			r:    DTResult{EarlyAccuracy: 80, EarlyConfidence: 75, HardAccuracy: 55, HardConfidence: 52},
			want: "calibrated",
		},
		{
			name: "overcorrected: confidence falls further than accuracy",
			r:    DTResult{EarlyAccuracy: 80, EarlyConfidence: 80, HardAccuracy: 75, HardConfidence: 55},
			want: "overcorrected",
		},
		{
			name: "no decline at all is not the effect",
			r:    DTResult{EarlyAccuracy: 70, EarlyConfidence: 70, HardAccuracy: 70, HardConfidence: 70},
			want: "calibrated",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Same thresholds the template branches on.
			var got string
			switch g := tc.r.Gap(); {
			case g > 10:
				got = "effect"
			case g < -5:
				got = "overcorrected"
			default:
				got = "calibrated"
			}
			if got != tc.want {
				t.Errorf("gap %d picked %q, want %q", tc.r.Gap(), got, tc.want)
			}
		})
	}
}

// Block 5 against block 1 is the only thing separating load from
// fatigue, and the email states which one happened. A tolerance that is
// too tight calls ordinary variation fatigue; too loose and a genuine
// collapse is reported as "it was the load".
func TestFatigueHeld(t *testing.T) {
	tests := []struct {
		name           string
		block1, block5 int
		want           bool
	}{
		{"identical", 83, 83, true},
		{"block 5 slightly worse, within noise", 83, 75, true},
		{"block 5 better, which happens", 70, 85, true},
		{"block 5 clearly worse: fatigue is in play", 83, 50, false},
		{"exactly at the edge", 80, 65, true},
		{"just past the edge", 80, 64, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := DTResult{Block1Accuracy: tc.block1, Block5Accuracy: tc.block5}
			if got := r.FatigueHeld(); got != tc.want {
				t.Errorf("block1=%d block5=%d: FatigueHeld() = %v, want %v",
					tc.block1, tc.block5, got, tc.want)
			}
		})
	}
}

// A run where nothing was answered must not produce a confident sentence
// about nothing. Zeroes throughout should land on the neutral branch
// rather than claiming an effect.
func TestEmptyRunSaysNothingDramatic(t *testing.T) {
	r := DTResult{}
	if r.Gap() != 0 {
		t.Errorf("Gap() = %d on an empty run, want 0", r.Gap())
	}
	if !r.FatigueHeld() {
		t.Error("an empty run should not be reported as fatigued")
	}
}
