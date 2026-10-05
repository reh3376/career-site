package users

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// Curation: the owner's judgement about whether a run, or one block of
// a run, is fit to use (roadmap M9).
//
// The vocabulary is here as well as in a CHECK constraint, and both
// earn their place. The constraint makes it true of the table whoever
// writes to it; this gives a caller a useful error instead of a
// constraint violation, and is the list the admin form renders from, so
// adding a status is one edit rather than three that can disagree.
//
// **Exactly one value excludes.** The owner settled that on 2026-10-05:
// `incomplete` describes a run and does not block it, because whether
// an interrupted run is usable is a judgement he makes per run from the
// data rather than a rule. So `do_not_use` is the only word that
// changes what the analysis sees, which is what makes "is this usable"
// answerable in one place. That place is `v_dt_answers.usable`, not
// here: Go must not acquire a second definition of it.

// DTReviewStatuses is the vocabulary, in the order a form should offer
// it. The empty string is not here: it means not yet reviewed, and is a
// state rather than a choice.
var DTReviewStatuses = []string{"good", "incomplete", "hold", "do_not_use"}

// DTReviewReasons says why something was excluded.
//
// instrument_fault and participant_reported are the split that earns
// this field. They point at opposite responses: the first is a bug to
// go and repair, and a count of them is a measure of the instrument's
// health; the second is nothing to fix and simply costs a data point.
// Without the distinction, "we lost nine runs" cannot be acted on.
var DTReviewReasons = []string{"instrument_fault", "participant_reported", "duplicate", "other"}

// DTExcludingStatus is the one value that removes data from the views
// that make claims about people. Named rather than written as a literal
// in several files.
const DTExcludingStatus = "do_not_use"

// DTReview is one judgement, about a session or about one of its
// blocks.
type DTReview struct {
	// 0 for the session as a whole, 1 to 5 for a block.
	BlockNo int
	Status  string
	Reason  string
	Note    string
}

// Validate normalises and checks a judgement.
//
// An unknown status or reason is refused rather than stored. A status
// nobody can interpret still lands in a count, and the count is what
// decides whether a finding is quotable.
func (r *DTReview) Validate() error {
	r.Status = strings.TrimSpace(r.Status)
	r.Reason = strings.TrimSpace(r.Reason)
	r.Note = strings.TrimSpace(r.Note)

	if r.BlockNo != 0 && (r.BlockNo < 1 || r.BlockNo > DTBlockCount) {
		return fmt.Errorf("block %d is not part of this test", r.BlockNo)
	}
	// Clearing a session's review is allowed: a reviewer who marked the
	// wrong row needs a way back to "not yet reviewed". A block has no
	// such state, because no row means unreviewed, so clearing one is a
	// delete and is handled by DTClearBlockReview.
	if r.Status == "" {
		if r.BlockNo != 0 {
			return fmt.Errorf("a block review needs a status")
		}
		if r.Reason != "" || r.Note != "" {
			return fmt.Errorf("clearing a review cannot carry a reason or a note")
		}
		return nil
	}
	if !slices.Contains(DTReviewStatuses, r.Status) {
		return fmt.Errorf("%q is not a review status", r.Status)
	}
	if r.Reason != "" && !slices.Contains(DTReviewReasons, r.Reason) {
		return fmt.Errorf("%q is not a review reason", r.Reason)
	}
	// A reason only means anything against an exclusion. Attaching one
	// to "good" would read as a caveat on data that has none.
	if r.Reason != "" && r.Status != DTExcludingStatus {
		return fmt.Errorf("a reason applies only to %s", DTExcludingStatus)
	}
	return nil
}

