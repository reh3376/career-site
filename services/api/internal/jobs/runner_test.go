package jobs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func quietRunner() *Runner {
	return New(slog.New(slog.NewTextHandler(io.Discard, nil)), time.Hour, time.Minute)
}

// waitFor polls until the job reaches a terminal status, so the tests
// do not depend on a fixed sleep.
func waitFor(t *testing.T, r *Runner, id string) *Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if j, ok := r.Get(id); ok && j.FinishedAt != nil {
			return j
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("job %s did not finish", id)
	return nil
}

func TestEventsRecordEveryReportInOrder(t *testing.T) {
	r := quietRunner()
	started, err := r.Start("test", func(_ context.Context, report Report) (string, error) {
		report(10, "first")
		report(50, "second")
		report(90, "third")
		return "done", nil
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	j := waitFor(t, r, started.ID)

	// Three reports plus the terminal event.
	if len(j.Events) != 4 {
		t.Fatalf("got %d events, want 4: %+v", len(j.Events), j.Events)
	}
	want := []string{"first", "second", "third", "done"}
	for i, w := range want {
		if j.Events[i].Summary != w {
			t.Errorf("event %d summary = %q, want %q", i, j.Events[i].Summary, w)
		}
	}
	if j.Events[3].Progress != 100 {
		t.Errorf("terminal event progress = %d, want 100", j.Events[3].Progress)
	}
	for i := 1; i < len(j.Events); i++ {
		if j.Events[i].At.Before(j.Events[i-1].At) {
			t.Errorf("event %d is timestamped before its predecessor", i)
		}
	}
}

func TestFailureIsRecordedInTheTimeline(t *testing.T) {
	r := quietRunner()
	started, _ := r.Start("test", func(_ context.Context, report Report) (string, error) {
		report(25, "got some way in")
		return "", errors.New("the model went away")
	})
	j := waitFor(t, r, started.ID)

	if j.Status != StatusFailed {
		t.Fatalf("status = %s, want failed", j.Status)
	}
	last := j.Events[len(j.Events)-1]
	if last.Summary != "failed: the model went away" {
		t.Errorf("last event = %q, want the failure", last.Summary)
	}
	// A history that stops at the last progress report cannot tell a
	// finished job from an unobserved one, which is the question the
	// timeline exists to answer.
	if len(j.Events) != 2 {
		t.Errorf("got %d events, want the report plus the failure", len(j.Events))
	}
}

func TestEventsAreCappedAndDropTheOldest(t *testing.T) {
	r := quietRunner()
	started, _ := r.Start("test", func(_ context.Context, report Report) (string, error) {
		for i := 0; i < MaxEvents+50; i++ {
			report(0, "tick")
		}
		return "finished", nil
	})
	j := waitFor(t, r, started.ID)

	if len(j.Events) > MaxEvents+1 {
		t.Fatalf("got %d events, cap is %d", len(j.Events), MaxEvents)
	}
	// The recent history is what a reader is watching, so the start of
	// the run is what gets dropped, not the end.
	if j.Events[len(j.Events)-1].Summary != "finished" {
		t.Error("the terminal event was dropped; the cap is trimming the wrong end")
	}
}

func TestSnapshotDoesNotShareEventsWithTheRunningJob(t *testing.T) {
	// `c := *j` copies the slice header, so without an explicit copy the
	// caller reads the same backing array the job's goroutine appends
	// to. That is a data race, and after the cap is reached the re-slice
	// would shift the caller's view under it.
	r := quietRunner()
	release := make(chan struct{})
	started, _ := r.Start("test", func(_ context.Context, report Report) (string, error) {
		report(10, "before")
		<-release
		report(20, "after")
		return "done", nil
	})

	// Let the first report land.
	var snap *Job
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if j, ok := r.Get(started.ID); ok && len(j.Events) > 0 {
			snap = j
			break
		}
		time.Sleep(time.Millisecond)
	}
	if snap == nil {
		t.Fatal("no snapshot with events")
	}
	before := len(snap.Events)

	close(release)
	waitFor(t, r, started.ID)

	if len(snap.Events) != before {
		t.Errorf("the earlier snapshot grew from %d to %d events; it aliases the live job",
			before, len(snap.Events))
	}
	// Mutating the copy must not reach the runner either.
	snap.Events[0].Summary = "tampered"
	live, _ := r.Get(started.ID)
	if live.Events[0].Summary == "tampered" {
		t.Error("mutating a snapshot changed the job the runner holds")
	}
}

// Run with -race. Readers poll while the job reports, which is exactly
// what /admin/ops does.
func TestConcurrentReadsWhileReporting(t *testing.T) {
	r := quietRunner()
	started, _ := r.Start("test", func(_ context.Context, report Report) (string, error) {
		for i := 0; i < 200; i++ {
			report(int32(i%100), "working")
		}
		return "done", nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for k := 0; k < 100; k++ {
				if j, ok := r.Get(started.ID); ok {
					for _, e := range j.Events {
						_ = e.Summary
					}
				}
				_ = r.Recent(25)
			}
		}()
	}
	wg.Wait()
	waitFor(t, r, started.ID)
}
