package users

import (
	"context"
	"fmt"
)

// What a participant is told about their own run.
//
// The shape is fixed by two rules that pull against each other.
//
// They are owed something real. Fifteen minutes of deliberate effort,
// given to somebody else's research, earns more than "thanks". The
// accuracy-against-confidence gap is the most interesting true thing
// that can be said about one person's run, and it is the thing they
// will remember.
//
// And no correct answer is ever disclosed, to anyone (FSD 2.4). The
// item set is standardized, so a leaked key contaminates the instrument
// permanently and only has to escape once. A participant asking "which
// ones did I get wrong" is asking for the one thing that cannot be
// given, which is why the email says so rather than leaving them to
// wonder whether it was an oversight.
//
// So: an overall score, the gap, and the mechanism. Never an item.

// DTResult is one participant's figures.
type DTResult struct {
	Name  string
	Email string

	Correct int
	Total   int

	// Blocks 1 and 2, before the transformation blocks.
	EarlyAccuracy   int
	EarlyConfidence int
	// Block 4, the hardest.
	HardAccuracy   int
	HardConfidence int

	// Blocks 1 and 5: same difficulty, twelve minutes apart. Together
	// they say whether a decline was load or fatigue, which is the one
	// thing a single run can honestly distinguish.
	Block1Accuracy int
	Block5Accuracy int

	// How many questions ran out of time. Worth telling them, because a
	// run with many expiries says something different about the fifteen
	// minutes than a run with none.
	Expired int
}

// AccuracyDrop and ConfidenceDrop are the two movements whose difference
// is the finding.
func (r DTResult) AccuracyDrop() int   { return r.EarlyAccuracy - r.HardAccuracy }
func (r DTResult) ConfidenceDrop() int { return r.EarlyConfidence - r.HardConfidence }

// Gap is how much further accuracy fell than confidence did. A large
// positive number is the effect the test exists to demonstrate.
func (r DTResult) Gap() int { return r.AccuracyDrop() - r.ConfidenceDrop() }

// FatigueHeld reports whether the control block came back near block 1.
// When it does, a decline in between is load rather than tiredness.
func (r DTResult) FatigueHeld() bool { return r.Block5Accuracy >= r.Block1Accuracy-15 }

// DTResultFor computes one participant's figures, for a completed run.
//
// Reads v_dt_answers, the projection boundary (ADR 0030), so these
// numbers are the same ones the admin console and any export show. A
// second query shape here would be a second definition, and the first
// time they disagreed the participant's copy would be the wrong one.
func (r *Repo) DTResultFor(ctx context.Context, sessionID int64) (*DTResult, error) {
	out := &DTResult{}
	err := r.pool.QueryRow(ctx, `
		WITH a AS (
		  SELECT v.* FROM v_dt_answers v
		   JOIN dt_sessions s ON s.public_id::text = v.session_key::text
		  WHERE s.id = $1
		)
		SELECT
		  coalesce((SELECT display_name FROM dt_participants p
		             JOIN dt_sessions s ON s.participant_id = p.id WHERE s.id = $1), ''),
		  coalesce((SELECT email FROM dt_participants p
		             JOIN dt_sessions s ON s.participant_id = p.id WHERE s.id = $1), ''),
		  (SELECT count(*) FROM a WHERE outcome = 'correct'),
		  (SELECT count(*) FROM a),
		  coalesce((SELECT round(100.0*avg(is_correct::int)) FROM a WHERE block_load IN ('d3','d4')), 0),
		  coalesce((SELECT round(avg(confidence))            FROM a WHERE block_load IN ('d3','d4')), 0),
		  coalesce((SELECT round(100.0*avg(is_correct::int)) FROM a WHERE block_load = 'd4_plus3'), 0),
		  coalesce((SELECT round(avg(confidence))            FROM a WHERE block_load = 'd4_plus3'), 0),
		  coalesce((SELECT round(100.0*avg(is_correct::int)) FROM a WHERE block_load = 'd3'), 0),
		  coalesce((SELECT round(100.0*avg(is_correct::int)) FROM a WHERE block_load = 'd3_control'), 0),
		  (SELECT count(*) FROM a WHERE outcome = 'expired')`, sessionID,
	).Scan(&out.Name, &out.Email, &out.Correct, &out.Total,
		&out.EarlyAccuracy, &out.EarlyConfidence,
		&out.HardAccuracy, &out.HardConfidence,
		&out.Block1Accuracy, &out.Block5Accuracy, &out.Expired)
	if err != nil {
		return nil, fmt.Errorf("decision test: result for %d: %w", sessionID, err)
	}
	return out, nil
}

// DTWantsResults reports whether this run asked for its results. Nothing
// is sent to anybody who did not ask, which is the commitment the intake
// form makes and the privacy policy states.
func (r *Repo) DTWantsResults(ctx context.Context, sessionID int64) (bool, string, error) {
	var wants bool
	var email string
	err := r.pool.QueryRow(ctx, `
		SELECT coalesce(p.wants_results,false), coalesce(p.email,'')
		  FROM dt_sessions s LEFT JOIN dt_participants p ON p.id = s.participant_id
		 WHERE s.id = $1`, sessionID).Scan(&wants, &email)
	if err != nil {
		return false, "", fmt.Errorf("decision test: wants results: %w", err)
	}
	return wants && email != "", email, nil
}
