package jd

import (
	"testing"
	"time"
)

// Every stage the pipeline actually reports must land in a real phase.
// These strings are copied from the progress calls in
// Scorer.ScoreAndPersist and Assessor.Assess; if one is reworded there
// and not here, this still passes, which is why phaseFor falls back to
// "other" rather than guessing: the drift shows up in the data.
func TestEveryReportedStageHasAPhase(t *testing.T) {
	for stage, want := range map[string]string{
		"starting":                                  phaseSetup,
		"queued behind another review":              phaseSetup,
		"checking the posting":                      phasePostingCheck,
		"reading the posting":                       phaseExtract,
		"12 requirements found; gathering evidence": phaseRetrieval,
		"judging requirement 1 of 14":               phaseJudge,
		"judging requirement 14 of 14":              phaseJudge,
		"computing the score":                       phaseScore,
		"writing the tailored résumé":               phaseResume,
		"rendering the PDF":                         phaseRender,
	} {
		if got := phaseFor(stage); got != want {
			t.Errorf("phaseFor(%q) = %q, want %q", stage, got, want)
		}
	}
}

func TestUnknownStageIsVisibleRatherThanFoldedIn(t *testing.T) {
	if got := phaseFor("doing something nobody mapped"); got != phaseOther {
		t.Errorf("got %q, want %q: an unmapped stage must not inflate a neighbouring phase", got, phaseOther)
	}
}

// The fourteen judging reports are one phase, not fourteen entries.
func TestRepeatedStagesAccumulateIntoOnePhase(t *testing.T) {
	p := newPhaseTimer(time.Now())
	for i := 0; i < 5; i++ {
		p.enter("judging requirement 1 of 14")
		time.Sleep(2 * time.Millisecond)
	}
	p.stop()

	got := p.snapshot()
	if _, ok := got[phaseJudge]; !ok {
		t.Fatalf("no judge phase recorded: %v", got)
	}
	if n := len(got); n > 2 { // judge, plus the setup span before the first report
		t.Errorf("got %d phases, want the judging reports collapsed into one: %v", n, got)
	}
}

// The run worth understanding is the one that failed part way through,
// so a timer that is stopped mid-pipeline must still report what it
// measured. Blue Origin died in judging twice; "we know it reached
// judging and spent the time there" is the whole value.
func TestTimingsSurviveStoppingPartWayThrough(t *testing.T) {
	p := newPhaseTimer(time.Now())
	p.enter("checking the posting")
	time.Sleep(2 * time.Millisecond)
	p.enter("judging requirement 11 of 14")
	time.Sleep(2 * time.Millisecond)
	p.stop() // as the deferred close does, after the error returned

	got := p.snapshot()
	if got[phaseJudge] <= 0 {
		t.Errorf("the phase it died in recorded no time: %v", got)
	}
	if got[phaseResume] != 0 {
		t.Errorf("recorded time for a phase never reached: %v", got)
	}
}

// stop runs from a defer that also fires on the failure path, and the
// repository coalesces on nil, so a second stop must not inflate or
// erase anything.
func TestStopIsIdempotent(t *testing.T) {
	p := newPhaseTimer(time.Now())
	p.enter("judging requirement 1 of 14")
	time.Sleep(2 * time.Millisecond)
	p.stop()
	first := p.snapshot()[phaseJudge]

	time.Sleep(3 * time.Millisecond)
	p.stop()
	if second := p.snapshot()[phaseJudge]; second != first {
		t.Errorf("a second stop changed the judge phase from %d to %d", first, second)
	}
}

// A run that dies waiting for a pipeline slot never builds a timer, and
// the deferred close calls straight into a nil one.
func TestNilTimerIsSafe(t *testing.T) {
	var p *phaseTimer
	p.enter("judging requirement 1 of 14")
	p.stop()
	if got := p.snapshot(); got != nil {
		t.Errorf("a nil timer produced %v, want nil so the column is left alone", got)
	}
}

// Nothing measured must read as nil, not as an empty object, or the
// close would overwrite a previous value with {}.
func TestNothingMeasuredIsNil(t *testing.T) {
	if got := newPhaseTimer(time.Now()).snapshot(); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}
