package users

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The decision test's write path, executed against a real Postgres.
//
// **Why this exists.** Two production outages in one evening, both from
// SQL that compiled, vetted, formatted and shipped:
//
//  1. The dt_sessions INSERT listed sixteen columns against fifteen
//     values, so StartSession returned 500 and nobody could begin the
//     test for roughly two hours.
//  2. The review UPDATE assigned a parameter Postgres had inferred as
//     text to a bigint column, so the Save button on the curation panel
//     did nothing at all and the owner found it by pressing it.
//
// Neither was catchable anywhere. `go build` and `go vet` do not look
// inside a string literal, `gofmt` has no opinion about SQL, the
// migrations job replays the schema but runs no Go, and the web tests
// never reach the api. A static check caught the first class and would
// not have caught the second: a column count is countable from source,
// a type inference is not.
//
// The only thing that catches both is executing the query. So this does
// that, against a database, for every write the decision test performs.
//
// Skipped when TEST_DATABASE_URL is unset, so `go test ./...` on a
// laptop with no Postgres still passes. CI sets it, which is the point:
// a skip locally is a convenience, a skip in CI would be the same hole
// again, so the CI job sets it unconditionally.
func dtTestRepo(t *testing.T) (*Repo, context.Context) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		// Skipping locally is a convenience. Skipping in CI would
		// silently disable the only guard that catches SQL faults, and
		// `go test` without -v prints nothing for a skip, so the hole
		// would look exactly like a pass. That is the same shape as
		// every fault this file exists for, so CI fails instead.
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_DATABASE_URL is unset in CI: the database-backed tests " +
				"would skip silently, which is indistinguishable from passing")
		}
		t.Skip("TEST_DATABASE_URL not set; skipping the database-backed path")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return New(pool), ctx
}

// A whole sitting, start to finish, then curated. Every statement in
// the participant and curation paths runs at least once.
func TestTheWholeWritePathAgainstPostgres(t *testing.T) {
	r, ctx := dtTestRepo(t)

	// Synthetic, so a test run can never be mistaken for data and is
	// excluded from every aggregate by the same flag real agent runs use.
	sess, err := r.StartDecisionTest(ctx,
		DTIntake{DisplayName: "db test", Email: "dbtest@example.test", WantsResults: false},
		DTConditions{AudioMode: "sound", DeviceClass: "desktop", TapCheckPassed: true,
			BaselineRTMs: 300, BaselineRTSDMs: 40},
		true, "db-test-visitor", nil,
		"v2-m6000-q25000-r20000", "items-2026-10-05", "key-1")
	if err != nil {
		// This is the failure that took the test down. The message
		// Postgres gives is specific and worth surfacing whole.
		t.Fatalf("StartDecisionTest: %v", err)
	}
	if sess.PublicID == "" {
		t.Fatal("StartDecisionTest returned no public id")
	}

	items, err := r.DTScoredItems(ctx)
	if err != nil {
		t.Fatalf("DTScoredItems: %v", err)
	}
	if len(items) < DTQuestionCount {
		t.Fatalf("item bank holds %d scored items, need %d", len(items), DTQuestionCount)
	}

	for bn := 1; bn <= DTBlockCount; bn++ {
		block, err := r.DTBuildBlock(ctx, bn, items[(bn-1)*DTQuestionsPerBlock:bn*DTQuestionsPerBlock])
		if err != nil {
			t.Fatalf("DTBuildBlock(%d): %v", bn, err)
		}
		if err := r.DTRecordBlockDigits(ctx, sess.ID, block); err != nil {
			t.Fatalf("DTRecordBlockDigits(%d): %v", bn, err)
		}
		for i := range DTQuestionsPerBlock {
			pos := (bn-1)*DTQuestionsPerBlock + i + 1
			if err := r.DTSaveAnswer(ctx, sess.ID, bn, pos, 0, 9000, 70, items[pos-1]); err != nil {
				t.Fatalf("DTSaveAnswer(%d): %v", pos, err)
			}
		}
		// An expiry too, since it takes a different branch: no
		// confidence, no latency, outcome `expired`.
		if err := r.DTSaveAnswer(ctx, sess.ID, bn, (bn-1)*DTQuestionsPerBlock+1, -1, 0, 0,
			items[(bn-1)*DTQuestionsPerBlock]); err != nil {
			t.Fatalf("DTSaveAnswer(expired): %v", err)
		}
		if _, _, err := r.DTSaveRecall(ctx, sess.ID, bn, "1234", 8000); err != nil {
			t.Fatalf("DTSaveRecall(%d): %v", bn, err)
		}
		if _, _, err := r.DTBlockTally(ctx, sess.ID, bn); err != nil {
			t.Fatalf("DTBlockTally(%d): %v", bn, err)
		}
	}

	if _, err := r.DTSessionSummary(ctx, sess.ID); err != nil {
		t.Fatalf("DTSessionSummary: %v", err)
	}
	// Each accepted strategy, including the third one, because the
	// handler's allowlist and the column have to agree.
	for _, strategy := range []string{"", "encode", "defer", "unsure"} {
		if _, _, err := r.DTFinish(ctx, sess.ID, strategy); err != nil {
			t.Fatalf("DTFinish(%q): %v", strategy, err)
		}
	}
	if _, err := r.DTResultFor(ctx, sess.ID); err != nil {
		t.Fatalf("DTResultFor: %v", err)
	}
}

