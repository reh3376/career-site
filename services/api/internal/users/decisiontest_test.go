package users

import "testing"

// Grading is the part of the decision test that cannot be wrong,
// because it runs once per answer against a volunteer's only sitting
// and there is no way to re-ask them.

// The transformation is what makes blocks 3 and 4 the steep part of the
// ramp: they move the task from holding digits to holding digits and
// operating on them.
func TestExpectedDigitsAppliesTheBlockTransformation(t *testing.T) {
	tests := []struct {
		name, digits, load, want string
	}{
		{"block 1 holds only", "482", "d3", "482"},
		{"block 2 holds only", "5173", "d4", "5173"},
		{"block 3 adds one", "1234", "d4_plus1", "2345"},
		{"block 4 adds three", "1234", "d4_plus3", "4567"},
		{"block 5 holds only, the control", "907", "d3_control", "907"},

		// Wrapping is not a detail. Without it a +3 block could never
		// show a digit above six, and a participant would learn the rule
		// from the numbers rather than from the instruction.
		{"nine plus one wraps to zero", "9999", "d4_plus1", "0000"},
		{"nine plus three wraps to two", "9876", "d4_plus3", "2109"},
		{"mixed wrap", "8261", "d4_plus3", "1594"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DTExpectedDigits(tc.digits, tc.load); got != tc.want {
				t.Errorf("DTExpectedDigits(%q, %q) = %q, want %q", tc.digits, tc.load, got, tc.want)
			}
		})
	}
}

// The distinction this pins is the sharpest signal in the instrument.
//
// Returning the digits untransformed means the number survived and the
// operation did not: storage held, the executive failed. Scoring recall
// pass/fail would erase exactly that, and it is the mechanism the whole
// test exists to measure.
func TestGradeRecallSeparatesStorageFailureFromExecutiveFailure(t *testing.T) {
	tests := []struct {
		name, presented, expected, response, wantOutcome string
		wantCorrect                                      int
	}{
		{"held it and transformed it", "1234", "4567", "4567", "exact", 4},
		{
			name: "held it and did not transform it", presented: "1234", expected: "4567",
			response: "1234", wantOutcome: "untransformed", wantCorrect: 0,
		},
		{"lost the number entirely", "1234", "4567", "8888", "wrong_digits", 0},
		{"some of it survived", "1234", "4567", "4500", "partial", 2},
		{"nothing entered, the step expired", "1234", "4567", "", "expired", 0},
		{"whitespace only is still an expiry", "1234", "4567", "   ", "expired", 0},

		// In a no-transform block, presented equals expected, so a
		// correct answer must read as exact rather than as
		// untransformed. The guard for that is the presented != expected
		// clause, and this is the test that would catch its removal.
		{"no-transform block answered correctly", "482", "482", "482", "exact", 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotOutcome, gotCorrect := DTGradeRecall(tc.presented, tc.expected, tc.response)
			if gotOutcome != tc.wantOutcome {
				t.Errorf("outcome = %q, want %q", gotOutcome, tc.wantOutcome)
			}
			if gotCorrect != tc.wantCorrect {
				t.Errorf("digitsCorrect = %d, want %d", gotCorrect, tc.wantCorrect)
			}
		})
	}
}

// The ramp has to run in the owner's order, and block 5 has to be the
// control rather than a sixth step up. A reordering here would leave
// load confounded with time-on-task and nothing in the data would say
// so.
func TestBlockOrderEndsWithTheFatigueControl(t *testing.T) {
	want := []string{"d3", "d4", "d4_plus1", "d4_plus3", "d3_control"}
	if len(dtBlockLoads) != len(want) {
		t.Fatalf("got %d blocks, want %d", len(dtBlockLoads), len(want))
	}
	for i := range want {
		if dtBlockLoads[i] != want[i] {
			t.Errorf("block %d = %q, want %q", i+1, dtBlockLoads[i], want[i])
		}
	}
	// The control must match block 1's difficulty, or it controls
	// nothing: same digit count, same absence of a transformation.
	if dtDigitCount["d3_control"] != dtDigitCount["d3"] {
		t.Error("block 5 does not hold the same number of digits as block 1, so it is not a control")
	}
	if dtTransform["d3_control"] != dtTransform["d3"] {
		t.Error("block 5 applies a different transformation from block 1, so it is not a control")
	}
}

// Thirty questions at six a block is what fits the owner's fifteen
// minutes once intake, instructions, practice and the thank-you come
// out. A change here changes the time budget.
func TestInstrumentShape(t *testing.T) {
	if DTBlockCount != 5 || DTQuestionsPerBlock != 6 || DTQuestionCount != 30 {
		t.Errorf("shape = %d blocks x %d = %d, want 5 x 6 = 30",
			DTBlockCount, DTQuestionsPerBlock, DTQuestionCount)
	}
}

// Digits are generated per session so a participant who takes the test
// twice does not meet the same numbers, which would make the second run
// a memory test of the first.
func TestRandomDigitsAreDigitsAndVary(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 40; i++ {
		got, err := dtRandomDigits(4)
		if err != nil {
			t.Fatalf("dtRandomDigits: %v", err)
		}
		if len(got) != 4 {
			t.Fatalf("length = %d, want 4", len(got))
		}
		for _, c := range got {
			if c < '0' || c > '9' {
				t.Fatalf("non-digit %q in %q", c, got)
			}
		}
		seen[got] = true
	}
	// Forty draws from ten thousand landing on one value would mean the
	// generator is not generating.
	if len(seen) < 2 {
		t.Error("forty draws produced one value; the numbers are not varying between sessions")
	}
}
