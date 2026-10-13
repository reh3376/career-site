package users

import (
	"testing"
)

// A run whose answers include one that was neither correct nor expired
// and carries no confidence must still be readable.
//
// v_dt_answers defines confidently_wrong as
//
//	outcome NOT IN ('correct','expired') AND confidence >= threshold
//
// In three-valued logic that is NULL when the first half is true and the
// confidence is absent: TRUE AND NULL is NULL, while FALSE AND NULL is
// FALSE. So only that combination produces it, which is why most runs
// were fine. The migration's own comment shows the author considered the
// expiry case and settled it; this one slipped through.
//
// Those columns scan into plain bools, pgx refuses NULL into a bool,
// DTGetRun returned the error, and GetDecisionTestRun turned every error
// into "no such run". Two of forty-six production runs reported
// themselves missing, including the only real anonymous completed one,
// and nobody could see why because the error was discarded.
func TestARunWithAnUnscoredAnswerIsStillReadable(t *testing.T) {
	r, ctx := dtTestRepo(t)

	// Any answer will do; the outcome and confidence are both set here
	// and restored afterwards, because a development database may hold
	// only correct answers and the combination that triggers this is
	// specific.
	var key string
	var sessionID int64
	var position int
	err := r.pool.QueryRow(ctx, `
		SELECT s.public_id::text, a.session_id, a.position_overall
		  FROM dt_answers a
		  JOIN dt_sessions s ON s.id = a.session_id
		 LIMIT 1`).Scan(&key, &sessionID, &position)
	if err != nil {
		t.Skipf("no decision-test answers in this database: %v", err)
	}

	var beforeConf *int
	var beforeOutcome string
	if err := r.pool.QueryRow(ctx,
		`SELECT confidence, outcome FROM dt_answers WHERE session_id=$1 AND position_overall=$2`,
		sessionID, position).Scan(&beforeConf, &beforeOutcome); err != nil {
		t.Fatalf("read the row back: %v", err)
	}
	t.Cleanup(func() {
		_, _ = r.pool.Exec(ctx,
			`UPDATE dt_answers SET confidence=$3, outcome=$4
			  WHERE session_id=$1 AND position_overall=$2`,
			sessionID, position, beforeConf, beforeOutcome)
	})
	if _, err := r.pool.Exec(ctx,
		`UPDATE dt_answers SET confidence=NULL, outcome='other'
		  WHERE session_id=$1 AND position_overall=$2`,
		sessionID, position); err != nil {
		t.Fatalf("set up the unscored answer: %v", err)
	}

	// Guard the guard: if the view stops producing NULL here, this test
	// passes while asserting nothing.
	var nulls int
	if err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM v_dt_answers
		 WHERE session_key::text = $1 AND confidently_wrong IS NULL`, key).Scan(&nulls); err != nil {
		t.Fatalf("count nulls: %v", err)
	}
	if nulls == 0 {
		t.Fatal("the view no longer returns NULL for confidently_wrong on an " +
			"unscored answer, so this test is not exercising the bug it exists for")
	}

	run, _, answers, err := r.DTGetRun(ctx, key)
	if err != nil {
		t.Fatalf("DTGetRun on a run with an unscored answer: %v\n"+
			"This is the failure that made two production runs report themselves missing.", err)
	}
	if run == nil || len(answers) == 0 {
		t.Fatal("run or answers came back empty; the detail page would render nothing")
	}
}
