package users

import (
	"fmt"
	"testing"
)

// The item bank's shape, asserted against the real one.
//
// S1 of docs/sprint-decision-test-item-pools.md, and the first step
// because it protects every later one.
//
// **What it is guarding.** Every block of six questions carries the same
// category mix: 2 arithmetic, 1 base-rate, 1 conjunction, 2 syllogism.
// That is a real property of the instrument. It is why a block's
// accuracy can be compared with another block's at all, and it is the
// reason the load curve means anything: if block 4 had an extra
// syllogism and one fewer arithmetic, its accuracy would differ from
// block 1 for a reason that has nothing to do with load.
//
// **And nothing was checking it.** The balance is a property of
// hand-ordered `position` values 1 to 30 in a migration. Retiring one
// base-rate item and adding one syllogism would silently change block
// 3's composition, and the first sign would be a load curve that moved
// for a reason nobody could name. That has already nearly happened once:
// migration 00052 retired three base-rate items, added one conjunction
// and two syllogisms, and re-interleaved the order by hand to keep the
// mix even. It was done correctly and by eye.
//
// Read from the database rather than from a constant, because the thing
// that can drift is the data, not the code.
func TestTheItemBankIsBalanced(t *testing.T) {
	r, ctx := dtTestRepo(t)

	// The quota, per block and per test. Changing these is changing the
	// instrument, which is exactly the edit that should require somebody
	// to come here and say so.
	perBlock := map[string]int{
		"arithmetic":  2,
		"syllogism":   2,
		"base_rate":   1,
		"conjunction": 1,
	}

	total := 0
	for _, n := range perBlock {
		total += n
	}
	if total != DTQuestionsPerBlock {
		t.Fatalf("the per-block quota sums to %d, but a block is %d questions",
			total, DTQuestionsPerBlock)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT ((position - 1) / $1) + 1 AS block, family, count(*)
		  FROM dt_items
		 WHERE active AND kind = 'scored'
		 GROUP BY 1, 2`, DTQuestionsPerBlock)
	if err != nil {
		t.Fatalf("read the bank: %v", err)
	}
	defer rows.Close()

	got := map[int]map[string]int{}
	for rows.Next() {
		var block, n int
		var family string
		if err := rows.Scan(&block, &family, &n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if got[block] == nil {
			got[block] = map[string]int{}
		}
		got[block][family] = n
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read the bank: %v", err)
	}

	for block := 1; block <= DTBlockCount; block++ {
		for family, want := range perBlock {
			if got[block][family] != want {
				t.Errorf("block %d has %d %s items, want %d. "+
					"Every block must carry the same mix, or a block's accuracy "+
					"differs from another's for a reason that is not load",
					block, got[block][family], family, want)
			}
		}
		sum := 0
		for _, n := range got[block] {
			sum += n
		}
		if sum != DTQuestionsPerBlock {
			t.Errorf("block %d holds %d items, want %d", block, sum, DTQuestionsPerBlock)
		}
	}

	// Positions must be 1..30 with no gap and no duplicate. A gap would
	// silently shorten a block; a duplicate would serve one item twice
	// and leave another unserved.
	var n, lo, hi, distinct int
	if err := r.pool.QueryRow(ctx, `
		SELECT count(*), coalesce(min(position),0), coalesce(max(position),0),
		       count(DISTINCT position)
		  FROM dt_items WHERE active AND kind = 'scored'`,
	).Scan(&n, &lo, &hi, &distinct); err != nil {
		t.Fatalf("read positions: %v", err)
	}
	if n != DTQuestionCount {
		t.Errorf("the bank holds %d active scored items, want %d", n, DTQuestionCount)
	}
	if distinct != n {
		t.Errorf("%d items share %d distinct positions: one item would be served "+
			"twice and another not at all", n, distinct)
	}
	if lo != 1 || hi != DTQuestionCount {
		t.Errorf("positions run %d to %d, want 1 to %d", lo, hi, DTQuestionCount)
	}

	// Practice is its own kind and must not be short either: the warm-up
	// exists so the first real trap does not arrive at question one.
	var practice int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM dt_items WHERE active AND kind = 'practice'`,
	).Scan(&practice); err != nil {
		t.Fatalf("count practice: %v", err)
	}
	if practice == 0 {
		t.Error("no active practice items: the first scored question would be " +
			"the participant's first sight of the format")
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
	if len(seen) != DTQuestionCount {
		t.Errorf("checked %d items, expected %d", len(seen), DTQuestionCount)
	}
}
