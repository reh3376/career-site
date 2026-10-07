package users

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Drawing a run's thirty questions.
//
// S3 of docs/sprint-decision-test-item-pools.md. Every participant used
// to be served the same thirty items in the same order, so a repeat
// sitting measured item recall: a third sitting on 2026-10-07 scored
// 30/30 at less than half the answer time of a first-time participant,
// on 27 items it had already answered twice.
//
// The rule, as the owner set it: random within a category, each item at
// most once per test, and the exclusion resets after each test rather
// than depleting the pools.
//
// **What is held fixed is the composition.** Every block keeps the mix
// it has today, two arithmetic, one base-rate, one conjunction and two
// syllogism, in that order within the block. That mix is why one block's
// accuracy can be compared with another's at all: a block with an extra
// syllogism would differ from block 1 for a reason that is not load. The
// items vary; the shape does not.

// dtQuerier is whatever can run a query: the pool, or a transaction.
//
// The draw has to be able to join the transaction that creates the
// session, so that a session can never exist without its items. Taking
// an interface rather than a *pgxpool.Pool is what makes that possible.
type dtQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// dtSlotFamilies is the order of categories within every block.
//
// Taken from the bank's current hand-ordered layout rather than
// invented, so that a run drawn by this code has the same within-block
// structure as every run taken before it. Changing this order changes
// the instrument.
var dtSlotFamilies = []string{
	"arithmetic", "arithmetic", "base_rate", "conjunction", "syllogism", "syllogism",
}

// DTDrawItems picks a run's thirty questions and records them.
//
// `exclude` is item ids this participant has already been served in an
// earlier sitting. Empty for a first sitting. Within one draw, nothing
// repeats regardless: that is enforced by the unique constraint on
// (session_id, item_id) as well as by the selection, because "used once
// per test" is a property of the data and not a convention of the code
// that happens to write it.
//
// It refuses rather than improvises. A category that cannot be filled
// returns an error and no rows, so a short or unbalanced block can never
// reach a participant. That is the one failure here that would cost real
// data rather than merely annoying somebody.
func DTDrawItems(ctx context.Context, q dtQuerier, sessionID int64, exclude []int64) error {
	if len(dtSlotFamilies) != DTQuestionsPerBlock {
		return fmt.Errorf("decision test: %d slots defined for a %d question block",
			len(dtSlotFamilies), DTQuestionsPerBlock)
	}

	// How many of each category the whole test needs, from the slot
	// layout, so the two cannot disagree.
	need := map[string]int{}
	for _, family := range dtSlotFamilies {
		need[family] += DTBlockCount
	}

	// Draw each category's share in one go. ORDER BY random() is the
	// randomness: there is no seed threaded through Go and no second
	// account of what was chosen, because the rows written below are the
	// record.
	drawn := map[string][]int64{}
	for family, n := range need {
		rows, err := q.Query(ctx, `
			SELECT id FROM dt_items
			 WHERE active AND kind = 'scored' AND family = $1
			   -- coalesce, because a nil slice arrives as NULL and
			   -- "id = ANY(NULL)" is NULL rather than false. Without it
			   -- the NOT is NULL for every row, so a first sitting,
			   -- which excludes nothing, draws nothing at all.
			   AND NOT (id = ANY(coalesce($2::bigint[], '{}'::bigint[])))
			 ORDER BY random()
			 LIMIT $3`, family, exclude, n)
		if err != nil {
			return fmt.Errorf("decision test: draw %s: %w", family, err)
		}
		var ids []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return fmt.Errorf("decision test: draw %s: %w", family, err)
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("decision test: draw %s: %w", family, err)
		}
		if len(ids) < n {
			// Said with the numbers in it, because the fix is to write
			// more items and the message should say how many.
			return fmt.Errorf(
				"decision test: category %q can supply %d of the %d questions a test needs"+
					" (%d excluded as already seen): refusing to serve a short block",
				family, len(ids), n, len(exclude))
		}
		drawn[family] = ids
	}

	// Lay them out: block by block, slot by slot, taking the next unused
	// item of that slot's category.
	taken := map[string]int{}
	positions := make([]int, 0, DTQuestionCount)
	items := make([]int64, 0, DTQuestionCount)
	for block := range DTBlockCount {
		for slot, family := range dtSlotFamilies {
			positions = append(positions, block*DTQuestionsPerBlock+slot+1)
			items = append(items, drawn[family][taken[family]])
			taken[family]++
		}
	}

	// One statement. unnest keeps it to a single round trip and means
	// the thirty rows land or none of them do, which matters because a
	// half-written draw is a run that cannot be served.
	if _, err := q.Exec(ctx, `
		INSERT INTO dt_session_items (session_id, position_overall, item_id)
		SELECT $1, p, i FROM unnest($2::int[], $3::bigint[]) AS t(p, i)`,
		sessionID, positions, items); err != nil {
		return fmt.Errorf("decision test: record the draw: %w", err)
	}
	return nil
}

