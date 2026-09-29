package jd

import (
	"context"
	"testing"
)

// The member path carries no observer, and that is the common case: it
// is every submission but an evaluation's. So the nil observer has to
// be safe to call, not merely safe to hold, because the scorer's
// progress closure calls it on every stage without checking.
func TestNilObserverIsSafeToNotify(t *testing.T) {
	observerFrom(context.Background()).notify(50, "judging requirement 7 of 14")
}

func TestObserverRoundTrips(t *testing.T) {
	type call struct {
		pct   int32
		stage string
	}
	var got []call
	ctx := withObserver(context.Background(), func(pct int32, stage string) {
		got = append(got, call{pct, stage})
	})

	observerFrom(ctx).notify(4, "checking the posting")
	observerFrom(ctx).notify(80, "writing the tailored résumé")

	if len(got) != 2 {
		t.Fatalf("got %d calls, want 2", len(got))
	}
	if got[0] != (call{4, "checking the posting"}) {
		t.Errorf("first call was %+v", got[0])
	}
	if got[1] != (call{80, "writing the tailored résumé"}) {
		t.Errorf("second call was %+v", got[1])
	}
}

// withObserver(nil) must not install a typed nil that observerFrom then
// hands back as non-nil. It returns the context untouched instead.
func TestWithNilObserverLeavesContextAlone(t *testing.T) {
	base := context.Background()
	if ctx := withObserver(base, nil); ctx != base {
		t.Error("withObserver(nil) wrapped the context")
	}
	if fn := observerFrom(withObserver(base, nil)); fn != nil {
		t.Error("a nil observer came back non-nil")
	}
}

// An observer belongs to one scoring run. A context derived for the
// next posting must not inherit the previous one's.
func TestObserverDoesNotLeakBetweenRuns(t *testing.T) {
	first := 0
	ctx := withObserver(context.Background(), func(int32, string) { first++ })

	second := 0
	next := withObserver(ctx, func(int32, string) { second++ })
	observerFrom(next).notify(10, "starting")

	if first != 0 {
		t.Errorf("the earlier observer was called %d times", first)
	}
	if second != 1 {
		t.Errorf("the current observer was called %d times, want 1", second)
	}
}

// Run takes a Status that the job runner supplies. Nothing guarantees
// one is set, and a run with no status must still score.
func TestNilStatusIsSafeToSet(t *testing.T) {
	var s Status
	s.set(42, "scoring blue-origin (5 of 9)")
}

func TestStatusSetForwards(t *testing.T) {
	var gotPct int32
	var gotSummary string
	s := Status(func(pct int32, summary string) { gotPct, gotSummary = pct, summary })

	s.set(42, "scoring blue-origin (5 of 9), judging requirement 7 of 14")

	if gotPct != 42 {
		t.Errorf("percent was %d, want 42", gotPct)
	}
	if gotSummary != "scoring blue-origin (5 of 9), judging requirement 7 of 14" {
		t.Errorf("summary was %q", gotSummary)
	}
}
