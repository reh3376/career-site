package users

import (
	"context"
	"testing"
)

// The draw, against a real database.
//
// S3 of docs/sprint-decision-test-item-pools.md. Three properties have
// to hold on every single draw, because one bad draw is one
// participant's fifteen minutes spent on an instrument that was not the
// instrument:
//
//   - the composition is exactly the quota, in every block;
//   - no item appears twice in one test;
//   - a category that cannot be filled refuses, rather than quietly
//     producing a five-question block.
func TestADrawIsAlwaysBalancedAndDistinct(t *testing.T) {
	r, ctx := dtTestRepo(t)

	// Repeated, because this is a randomised selection and a single draw
	// proves very little about one. Twenty-five is enough to catch an
	// off-by-one in the slot layout or a category that is short only
	// sometimes.
	for attempt := range 25 {
		sess := newDrawSession(t, r, ctx)
		if err := DTDrawItems(ctx, r.pool, sess, nil, true); err != nil {
			t.Fatalf("draw %d: %v", attempt, err)
		}

		rows, err := r.pool.Query(ctx, `
			SELECT si.position_overall, i.family, i.id
			  FROM dt_session_items si
			  JOIN dt_items i ON i.id = si.item_id
			 WHERE si.session_id = $1
			 ORDER BY si.position_overall`, sess)
		if err != nil {
			t.Fatalf("read the draw: %v", err)
		}
		perBlock := map[int]map[string]int{}
		seen := map[int64]int{}
		positions := map[int]bool{}
		n := 0
		for rows.Next() {
			var pos int
			var family string
			var id int64
			if err := rows.Scan(&pos, &family, &id); err != nil {
				t.Fatalf("scan: %v", err)
			}
			n++
			positions[pos] = true
			seen[id]++
			block := (pos-1)/DTQuestionsPerBlock + 1
			if perBlock[block] == nil {
				perBlock[block] = map[string]int{}
			}
			perBlock[block][family]++
		}
		rows.Close()

		if n != DTQuestionCount {
			t.Fatalf("draw %d produced %d questions, want %d", attempt, n, DTQuestionCount)
		}
		for pos := 1; pos <= DTQuestionCount; pos++ {
			if !positions[pos] {
				t.Errorf("draw %d has no question at position %d", attempt, pos)
			}
		}
		for id, times := range seen {
			if times > 1 {
				t.Errorf("draw %d used item %d %d times: an item must appear at "+
					"most once per test", attempt, id, times)
			}
		}
		// The composition, read from the same function the draw and the
		// view use, so the test cannot drift from the rule.
		for block := 1; block <= DTBlockCount; block++ {
			for _, family := range []string{"arithmetic", "base_rate", "conjunction", "syllogism"} {
				var want int
				if err := r.pool.QueryRow(ctx,
					`SELECT dt_category_quota($1)`, family).Scan(&want); err != nil {
					t.Fatalf("quota: %v", err)
				}
				if perBlock[block][family] != want {
					t.Errorf("draw %d block %d has %d %s, want %d",
						attempt, block, perBlock[block][family], family, want)
				}
			}
		}
	}
}

// The database refuses a repeat even if the selection ever stopped
// preventing one. "Used once per test" is a property of the data.
func TestTheDatabaseRefusesARepeatedItem(t *testing.T) {
	r, ctx := dtTestRepo(t)
	sess := newDrawSession(t, r, ctx)

	var item int64
	if err := r.pool.QueryRow(ctx,
		`SELECT id FROM dt_items WHERE active AND kind='scored' LIMIT 1`).Scan(&item); err != nil {
		t.Fatalf("pick an item: %v", err)
	}
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO dt_session_items (session_id, position_overall, item_id) VALUES ($1,1,$2)`,
		sess, item); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO dt_session_items (session_id, position_overall, item_id) VALUES ($1,2,$2)`,
		sess, item); err == nil {
		t.Error("the same item was accepted twice in one test; the unique " +
			"constraint on (session_id, item_id) is the no-repeat rule and it did not hold")
	}
}