// DTDrawnBlock reads back the questions one block was drawn.
//
// Replaces slicing the bank's fixed order. The draw is the record, so
// this reads it rather than recomputing anything.
func (r *Repo) DTDrawnBlock(ctx context.Context, sessionID, blockNo int64) ([]DTItem, error) {
	first := (blockNo-1)*DTQuestionsPerBlock + 1
	last := blockNo * DTQuestionsPerBlock
	rows, err := r.pool.Query(ctx, `
		SELECT i.id, i.code, i.version, i.family, i.kind, i.prompt, i.reminder,
		       i.options, i.correct_index, i.lure_index
		  FROM dt_session_items si
		  JOIN dt_items i ON i.id = si.item_id
		 WHERE si.session_id = $1
		   AND si.position_overall BETWEEN $2 AND $3
		 ORDER BY si.position_overall`, sessionID, first, last)
	if err != nil {
		return nil, fmt.Errorf("decision test: drawn block: %w", err)
	}
	defer rows.Close()
	var out []DTItem
	for rows.Next() {
		var it DTItem
		var raw []byte
		if err := rows.Scan(&it.ID, &it.Code, &it.Version, &it.Family, &it.Kind,
			&it.Prompt, &it.Reminder, &raw, &it.CorrectIndex, &it.LureIndex); err != nil {
			return nil, fmt.Errorf("decision test: scan drawn item: %w", err)
		}
		if err := json.Unmarshal(raw, &it.Options); err != nil {
			return nil, fmt.Errorf("decision test: item %s options: %w", it.Code, err)
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) != DTQuestionsPerBlock {
		// A short block means the draw is incomplete, which should be
		// impossible: the insert is one statement. Refuse rather than
		// serve five questions and record it as a block of six.
		return nil, fmt.Errorf(
			"decision test: session %d block %d has %d of %d drawn questions",
			sessionID, blockNo, len(out), DTQuestionsPerBlock)
	}
	return out, nil
}

// DTSeenItems lists every item a participant has already been served,
// across all of their earlier sittings.
//
// The exclusion set for a repeat sitting (D3a). Identity is whatever the
// session already records: the visitor key for anyone, the participant
// row where one exists. A participant the system cannot recognise gets
// an empty set, which is the honest answer rather than a guess.
//
// **Synthetic runs do not count as seen**, and that is a judgement call
// worth stating. An agent-driven run is not a person reading a question,
// and agent runs are frequent during development: letting them narrow a
// real participant's pool would mean a volunteer's draw depends on how
// much testing happened to run from a browser that shared their cookie.
// The cost is that somebody who personally drove a synthetic run is not
// protected from seeing those items again, which is a smaller and much
// rarer problem.
func (r *Repo) DTSeenItems(ctx context.Context, visitorKey string, participantID *int64) ([]int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT si.item_id
		  FROM dt_session_items si
		  JOIN dt_sessions s ON s.id = si.session_id
		 WHERE NOT s.is_synthetic
		   AND ( ($1 <> '' AND s.visitor_key = $1)
		      OR ($2::bigint IS NOT NULL AND s.participant_id = $2) )`,
		visitorKey, participantID)
	if err != nil {
		return nil, fmt.Errorf("decision test: seen items: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("decision test: scan seen item: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// DTPoolStatus is one category's room to vary.
type DTPoolStatus struct {
	Category string
	Pool     int
	PerTest  int
	Spare    int
	CanVary  bool
}

// DTItemPools reads v_dt_item_pools.
//
// Exists because the mechanism can look like it is working when it
// cannot. While a pool is exactly the size of its own quota, "draw ten
// at random from ten" returns the same ten every time, every participant
// is served the same questions, and nothing on any screen looks wrong.
func (r *Repo) DTItemPools(ctx context.Context) ([]DTPoolStatus, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT category, pool, per_test, spare, can_vary
		  FROM v_dt_item_pools ORDER BY category`)
	if err != nil {
		return nil, fmt.Errorf("decision test: item pools: %w", err)
	}
	defer rows.Close()
	var out []DTPoolStatus
	for rows.Next() {
		var p DTPoolStatus
		if err := rows.Scan(&p.Category, &p.Pool, &p.PerTest, &p.Spare, &p.CanVary); err != nil {
			return nil, fmt.Errorf("decision test: scan pool: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
