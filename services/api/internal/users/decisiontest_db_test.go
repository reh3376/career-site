package users

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

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

// DTFinish is called twice for every run and the second call must not
// undo the first.
//
// The run screen closes the run BEFORE showing the debrief question, so
// that a participant who shuts the tab on it is recorded as having
// completed thirty answers rather than abandoned them. The debrief
// answer is a second call. That ordering is deliberate and it means the
// first call always carries an empty strategy, so a plain assignment
// would depend on which call landed last.
//
// It also means finished_at is written twice, once when the run really
// ended and once however long the participant took to read three
// options. Every duration in the data would carry that reading time.
func TestFinishIsIdempotentAcrossTheDebrief(t *testing.T) {
	r, ctx := dtTestRepo(t)

	sess, err := r.StartDecisionTest(ctx, DTIntake{DisplayName: "finish twice"},
		DTConditions{AudioMode: "sound", DeviceClass: "desktop"},
		true, "db-test-finish", nil,
		"v2-m6000-q25000-r20000", "items-2026-10-05", "key-1")
	if err != nil {
		t.Fatalf("StartDecisionTest: %v", err)
	}

	// Exactly what the run screen does: close, then answer.
	if _, _, err := r.DTFinish(ctx, sess.ID, ""); err != nil {
		t.Fatalf("DTFinish(close): %v", err)
	}
	var closedAt time.Time
	if err := r.pool.QueryRow(ctx,
		`SELECT finished_at FROM dt_sessions WHERE id=$1`, sess.ID).Scan(&closedAt); err != nil {
		t.Fatalf("read finished_at: %v", err)
	}

	if _, _, err := r.DTFinish(ctx, sess.ID, "defer"); err != nil {
		t.Fatalf("DTFinish(debrief): %v", err)
	}
	var strategy string
	var finishedAt time.Time
	if err := r.pool.QueryRow(ctx,
		`SELECT recall_strategy, finished_at FROM dt_sessions WHERE id=$1`,
		sess.ID).Scan(&strategy, &finishedAt); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if strategy != "defer" {
		t.Errorf("recall_strategy is %q after the debrief answer, want %q: "+
			"the answer the participant gave was not stored", strategy, "defer")
	}
	if !finishedAt.Equal(closedAt) {
		t.Errorf("finished_at moved from %s to %s: the run ended when it was "+
			"closed, not when the debrief was read", closedAt, finishedAt)
	}

	// And the reverse order cannot erase it. A retry, a double click or
	// a duplicated request must not turn a stored answer back into the
	// empty string, which reads as "never asked".
	if _, _, err := r.DTFinish(ctx, sess.ID, ""); err != nil {
		t.Fatalf("DTFinish(empty, again): %v", err)
	}
	if err := r.pool.QueryRow(ctx,
		`SELECT recall_strategy FROM dt_sessions WHERE id=$1`, sess.ID).Scan(&strategy); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if strategy != "defer" {
		t.Errorf("an empty strategy overwrote the stored %q", "defer")
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

// A run must stay reachable by key once there are more runs than the
// list shows.
//
// DTGetRun used to call DTListRuns, which is capped at 200, and scan the
// returned slice in Go for a matching key. With 18 runs in production
// that worked perfectly. With 201 it would have reported "no run" for
// every run outside the newest 200, on a page whose entire job is to
// show a run that exists, and nothing would have errored: the archive
// grows and is never pruned, so this was a matter of time rather than
// of chance.
//
// 201 rows inserted directly. Going through StartDecisionTest would
// test the write path again rather than the read, and would be slower
// for no extra coverage.
func TestARunStaysReachableBeyondTheListCap(t *testing.T) {
	r, ctx := dtTestRepo(t)

	var firstKey string
	for i := range 201 {
		var key string
		if err := r.pool.QueryRow(ctx, `
			INSERT INTO dt_sessions
			  (instrument_version, item_set_version, key_version, visitor_key, is_synthetic)
			VALUES ('v-cap-test', 'items-cap-test', 'key-1', $1, true)
			RETURNING public_id::text`,
			fmt.Sprintf("cap-test-%d", i)).Scan(&key); err != nil {
			t.Fatalf("insert session %d: %v", i, err)
		}
		if i == 0 {
			firstKey = key
		}
	}

	// The oldest of the 201, which is exactly the one a Go-side scan of
	// the newest 200 would miss.
	run, _, _, err := r.DTGetRun(ctx, firstKey)
	if err != nil {
		t.Fatalf("DTGetRun on the 201st-newest run: %v: the detail page "+
			"reports that a run which exists does not", err)
	}
	if run.SessionKey != firstKey {
		t.Fatalf("DTGetRun returned %q, want %q", run.SessionKey, firstKey)
	}
}

// The attempt sequence as the console reads it.
//
// The rule lives in SQL (migration 00058) and Go must not acquire a
// second copy of it, so this checks that the console's query reports
// what those functions define, including the case that is easiest to
// get wrong: a run no identity could place is not a first run.
func TestTheConsoleReadsTheAttemptSequence(t *testing.T) {
	r, ctx := dtTestRepo(t)

	cases := []struct {
		name                   string
		account, email, cookie *int
		wantNo                 int // 0 means NULL
		wantSource             string
		wantDisagree           bool
	}{
		{name: "no identity at all", wantNo: 0, wantSource: "none"},
		{name: "cookie saw nothing", cookie: ptr(0), wantNo: 1, wantSource: "cookie"},
		{name: "cookie saw two priors", cookie: ptr(2), wantNo: 3, wantSource: "cookie"},
		{name: "account outranks cookie on a tie",
			account: ptr(1), cookie: ptr(1), wantNo: 2, wantSource: "account"},
		{name: "the union wins and the sources disagree",
			account: ptr(0), cookie: ptr(3), wantNo: 4, wantSource: "cookie", wantDisagree: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var key string
			if err := r.pool.QueryRow(ctx, `
				INSERT INTO dt_sessions
				  (instrument_version, item_set_version, key_version, is_synthetic,
				   prior_by_account, prior_by_email, prior_by_cookie)
				VALUES ('v-attempt-test', 'items-attempt-test', 'key-1', true, $1, $2, $3)
				RETURNING public_id::text`,
				c.account, c.email, c.cookie).Scan(&key); err != nil {
				t.Fatalf("insert: %v", err)
			}
			run, _, _, err := r.DTGetRun(ctx, key)
			if err != nil {
				t.Fatalf("DTGetRun: %v", err)
			}
			got := 0
			if run.AttemptNo != nil {
				got = *run.AttemptNo
			}
			if got != c.wantNo {
				t.Errorf("attempt_no = %d, want %d", got, c.wantNo)
			}
			if run.AttemptSource != c.wantSource {
				t.Errorf("attempt_source = %q, want %q", run.AttemptSource, c.wantSource)
			}
			if run.AttemptSourcesDisagree != c.wantDisagree {
				t.Errorf("attempt_sources_disagree = %v, want %v",
					run.AttemptSourcesDisagree, c.wantDisagree)
			}
			// NULL must survive as NULL. Collapsing "no such identity"
			// into zero would turn an absent link into a claim that the
			// link existed and saw nothing.
			if c.cookie == nil && run.PriorByCookie != nil {
				t.Errorf("prior_by_cookie came back %d, want NULL", *run.PriorByCookie)
			}
		})
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

// The console must return the whole record, not most of it.
//
// The owner asked for all the data per test to be reachable from
// /admin/decision-test. Each field here was recorded from the first
// session and readable only through /admin/db, and a reviewer who has
// to leave the page to judge a run will sometimes judge it without
// leaving.
//
// This also executes the jsonb subscripting into dt_items.options,
// which was written as array subscripting first and failed with
// "invalid input syntax for type json" on every run page. Nothing
// static could see it: the SQL is a string literal and the column type
// lives in the database.
func TestTheConsoleReturnsTheWholeRecord(t *testing.T) {
	r, ctx := dtTestRepo(t)

	sess, err := r.StartDecisionTest(ctx,
		DTIntake{DisplayName: "whole record", Email: "whole@example.test", WantsResults: true},
		DTConditions{AudioMode: "sound", DeviceClass: "desktop", TapCheckPassed: true,
			BaselineRTMs: 300, BaselineRTSDMs: 40},
		true, "db-test-whole", nil,
		"v2-m6000-q25000-r20000", "items-2026-10-05", "key-under-test")
	if err != nil {
		t.Fatalf("StartDecisionTest: %v", err)
	}
	items, err := r.DTScoredItems(ctx)
	if err != nil {
		t.Fatalf("DTScoredItems: %v", err)
	}
	block, err := r.DTBuildBlock(ctx, 1, items[:DTQuestionsPerBlock])
	if err != nil {
		t.Fatalf("DTBuildBlock: %v", err)
	}
	if err := r.DTRecordBlockDigits(ctx, sess.ID, block); err != nil {
		t.Fatalf("DTRecordBlockDigits: %v", err)
	}
	if err := r.DTSaveAnswer(ctx, sess.ID, 1, 1, 0, 9000, 90, items[0]); err != nil {
		t.Fatalf("DTSaveAnswer: %v", err)
	}
	// Give the number back correctly, so retention is non-zero and the
	// block's counts have something to report.
	var expected string
	if err := r.pool.QueryRow(ctx,
		`SELECT expected_digits FROM dt_recalls WHERE session_id=$1 AND block_no=1`,
		sess.ID).Scan(&expected); err != nil {
		t.Fatalf("read the expected digits: %v", err)
	}
	if _, _, err := r.DTSaveRecall(ctx, sess.ID, 1, expected, 8000); err != nil {
		t.Fatalf("DTSaveRecall: %v", err)
	}

	run, blocks, answers, err := r.DTGetRun(ctx, sess.PublicID)
	if err != nil {
		t.Fatalf("DTGetRun: %v", err)
	}

	// Session provenance and intent.
	if run.KeyVersion != "key-under-test" {
		t.Errorf("key_version = %q, want the version the run was graded under", run.KeyVersion)
	}
	if !run.WantsResults {
		t.Error("wants_results is false although the participant asked for results")
	}

	// The question and the option, which is the pair a reviewer needs
	// and the thing the export deliberately withholds.
	if len(answers) == 0 {
		t.Fatal("no answers came back")
	}
	a := answers[0]
	if a.Prompt == "" {
		t.Error("the answer carries no prompt: a reviewer cannot tell a careless " +
			"answer from a misread item without seeing what was asked")
	}
	if a.ChosenText == "" {
		t.Error("the answer carries no chosen option text")
	}
	if a.CorrectText == "" {
		t.Error("the answer carries no correct option text")
	}
	if a.ChosenIndex != 0 {
		t.Errorf("chosen_index = %d, want 0", a.ChosenIndex)
	}

	// Recall detail, which the memory-failure percentage is derived from.
	if len(blocks) == 0 {
		t.Fatal("no blocks came back")
	}
	if blocks[0].DigitsHeld != len(expected) {
		t.Errorf("digits_held = %d for a perfectly recalled %d-digit number, "+
			"so the memory-failure percentage has nothing visible behind it",
			blocks[0].DigitsHeld, len(expected))
	}
	if blocks[0].RecallLatencyMs == 0 {
		t.Error("the block reports no recall latency")
	}

	// The audit trail, which existed and was unreadable.
	if err := r.DTReviewSession(ctx, sess.PublicID,
		DTReview{Status: DTExcludingStatus, Reason: "instrument_fault", Note: "a note"},
		nil); err != nil {
		t.Fatalf("DTReviewSession: %v", err)
	}
	history, err := r.DTReviewHistory(ctx, sess.PublicID)
	if err != nil {
		t.Fatalf("DTReviewHistory: %v", err)
	}
	if len(history) == 0 {
		t.Fatal("the curation history is empty after a judgement was recorded: " +
			"an exclusion that cannot be explained is worse than no exclusion")
	}
	last := history[len(history)-1]
	if last.Status != DTExcludingStatus || last.Reason != "instrument_fault" || last.Note != "a note" {
		t.Errorf("history records %+v, want the judgement that was just made", last)
	}
}
