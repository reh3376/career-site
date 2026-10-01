package users

import (
	"context"
	"testing"
)

// makeRun inserts one jd_runs row in a given state and returns its id.
//
// finished is passed as a flag rather than a time because the only thing
// that matters here is whether the column was written at all: that is
// the half of run 32's shape the old reaper tripped on.
func makeRun(t *testing.T, r *Repo, ctx context.Context, status string, finished bool) int64 {
	t.Helper()

	// A run needs a submission to point at, and the FK cascades, so
	// deleting the submission in cleanup takes the run with it.
	//
	// The uuids come from Postgres rather than from a Go package: the
	// api does not otherwise depend on a uuid library, and a test is a
	// poor reason to add one.
	var subID int64
	err := r.pool.QueryRow(ctx, `
    INSERT INTO jd_submissions (source_kind, jd_text, jd_hash, text_head, status)
    VALUES ('paste', 'reaper test posting',
            sha256(gen_random_uuid()::text::bytea), 'reaper test', 'received')
    RETURNING id`).Scan(&subID)
	if err != nil {
		t.Fatalf("make submission: %v", err)
	}
	t.Cleanup(func() {
		_, _ = r.pool.Exec(context.Background(),
			`DELETE FROM jd_submissions WHERE id = $1`, subID)
	})

	var runID int64
	err = r.pool.QueryRow(ctx, `
    INSERT INTO jd_runs (run_id, submission_id, status, finished_at)
    VALUES (gen_random_uuid(), $1, $2,
            CASE WHEN $3::boolean THEN now() ELSE NULL END)
    RETURNING id`, subID, status, finished).Scan(&runID)
	if err != nil {
		t.Fatalf("make run (%s): %v", status, err)
	}
	return runID
}

func runStatus(t *testing.T, r *Repo, ctx context.Context, id int64) string {
	t.Helper()
	var s string
	if err := r.pool.QueryRow(ctx,
		`SELECT status FROM jd_runs WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatalf("read status: %v", err)
	}
	return s
}

// The reaper must close every non-terminal run, whatever it is called
// and whether or not finished_at was written.
//
// This is run 32's bug, as a test. That row sat at `generating` with a
// finished_at on production for six days because the reaper looked for
// `status = 'running' AND finished_at IS NULL` and the row matched
// neither half. Both halves are represented below; the `generating,
// finished` case is the exact shape that escaped.
func TestFailStrandedRunsClosesEveryNonTerminalState(t *testing.T) {
	r, ctx := chatRepo(t)

	stranded := []struct {
		status   string
		finished bool
	}{
		{"running", false},    // the case the old query did catch
		{"running", true},     // died after writing finished_at
		{"generating", false}, // a status the old query did not know
		{"generating", true},  // run 32, exactly
	}

	ids := make([]int64, len(stranded))
	for i, c := range stranded {
		ids[i] = makeRun(t, r, ctx, c.status, c.finished)
	}

	if _, err := r.FailStrandedRuns(ctx); err != nil {
		t.Fatalf("reap: %v", err)
	}

	for i, c := range stranded {
		if got := runStatus(t, r, ctx, ids[i]); got != "failed" {
			t.Errorf("run left at %q: a %s run with finished_at=%v was not reaped",
				got, c.status, c.finished)
		}
	}
}

// A run that already ended is evidence, and the reaper must not rewrite
// it. Reaping a terminal row would turn a real below_threshold result
// into a failure and quietly corrupt the record the reviewer is judged
// on.
func TestFailStrandedRunsLeavesTerminalRunsAlone(t *testing.T) {
	r, ctx := chatRepo(t)

	for _, status := range jdRunTerminalStatuses {
		t.Run(status, func(t *testing.T) {
			id := makeRun(t, r, ctx, status, true)
			if _, err := r.FailStrandedRuns(ctx); err != nil {
				t.Fatalf("reap: %v", err)
			}
			if got := runStatus(t, r, ctx, id); got != status {
				t.Errorf("terminal run %q was rewritten to %q", status, got)
			}
		})
	}
}

// An error already recorded survives the reaper.
//
// The reason the run stopped is more useful than the fact that a later
// restart noticed, so the generic message is only written where there is
// nothing there yet.
func TestFailStrandedRunsKeepsAnExistingError(t *testing.T) {
	r, ctx := chatRepo(t)
	id := makeRun(t, r, ctx, "running", false)

	const real = "ollama refused the request"
	if _, err := r.pool.Exec(ctx,
		`UPDATE jd_runs SET error = $2 WHERE id = $1`, id, real); err != nil {
		t.Fatalf("set error: %v", err)
	}
	if _, err := r.FailStrandedRuns(ctx); err != nil {
		t.Fatalf("reap: %v", err)
	}

	var gotErr string
	if err := r.pool.QueryRow(ctx,
		`SELECT error FROM jd_runs WHERE id = $1`, id).Scan(&gotErr); err != nil {
		t.Fatalf("read error: %v", err)
	}
	if gotErr != real {
		t.Errorf("error overwritten: want %q, got %q", real, gotErr)
	}
}

// finished_at is when the work stopped, not when a later boot noticed.
//
// Overwriting it with now() would date every stranded run to an
// unrelated restart, which is the one thing the column is read for.
func TestFailStrandedRunsPreservesFinishedAt(t *testing.T) {
	r, ctx := chatRepo(t)
	id := makeRun(t, r, ctx, "generating", true)

	var before, after *string
	if err := r.pool.QueryRow(ctx,
		`SELECT finished_at::text FROM jd_runs WHERE id = $1`, id).Scan(&before); err != nil {
		t.Fatalf("read finished_at: %v", err)
	}
	if _, err := r.FailStrandedRuns(ctx); err != nil {
		t.Fatalf("reap: %v", err)
	}
	if err := r.pool.QueryRow(ctx,
		`SELECT finished_at::text FROM jd_runs WHERE id = $1`, id).Scan(&after); err != nil {
		t.Fatalf("read finished_at: %v", err)
	}
	if before == nil || after == nil || *before != *after {
		t.Errorf("finished_at moved: was %v, now %v", before, after)
	}
}

// The reaper and the cache-slot check must agree about what in flight
// means.
//
// JDRunInProgress gates chat prefix warming on this table. If it counts
// a state the reaper does not close, a dead run pauses warming for ever
// and the only symptom is answers quietly becoming a minute slower. This
// asserts the pair directly: after a reap, nothing is in progress.
func TestNothingIsInProgressAfterAReap(t *testing.T) {
	r, ctx := chatRepo(t)

	makeRun(t, r, ctx, "running", false)
	makeRun(t, r, ctx, "generating", true)

	if _, err := r.FailStrandedRuns(ctx); err != nil {
		t.Fatalf("reap: %v", err)
	}

	busy, err := r.JDRunInProgress(ctx)
	if err != nil {
		t.Fatalf("in progress: %v", err)
	}
	if busy {
		t.Error("a run is still reported in progress after a reap; " +
			"prefix warming would stay paused for ever")
	}
}