// A category that cannot be filled must refuse, not improvise.
//
// This is the failure that would cost real data: a short block served to
// a participant is fifteen minutes spent on something that is not the
// instrument, and it would look entirely normal on screen.
func TestADrawRefusesRatherThanServeAShortBlock(t *testing.T) {
	r, ctx := dtTestRepo(t)
	sess := newDrawSession(t, r, ctx)

	// Exclude every base-rate item, as a repeat participant who had seen
	// them all would.
	rows, err := r.pool.Query(ctx,
		`SELECT id FROM dt_items WHERE active AND kind='scored' AND family='base_rate'`)
	if err != nil {
		t.Fatalf("read base_rate: %v", err)
	}
	var exclude []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		exclude = append(exclude, id)
	}
	rows.Close()
	if len(exclude) == 0 {
		t.Fatal("no base_rate items to exclude")
	}

	err = DTDrawItems(ctx, r.pool, sess, exclude, true)
	if err == nil {
		t.Fatal("the draw succeeded with every base_rate item excluded: it must " +
			"refuse rather than serve a block short of its quota")
	}

	// And it must leave nothing behind, or the next attempt would hit
	// the unique constraint on a half-written draw.
	var left int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM dt_session_items WHERE session_id=$1`, sess).Scan(&left); err != nil {
		t.Fatalf("count: %v", err)
	}
	if left != 0 {
		t.Errorf("a refused draw left %d rows behind", left)
	}
}

// Two sittings by the same visitor must not share questions, which is
// the whole reason any of this exists.
func TestARepeatSittingDrawsWhatWasNotSeen(t *testing.T) {
	r, ctx := dtTestRepo(t)

	var pools []DTPoolStatus
	pools, err := r.DTItemPools(ctx)
	if err != nil {
		t.Fatalf("DTItemPools: %v", err)
	}
	for _, p := range pools {
		if !p.CanVary {
			t.Skipf("category %q has a pool of %d for a per-test need of %d, so a "+
				"second sitting cannot avoid the first. This is the honest state of "+
				"the bank today and the reason S7 exists; the test will run once "+
				"items are added.", p.Category, p.Pool, p.PerTest)
		}
	}

	// Not synthetic: this models a real participant coming back, and
	// DTSeenItems deliberately ignores agent-driven runs.
	const visitor = "draw-test-repeat-visitor"
	first := newRealDrawSessionFor(t, r, ctx, visitor)
	if err := DTDrawItems(ctx, r.pool, first, nil, true); err != nil {
		t.Fatalf("first draw: %v", err)
	}
	seen, err := r.DTSeenItems(ctx, visitor, nil)
	if err != nil {
		t.Fatalf("DTSeenItems: %v", err)
	}
	if len(seen) != DTQuestionCount {
		t.Fatalf("the first sitting is remembered as %d items, want %d",
			len(seen), DTQuestionCount)
	}

	second := newRealDrawSessionFor(t, r, ctx, visitor)
	if err := DTDrawItems(ctx, r.pool, second, seen, true); err != nil {
		t.Fatalf("second draw: %v", err)
	}
	var shared int
	if err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM dt_session_items a
		  JOIN dt_session_items b ON b.item_id = a.item_id
		 WHERE a.session_id = $1 AND b.session_id = $2`, first, second).Scan(&shared); err != nil {
		t.Fatalf("count shared: %v", err)
	}
	if shared != 0 {
		t.Errorf("the second sitting repeated %d questions from the first", shared)
	}
}

func newDrawSession(t *testing.T, r *Repo, ctx context.Context) int64 {
	t.Helper()
	return newDrawSessionFor(t, r, ctx, "draw-test-visitor")
}

func newDrawSessionFor(t *testing.T, r *Repo, ctx context.Context, visitor string) int64 {
	t.Helper()
	return insertDrawSession(t, r, ctx, visitor, true)
}

// A run that counts as a person having seen the items. Only the repeat
// test needs one, because DTSeenItems ignores synthetic runs on purpose.
func newRealDrawSessionFor(t *testing.T, r *Repo, ctx context.Context, visitor string) int64 {
	t.Helper()
	return insertDrawSession(t, r, ctx, visitor, false)
}

func insertDrawSession(t *testing.T, r *Repo, ctx context.Context, visitor string, synthetic bool) int64 {
	t.Helper()
	var id int64
	if err := r.pool.QueryRow(ctx, `
		INSERT INTO dt_sessions
		  (instrument_version, item_set_version, key_version, visitor_key, is_synthetic)
		VALUES ('v-draw-test', 'items-draw-test', 'key-1', $1, $2)
		RETURNING id`, visitor, synthetic).Scan(&id); err != nil {
		t.Fatalf("create a session: %v", err)
	}
	return id
}