// DTReviewSession records a judgement about a whole run.
//
// The verdict is written to the session row so every query can filter
// on it cheaply, and appended to dt_review_events so the history
// survives. Both in one transaction: a verdict with no history, or a
// history with no verdict, would each be worse than neither.
func (r *Repo) DTReviewSession(ctx context.Context, sessionKey string, rev DTReview, reviewerID *int64) error {
	if err := rev.Validate(); err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("decision test: review: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var sessionID int64
	if err := tx.QueryRow(ctx,
		`UPDATE dt_sessions
		    SET review_status=$2, review_reason=$3, review_note=$4,
		        reviewed_at = CASE WHEN $2 = '' THEN NULL ELSE now() END,
		        -- $5 is cast explicitly. Postgres infers a parameter's
		        -- type from the context it appears in, and inside a CASE
		        -- whose other branch is an untyped NULL there is nothing
		        -- to infer from but the sibling comparison against a
		        -- text literal, so it decided $5 was text and refused
		        -- the assignment to a bigint column:
		        --   column "reviewed_by" is of type bigint but
		        --   expression is of type text (SQLSTATE 42804)
		        reviewed_by = CASE WHEN $2 = '' THEN NULL ELSE $5::bigint END
		  WHERE public_id::text = $1
		  RETURNING id`,
		sessionKey, rev.Status, rev.Reason, rev.Note, reviewerID,
	).Scan(&sessionID); err != nil {
		return fmt.Errorf("decision test: review session: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO dt_review_events (session_id, block_no, status, reason, note, reviewed_by)
		 VALUES ($1, NULL, $2, $3, $4, $5)`,
		sessionID, rev.Status, rev.Reason, rev.Note, reviewerID,
	); err != nil {
		return fmt.Errorf("decision test: review history: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("decision test: review commit: %w", err)
	}
	return nil
}

// DTReviewBlock records a judgement about one block.
//
// Upsert on (session, block): re-reviewing replaces rather than stacks,
// the same convention ReviewDecision uses, while the history keeps both.
func (r *Repo) DTReviewBlock(ctx context.Context, sessionKey string, rev DTReview, reviewerID *int64) error {
	if rev.BlockNo == 0 {
		return fmt.Errorf("a block review needs a block number")
	}
	if err := rev.Validate(); err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("decision test: block review: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var sessionID int64
	if err := tx.QueryRow(ctx,
		`SELECT id FROM dt_sessions WHERE public_id::text = $1`, sessionKey).Scan(&sessionID); err != nil {
		return fmt.Errorf("decision test: block review lookup: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO dt_block_reviews (session_id, block_no, status, reason, note, reviewed_by)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (session_id, block_no) DO UPDATE
		   SET status=$3, reason=$4, note=$5, reviewed_by=$6, reviewed_at=now()`,
		sessionID, rev.BlockNo, rev.Status, rev.Reason, rev.Note, reviewerID,
	); err != nil {
		return fmt.Errorf("decision test: block review: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO dt_review_events (session_id, block_no, status, reason, note, reviewed_by)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		sessionID, rev.BlockNo, rev.Status, rev.Reason, rev.Note, reviewerID,
	); err != nil {
		return fmt.Errorf("decision test: block review history: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("decision test: block review commit: %w", err)
	}
	return nil
}

// DTClearBlockReview removes a block's judgement, returning it to
// unreviewed. The history keeps the decision that was cleared, and the
// clearing itself is recorded: an exclusion that quietly disappears is
// the thing somebody would most want explained.
func (r *Repo) DTClearBlockReview(ctx context.Context, sessionKey string, blockNo int, reviewerID *int64) error {
	if blockNo < 1 || blockNo > DTBlockCount {
		return fmt.Errorf("block %d is not part of this test", blockNo)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("decision test: clear block review: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var sessionID int64
	if err := tx.QueryRow(ctx,
		`SELECT id FROM dt_sessions WHERE public_id::text = $1`, sessionKey).Scan(&sessionID); err != nil {
		return fmt.Errorf("decision test: clear block review lookup: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM dt_block_reviews WHERE session_id=$1 AND block_no=$2`,
		sessionID, blockNo); err != nil {
		return fmt.Errorf("decision test: clear block review: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO dt_review_events (session_id, block_no, status, note, reviewed_by)
		 VALUES ($1,$2,'cleared','',$3)`,
		sessionID, blockNo, reviewerID); err != nil {
		return fmt.Errorf("decision test: clear block review history: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("decision test: clear block review commit: %w", err)
	}
	return nil
}

// DTBlockReview is one block's judgement as read back.
type DTBlockReview struct {
	BlockNo int
	Status  string
	Reason  string
	Note    string
}

// DTBlockReviews reads every block judgement for a run, keyed by block.
func (r *Repo) DTBlockReviews(ctx context.Context, sessionKey string) (map[int]DTBlockReview, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT br.block_no, br.status, br.reason, br.note
		  FROM dt_block_reviews br
		  JOIN dt_sessions s ON s.id = br.session_id
		 WHERE s.public_id::text = $1
		 ORDER BY br.block_no`, sessionKey)
	if err != nil {
		return nil, fmt.Errorf("decision test: block reviews: %w", err)
	}
	defer rows.Close()
	out := map[int]DTBlockReview{}
	for rows.Next() {
		var b DTBlockReview
		if err := rows.Scan(&b.BlockNo, &b.Status, &b.Reason, &b.Note); err != nil {
			return nil, fmt.Errorf("decision test: scan block review: %w", err)
		}
		out[b.BlockNo] = b
	}
	return out, rows.Err()
}

// DTReviewCounts is the queue: how much is left to look at.
type DTReviewCounts struct {
	Total      int
	Unreviewed int
	Good       int
	Incomplete int
	Hold       int
	DoNotUse   int
}

// DTReviewSummary counts real runs by review status.
//
// Synthetic runs are excluded: they are not awaiting anybody's
// judgement and counting them would make the queue look like work that
// does not exist.
func (r *Repo) DTReviewSummary(ctx context.Context) (DTReviewCounts, error) {
	var c DTReviewCounts
	err := r.pool.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE review_status = ''),
		       count(*) FILTER (WHERE review_status = 'good'),
		       count(*) FILTER (WHERE review_status = 'incomplete'),
		       count(*) FILTER (WHERE review_status = 'hold'),
		       count(*) FILTER (WHERE review_status = 'do_not_use')
		  FROM dt_sessions WHERE NOT is_synthetic`,
	).Scan(&c.Total, &c.Unreviewed, &c.Good, &c.Incomplete, &c.Hold, &c.DoNotUse)
	if err != nil {
		return c, fmt.Errorf("decision test: review summary: %w", err)
	}
	return c, nil
}
