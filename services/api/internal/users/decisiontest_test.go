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
			gotOutcome, gotCorrect, _ := DTGradeRecall(tc.presented, tc.expected, tc.response)
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

// Memory severity, the owner's measure: an incorrect number is a memory
// failure and what can be scored is how bad it was. Miss one digit of
// four and a quarter of the number was lost; miss all four and it is a
// complete loss.
//
// The case that decides the design is the untransformed one. Scored
// against the expected number alone it reads as total memory failure
// when the number was in fact held perfectly, which is backwards and
// would bury the storage-held-executive-failed signal entirely.
func TestDigitsHeldMeasuresRetentionNotCorrectness(t *testing.T) {
	tests := []struct {
		name                      string
		presented, expected, resp string
		wantHeld                  int
		wantSeverity              float64 // 1 - held/len
	}{
		{
			name:      "held and transformed: nothing lost",
			presented: "5359", expected: "6460", resp: "6460",
			wantHeld: 4, wantSeverity: 0,
		},
		{
			// The owner's run 3 block 3. Memory was perfect; only the
			// operation failed. Against the expected number this scores
			// zero digits and would read as 1.0.
			name:      "held but not transformed: still nothing lost",
			presented: "3777", expected: "4888", resp: "3777",
			wantHeld: 4, wantSeverity: 0,
		},
		{
			name:      "one digit off on a no-transform block",
			presented: "929", expected: "929", resp: "927",
			wantHeld: 2, wantSeverity: 1.0 / 3.0,
		},
		{
			name:      "mostly lost under a transformation",
			presented: "7462", expected: "0795", resp: "0649",
			wantHeld: 1, wantSeverity: 0.75,
		},
		{
			name:      "lost entirely",
			presented: "1234", expected: "1234", resp: "8888",
			wantHeld: 0, wantSeverity: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, held := DTGradeRecall(tc.presented, tc.expected, tc.resp)
			if held != tc.wantHeld {
				t.Errorf("digitsHeld = %d, want %d", held, tc.wantHeld)
			}
			sev := 1 - float64(held)/float64(len(tc.presented))
			if diff := sev - tc.wantSeverity; diff > 0.01 || diff < -0.01 {
				t.Errorf("severity = %.2f, want %.2f", sev, tc.wantSeverity)
			}
		})
	}
}

// An expired recall is not a memory failure of any size: nothing was
// attempted. Scoring it 1.0 would put "ran out of time" and "forgot
// completely" in the same bucket, and the view returns null instead.
func TestExpiredRecallHoldsNothingAndIsNotScored(t *testing.T) {
	outcome, _, held := DTGradeRecall("1234", "4567", "")
	if outcome != "expired" {
		t.Errorf("outcome = %q, want expired", outcome)
	}
	if held != 0 {
		t.Errorf("digitsHeld = %d on an expired recall, want 0", held)
	}
}