// The curation writes. This is the path whose Save button did nothing,
// and the bug was a type inference no static check can see.
func TestCurationWritesAgainstPostgres(t *testing.T) {
	r, ctx := dtTestRepo(t)

	sess, err := r.StartDecisionTest(ctx, DTIntake{DisplayName: "curation db test"},
		DTConditions{AudioMode: "sound", DeviceClass: "desktop"},
		true, "db-test-curation", nil,
		"v2-m6000-q25000-r20000", "items-2026-10-05", "key-1")
	if err != nil {
		t.Fatalf("StartDecisionTest: %v", err)
	}

	// A real reviewer row, because reviewed_by is a foreign key. Using a
	// made-up id passed the type check the fix was about and then failed
	// on dt_sessions_reviewed_by_fkey, which is the test being more
	// honest than I was: the production path always has a real admin.
	var reviewer int64
	if err := r.pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, name, role, status)
		VALUES ('dbtest-reviewer@example.test', 'x', 'db test reviewer', 'admin', 'active')
		ON CONFLICT (email) DO UPDATE SET name = excluded.name
		RETURNING id`).Scan(&reviewer); err != nil {
		t.Fatalf("create a reviewer: %v", err)
	}

	// Every status, with and without a reviewer, because reviewed_by is
	// nullable and the nil path takes a different branch through the
	// CASE that broke.
	for _, st := range DTReviewStatuses {
		rev := DTReview{Status: st}
		if st == DTExcludingStatus {
			rev.Reason = "instrument_fault"
		}
		if err := r.DTReviewSession(ctx, sess.PublicID, rev, &reviewer); err != nil {
			t.Fatalf("DTReviewSession(%q) with a reviewer: %v", st, err)
		}
		if err := r.DTReviewSession(ctx, sess.PublicID, rev, nil); err != nil {
			t.Fatalf("DTReviewSession(%q) with no reviewer: %v", st, err)
		}
	}
	// Clearing back to unreviewed, which is the branch where the CASE
	// evaluates to NULL and the parameter is never used.
	if err := r.DTReviewSession(ctx, sess.PublicID, DTReview{Status: ""}, &reviewer); err != nil {
		t.Fatalf("DTReviewSession(clear): %v", err)
	}

	for _, st := range DTReviewStatuses {
		rev := DTReview{BlockNo: 3, Status: st}
		if st == DTExcludingStatus {
			rev.Reason = "participant_reported"
		}
		if err := r.DTReviewBlock(ctx, sess.PublicID, rev, &reviewer); err != nil {
			t.Fatalf("DTReviewBlock(%q): %v", st, err)
		}
	}
	if err := r.DTClearBlockReview(ctx, sess.PublicID, 3, &reviewer); err != nil {
		t.Fatalf("DTClearBlockReview: %v", err)
	}
	if _, err := r.DTBlockReviews(ctx, sess.PublicID); err != nil {
		t.Fatalf("DTBlockReviews: %v", err)
	}
	if _, err := r.DTReviewSummary(ctx); err != nil {
		t.Fatalf("DTReviewSummary: %v", err)
	}
}

// The reads the admin console and the export depend on. A view that
// cannot be selected, or a scan whose columns have drifted from the
// query, fails here rather than on the page.
func TestDecisionTestReadsAgainstPostgres(t *testing.T) {
	r, ctx := dtTestRepo(t)

	runs, err := r.DTListRuns(ctx, true, "")
	if err != nil {
		t.Fatalf("DTListRuns: %v", err)
	}
	for _, status := range append([]string{"unreviewed"}, DTReviewStatuses...) {
		if _, err := r.DTListRuns(ctx, true, status); err != nil {
			t.Fatalf("DTListRuns(%q): %v", status, err)
		}
	}
	if len(runs) > 0 {
		if _, _, _, err := r.DTGetRun(ctx, runs[0].SessionKey); err != nil {
			t.Fatalf("DTGetRun: %v", err)
		}
	}
	// Both opt-ins, so each branch of the export's predicate runs.
	for _, synth := range []bool{true, false} {
		for _, excl := range []bool{true, false} {
			if _, err := r.DTExportCSV(ctx, synth, excl); err != nil {
				t.Fatalf("DTExportCSV(synthetic=%v, excluded=%v): %v", synth, excl, err)
			}
		}
	}
	if _, err := r.DTSessionsOnVersion(ctx, "v2-m6000-q25000-r20000"); err != nil {
		t.Fatalf("DTSessionsOnVersion: %v", err)
	}
	if _, err := r.DTPracticeItems(ctx); err != nil {
		t.Fatalf("DTPracticeItems: %v", err)
	}
}
