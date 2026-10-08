package users

import (
	"context"
	"fmt"
)

// The three views that answer the research question, read for the
// admin console.
//
// They have existed since migration 00054 and were readable only
// through /admin/db, which means the finding this whole instrument
// exists to produce has been one SQL console away from anybody wanting
// to look at it. These are thin on purpose: every number is defined in
// the view (FR-DT-15, docs/metrics.md) and nothing is recomputed here,
// because a second definition is how two numbers that should agree
// stop agreeing.

// DTLoadPoint is one load level on the curve.
type DTLoadPoint struct {
	ItemSetVersion      string
	InstrumentVersion   string
	Load                string
	LoadRank            int
	Sessions            int
	SessionsUnreviewed  int
	SessionsFirst       int
	SessionsRepeat      int
	Answers             int
	AccuracyPct         int
	LurePct             int
	ExpiryPct           int
	MeanConfidence      int
	GapPct              int
	ConfidentlyWrongPct int
	MeanBrier           float64
	// The item families behind this point, joined with '+'. A point
	// drawing on one family measures the family as much as the load,
	// which is what made the first two real runs unusable: the blocks
	// were family-blocked, so block 1 was six arithmetic items and
	// block 4 was conjunctions and syllogisms.
	//
	// Text rather than a count, because which families is the useful
	// question and the count follows from it.
	Families string
}

// DTLoadCurve is the result: accuracy and confidence by load, grouped
// by instrument so runs taken under different items or timings never
// pool into one claim.
func (r *Repo) DTLoadCurve(ctx context.Context) ([]DTLoadPoint, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT item_set_version, instrument_version, block_load, load_rank,
		       sessions, sessions_unreviewed, sessions_first_attempt, sessions_repeat,
		       answers, coalesce(accuracy_pct,0), coalesce(lure_pct,0),
		       coalesce(expiry_pct,0), coalesce(mean_confidence,0), coalesce(gap_pct,0),
		       coalesce(confidently_wrong_pct,0), coalesce(mean_brier,0),
		       coalesce(families,'')
		  FROM v_dt_load_curve
		 ORDER BY instrument_version, item_set_version, load_rank`)
	if err != nil {
		return nil, fmt.Errorf("decision test: load curve: %w", err)
	}
	defer rows.Close()
	var out []DTLoadPoint
	for rows.Next() {
		var p DTLoadPoint
		if err := rows.Scan(&p.ItemSetVersion, &p.InstrumentVersion, &p.Load, &p.LoadRank,
			&p.Sessions, &p.SessionsUnreviewed, &p.SessionsFirst, &p.SessionsRepeat,
			&p.Answers, &p.AccuracyPct, &p.LurePct, &p.ExpiryPct, &p.MeanConfidence,
			&p.GapPct, &p.ConfidentlyWrongPct, &p.MeanBrier, &p.Families); err != nil {
			return nil, fmt.Errorf("decision test: scan load point: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DTCalibrationPoint is one confidence band at one load.
type DTCalibrationPoint struct {
	ItemSetVersion    string
	InstrumentVersion string
	Load              string
	LoadRank          int
	Band              int
	Answers           int
	AccuracyPct       int
	OverclaimPct      int
	MeanBrier         float64
}

// DTCalibration is the reliability curve: what each confidence band
// actually achieved. Positive overclaim is a band more sure than right.
func (r *Repo) DTCalibration(ctx context.Context) ([]DTCalibrationPoint, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT item_set_version, instrument_version, block_load, load_rank,
		       confidence_band, answers, coalesce(accuracy_pct,0),
		       coalesce(overclaim_pct,0), coalesce(mean_brier,0)
		  FROM v_dt_calibration
		 ORDER BY instrument_version, item_set_version, load_rank, confidence_band`)
	if err != nil {
		return nil, fmt.Errorf("decision test: calibration: %w", err)
	}
	defer rows.Close()
	var out []DTCalibrationPoint
	for rows.Next() {
		var p DTCalibrationPoint
		if err := rows.Scan(&p.ItemSetVersion, &p.InstrumentVersion, &p.Load, &p.LoadRank,
			&p.Band, &p.Answers, &p.AccuracyPct, &p.OverclaimPct, &p.MeanBrier); err != nil {
			return nil, fmt.Errorf("decision test: scan calibration point: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DTThresholdRow is one participant's threshold.
type DTThresholdRow struct {
	SessionKey      string
	AttemptNo       *int
	ReviewStatus    string
	AccuracyPct     int
	GapPct          int
	FatigueDeltaPct int
	// The lowest load on the ramp at which this participant took the
	// intended wrong answer while still reporting high confidence.
	LuredAtRank      int
	LuredAtLoad      string
	LuredInControl   bool
	ConfidentlyWrong int
}

// DTThresholds is the per-person answer, newest first.
//
// Synthetic runs are filtered HERE, not by the view. v_dt_threshold is
// a review surface and exposes `is_synthetic` rather than removing it,
// the same as v_dt_sessions and v_dt_blocks, so that an agent-driven
// run can still be looked at deliberately. The claim-making views
// (v_dt_load_curve, v_dt_calibration) filter it themselves.
//
// Reading it without this filter put fifteen of my own test runs in a
// table on a page that said agent-driven runs were excluded. Caught by
// opening the page, which is the only thing that would have caught it:
// the query was valid, the view was right, and the page was confidently
// wrong.
func (r *Repo) DTThresholds(ctx context.Context) ([]DTThresholdRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT session_key::text, attempt_no, session_review_status,
		       coalesce(accuracy_pct,0), coalesce(gap_pct,0),
		       coalesce(fatigue_delta_pct,0), coalesce(lured_at_rank,0),
		       coalesce(lured_at_load,''), coalesce(lured_in_control,false),
		       coalesce(confidently_wrong,0)
		  FROM v_dt_threshold
		 WHERE NOT is_synthetic
		 ORDER BY started_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("decision test: thresholds: %w", err)
	}
	defer rows.Close()
	var out []DTThresholdRow
	for rows.Next() {
		var t DTThresholdRow
		if err := rows.Scan(&t.SessionKey, &t.AttemptNo, &t.ReviewStatus,
			&t.AccuracyPct, &t.GapPct, &t.FatigueDeltaPct, &t.LuredAtRank,
			&t.LuredAtLoad, &t.LuredInControl, &t.ConfidentlyWrong); err != nil {
			return nil, fmt.Errorf("decision test: scan threshold: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
