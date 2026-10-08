package users

import (
	"fmt"
	"testing"
)

// The item bank's shape, asserted against the real one.
//
// S1 of docs/sprint-decision-test-item-pools.md, rewritten at S8 when
// the bank stopped being a fixed ordered test and became four pools.
//
// Read from the database rather than from a constant, because the thing
// that drifts is the data, not the code.
func TestEveryPoolCanFillATest(t *testing.T) {
	r, ctx := dtTestRepo(t)

	// **This invariant changed on 2026-10-07 and the change is the
	// point.** It used to assert a fixed ordered bank of thirty: block 1
	// is positions 1 to 6, holding 2 arithmetic, 1 base-rate, 1
	// conjunction and 2 syllogism, and so on. That was true while every
	// participant was served the same thirty items in the same order.
	//
	// Now the bank is four pools and each run draws from them, so
	// position no longer decides composition: `dt_category_quota` does,
	// and the draw honours it on every single draw (see
	// decisiontest_draw_test.go). What the bank itself has to guarantee
	// is simply that each pool can fill a test.
	rows, err := r.pool.Query(ctx, `
		SELECT category, pool, per_test, can_vary FROM v_dt_item_pools ORDER BY category`)
	if err != nil {
		t.Fatalf("read the pools: %v", err)
	}
	defer rows.Close()

	seen := 0
	for rows.Next() {
		var category string
		var pool, perTest int
		var canVary bool
		if err := rows.Scan(&category, &pool, &perTest, &canVary); err != nil {
			t.Fatalf("scan: %v", err)
		}
		seen++
		if perTest == 0 {
			t.Errorf("category %q has no quota, so dt_category_quota does not know "+
				"about it and the draw will never select from it", category)
		}
		if pool < perTest {
			t.Errorf("category %q holds %d items for the %d a single test needs: "+
				"every draw will refuse", category, pool, perTest)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read the pools: %v", err)
	}
	if seen != 4 {
		t.Fatalf("v_dt_item_pools reports %d categories, expected 4", seen)
	}

	// The quota must add up to a block, or the draw lays out five
	// questions where six belong.
	var perBlock int
	if err := r.pool.QueryRow(ctx, `
		SELECT sum(dt_category_quota(family))::int
		  FROM (SELECT DISTINCT family FROM dt_items
		         WHERE active AND kind = 'scored') f`).Scan(&perBlock); err != nil {
		t.Fatalf("sum the quota: %v", err)
	}
	if perBlock != DTQuestionsPerBlock {
		t.Errorf("the category quotas sum to %d, but a block holds %d questions",
			perBlock, DTQuestionsPerBlock)
	}
}

// A production database must never be able to serve placeholders.
//
// The real items are synced privately and the public repository seeds
// fixtures so CI has a bank. The flag that separates them is the only
// thing standing between a volunteer and fifteen minutes of "which
// answer is the number four", so it is asserted rather than trusted.
func TestFixturesAreRefusedUnlessAllowed(t *testing.T) {
	r, ctx := dtTestRepo(t)

	var fixtures int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM dt_items WHERE active AND kind='scored' AND is_fixture`,
	).Scan(&fixtures); err != nil {
		t.Fatalf("count fixtures: %v", err)
	}
	if fixtures == 0 {
		t.Skip("no fixtures in this database, so there is nothing to refuse")
	}

	var realItems int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM dt_items WHERE active AND kind='scored' AND NOT is_fixture`,
	).Scan(&realItems); err != nil {
		t.Fatalf("count real items: %v", err)
	}
	if realItems > 0 {
		t.Skip("this database holds real items too, so a refusal would not prove anything")
	}

	sess := newDrawSession(t, r, ctx)
	if err := DTDrawItems(ctx, r.pool, sess, nil, false); err == nil {
		t.Error("a draw succeeded against a fixture-only bank with fixtures " +
			"disallowed: production would serve placeholders to a participant")
	}
	// And allowed, it must work, or CI has no bank at all.
	if err := DTDrawItems(ctx, r.pool, sess, nil, true); err != nil {
		t.Errorf("a draw failed against fixtures with them allowed: %v", err)
	}
}

// Every scored item must be answerable and must have a designed wrong
// answer, or it measures nothing.
//
// A lure index equal to the correct index would make the intended wrong
// answer the right one, and `is_lure` would then be true of a correct
// answer: `v_dt_items.lure_pct` would read as the item working
// perfectly while recording the opposite.
func TestEveryScoredItemHasADistinctLure(t *testing.T) {
	r, ctx := dtTestRepo(t)

	rows, err := r.pool.Query(ctx, `
		SELECT code, family, correct_index, lure_index, jsonb_array_length(options)
		  FROM dt_items WHERE active AND kind = 'scored' ORDER BY position`)
	if err != nil {
		t.Fatalf("read the bank: %v", err)
	}
	defer rows.Close()

	seen := map[string]bool{}
	for rows.Next() {
		var code, family string
		var correct, lure, opts int
		if err := rows.Scan(&code, &family, &correct, &lure, &opts); err != nil {
			t.Fatalf("scan: %v", err)
		}
		where := fmt.Sprintf("item %s (%s)", code, family)
		if seen[code] {
			t.Errorf("%s appears twice in the active bank", where)
		}
		seen[code] = true

		if opts < 2 {
			t.Errorf("%s offers %d options", where, opts)
		}
		if correct < 0 || correct >= opts {
			t.Errorf("%s has correct_index %d against %d options", where, correct, opts)
		}
		if lure < 0 || lure >= opts {
			t.Errorf("%s has lure_index %d against %d options", where, lure, opts)
		}
		if correct == lure {
			t.Errorf("%s has the lure on the correct answer, so taking the lure "+
				"and being right are the same event and lure_pct is meaningless",
				where)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read the bank: %v", err)
	}
	// At least a full test's worth, and in practice far more now that
	// the bank is pools. A hard number here would have to be edited
	// every time an item is written, which is how a guard becomes a
	// nuisance and then gets deleted.
	if len(seen) < DTQuestionCount {
		t.Errorf("checked %d active scored items, which cannot fill a %d question test",
			len(seen), DTQuestionCount)
	}
}