// Two independent participants should not sit the same test.
//
// This is the promise the whole exercise exists to keep, and it is
// worth measuring rather than assuming: the draw can be perfectly
// balanced and perfectly distinct within a test while still handing
// everybody the same thirty questions, which is exactly what happens
// when a pool is the same size as its quota.
//
// Skips, loudly, while the bank cannot support it.
func TestTwoParticipantsGetDifferentTests(t *testing.T) {
	r, ctx := dtTestRepo(t)

	pools, err := r.DTItemPools(ctx)
	if err != nil {
		t.Fatalf("DTItemPools: %v", err)
	}
	for _, p := range pools {
		if !p.CanVary {
			t.Skipf("category %q holds %d real items for a per-test need of %d, "+
				"so every participant sees the same questions however the draw "+
				"is written", p.Category, p.Pool, p.PerTest)
		}
	}

	// Several pairs, because two draws could coincide closely by chance
	// and a single comparison would not notice a selection that is
	// barely random.
	worst := 0
	total := 0
	const pairs = 10
	for i := range pairs {
		a := newDrawSession(t, r, ctx)
		b := newDrawSession(t, r, ctx)
		if err := DTDrawItems(ctx, r.pool, a, nil, true); err != nil {
			t.Fatalf("draw a: %v", err)
		}
		if err := DTDrawItems(ctx, r.pool, b, nil, true); err != nil {
			t.Fatalf("draw b: %v", err)
		}
		var shared int
		if err := r.pool.QueryRow(ctx, `
			SELECT count(*) FROM dt_session_items x
			  JOIN dt_session_items y ON y.item_id = x.item_id
			 WHERE x.session_id = $1 AND y.session_id = $2`, a, b).Scan(&shared); err != nil {
			t.Fatalf("count shared: %v", err)
		}
		total += shared
		if shared > worst {
			worst = shared
		}
		if shared == DTQuestionCount {
			t.Fatalf("pair %d sat an identical test: the draw is not varying at all", i)
		}
	}
	mean := float64(total) / float64(pairs)
	t.Logf("across %d pairs: %.1f of %d questions shared on average, worst %d",
		pairs, mean, DTQuestionCount, worst)

	// With each pool at three times its quota, a third of each category
	// is drawn, so roughly a third of questions coinciding is expected.
	// Well over half would mean the selection is barely random.
	if mean > float64(DTQuestionCount)*0.6 {
		t.Errorf("two participants share %.1f of %d questions on average, which is "+
			"too close to the same test", mean, DTQuestionCount)
	}
}

// A run that stopped early must still show every question it was given.
//
// The difference between "stopped at question six" and "was asked six
// questions" decides whether a short run is usable, and the answer
// table cannot tell them apart: it has six rows either way.
func TestTheDrawIsVisibleForAnAbandonedRun(t *testing.T) {
	r, ctx := dtTestRepo(t)

	sess, err := r.StartDecisionTest(ctx, DTIntake{DisplayName: "stopped early"},
		DTConditions{AudioMode: "sound", DeviceClass: "phone"},
		true, "db-test-abandoned", nil,
		"v-draw-test", "pool-test", "key-1", true)
	if err != nil {
		t.Fatalf("StartDecisionTest: %v", err)
	}

	// Two questions answered, then nothing: the shape of a run somebody
	// walked away from.
	block, err := r.DTDrawnBlock(ctx, sess.ID, 1)
	if err != nil {
		t.Fatalf("DTDrawnBlock: %v", err)
	}
	for i := range 2 {
		if err := r.DTSaveAnswer(ctx, sess.ID, 1, i+1, 0, 5000, 50, block[i]); err != nil {
			t.Fatalf("DTSaveAnswer: %v", err)
		}
	}

	drawn, err := r.DTDrawnItems(ctx, sess.PublicID)
	if err != nil {
		t.Fatalf("DTDrawnItems: %v", err)
	}
	if len(drawn) != DTQuestionCount {
		t.Fatalf("the run shows %d drawn questions, want %d: an abandoned run "+
			"must still account for everything it was given", len(drawn), DTQuestionCount)
	}
	reached := 0
	for _, d := range drawn {
		if d.Answered {
			reached++
		}
		if d.Code == "" || d.Family == "" {
			t.Errorf("position %d has no code or category", d.Position)
		}
	}
	if reached != 2 {
		t.Errorf("%d questions marked reached, want 2", reached)
	}
	// Ordered, or the list cannot be read against the answer table.
	for i, d := range drawn {
		if d.Position != i+1 {
			t.Fatalf("drawn item %d is at position %d", i, d.Position)
		}
	}
}
