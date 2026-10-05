package users

import (
	"strings"
	"testing"
)

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

// Four different things can happen to a person, and the email says a
// different true thing about each. Getting the reading wrong would tell
// somebody their calibration was good when it collapsed, or hand them a
// flattering explanation of a decline that never happened.
//
// This now asserts DTResult.Reading rather than re-deriving the
// thresholds, which is the point: the decision lives in one place and
// the template prints it.
func TestWhichSentenceTheEmailPicks(t *testing.T) {
	tests := []struct {
		name string
		r    DTResult
		want string // "effect" | "overcorrected" | "calibrated" | "no_decline"
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
			name: "nothing moved: there is no decline to explain",
			r:    DTResult{EarlyAccuracy: 70, EarlyConfidence: 70, HardAccuracy: 70, HardConfidence: 70},
			want: "no_decline",
		},
		{
			// The owner's own test send on 2026-10-05. Accuracy ROSE 16
			// points and confidence slipped 3, giving a gap of -19,
			// which the old template read as "overcorrected" and told
			// him he had registered the difficulty and adjusted for it.
			// Nothing had got harder for him. no_decline has to outrank
			// the gap, because every other reading presupposes a fall.
			name: "accuracy rose: must not be read as overcorrection",
			r:    DTResult{EarlyAccuracy: 17, EarlyConfidence: 81, HardAccuracy: 33, HardConfidence: 78},
			want: "no_decline",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.r.Reading(); got != tc.want {
				t.Errorf("Reading() = %q, want %q (accuracy drop %d, gap %d)",
					got, tc.want, tc.r.AccuracyDrop(), tc.r.Gap())
			}
		})
	}
}

// A participant reads these two sentences as the summary of their own
// fifteen minutes. "Your accuracy moved -16 points" went out to the
// owner on the first real send: a minus sign doing the work a verb
// should do, and pointing the wrong way.
func TestMovementReadsAsEnglish(t *testing.T) {
	tests := []struct {
		name string
		r    DTResult
		acc  string
		conf string
	}{
		{
			name: "a fall says fell",
			r:    DTResult{EarlyAccuracy: 83, EarlyConfidence: 79, HardAccuracy: 50, HardConfidence: 76},
			acc:  "Your accuracy fell 33 points",
			conf: "Your confidence fell 3 points",
		},
		{
			name: "a rise says rose, with no minus sign",
			r:    DTResult{EarlyAccuracy: 17, EarlyConfidence: 81, HardAccuracy: 33, HardConfidence: 78},
			acc:  "Your accuracy rose 16 points",
			conf: "Your confidence fell 3 points",
		},
		{
			name: "no movement says so rather than printing a zero",
			r:    DTResult{EarlyAccuracy: 70, EarlyConfidence: 70, HardAccuracy: 70, HardConfidence: 70},
			acc:  "Your accuracy did not move",
			conf: "Your confidence did not move",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.r.AccuracyPhrase(); got != tc.acc {
				t.Errorf("AccuracyPhrase() = %q, want %q", got, tc.acc)
			}
			if got := tc.r.ConfidencePhrase(); got != tc.conf {
				t.Errorf("ConfidencePhrase() = %q, want %q", got, tc.conf)
			}
		})
	}
	// No phrase may contain a minus sign: the direction is the verb.
	for _, r := range []DTResult{
		{EarlyAccuracy: 10, HardAccuracy: 90, EarlyConfidence: 10, HardConfidence: 90},
		{EarlyAccuracy: 90, HardAccuracy: 10, EarlyConfidence: 90, HardConfidence: 10},
	} {
		for _, p := range []string{r.AccuracyPhrase(), r.ConfidencePhrase()} {
			if strings.Contains(p, "-") {
				t.Errorf("phrase carries a minus sign: %q", p)
			}
		}
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
