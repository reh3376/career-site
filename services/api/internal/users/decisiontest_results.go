package users

import (
	"context"
	"fmt"
	"time"
)

// Reading decision test runs back, for the admin console.
//
// Everything here goes through v_dt_answers where it can, because that
// view is the projection boundary (ADR 0030): analysis, the export and
// any later graph load all read it, and a second query shape that
// bypasses it is a second definition waiting to disagree.

// DTRun is one run, summarised.
type DTRun struct {
	SessionKey        string
	Status            string
	DisplayName       string
	AgeRange          string
	Education         string
	Occupation        string
	GaveEmail         bool
	AudioMode         string
	DeviceClass       string
	TapCheckPassed    bool
	BaselineRTMs      int
	IsRepeat          bool
	IsSynthetic       bool
	InstrumentVersion string
	ItemSetVersion    string
	Correct           int
	Answered          int
	Expired           int
	MeanConfidence    int
	DurationS         int
	StartedAt         time.Time

	// Curation (M9). ReviewStatus empty means nobody has looked at it,
	// which is a state rather than a verdict.
	ReviewStatus   string
	ReviewReason   string
	ReviewNote     string
	ReviewedAt     *time.Time
	BlocksExcluded int

	// Conditions a reviewer needs in order to judge a run, which were
	// recorded from the first session and shown nowhere until M9.
	// RecallStrategy is the sharpest of them: §3.1 records whether the
	// participant converted the number at encoding or carried it and
	// transformed at recall precisely to turn an uncontrolled variable
	// into a recorded one, and it was invisible.
	RecallStrategy  string
	BaselineRTSDMs  int
	RepeatMatchedBy string
}

// DTBlockSummary is one block of a run, with its recall.
type DTBlockSummary struct {
	BlockNo         int
	Load            string
	Correct         int
	Total           int
	Lure            int
	Expired         int
	MeanConfidence  int
	MeanLatencyMs   int
	PresentedDigits string
	ExpectedDigits  string
	ResponseDigits  string
	RecallOutcome   string
	// 0.0 is a number held intact, 1.0 one lost entirely. Negative means
	// not scored, which is an expired recall: nothing was attempted.
	MemoryFailure float64
}

// DTAnswerRow is one answer.
type DTAnswerRow struct {
	Position   int
	BlockNo    int
	ItemCode   string
	ItemFamily string
	Outcome    string
	Confidence int
	LatencyMs  int
}

// DTListRuns returns runs newest first.
//
// Synthetic runs are excluded unless asked for. They are agent-driven
// and are not data; keeping them out by default means the list reads as
// what it claims to be.
func (r *Repo) DTListRuns(ctx context.Context, includeSynthetic bool, reviewStatus string) ([]DTRun, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.public_id::text, s.status,
		       coalesce(p.display_name,''), coalesce(p.age_range,''),
		       coalesce(p.education,''), coalesce(p.occupation,''),
		       coalesce(p.email,'') <> '',
		       s.audio_mode, s.device_class, s.tap_check_passed,
		       coalesce(s.baseline_rt_ms,0), s.is_repeat, s.is_synthetic,
		       s.instrument_version, s.item_set_version,
		       (SELECT count(*) FROM dt_answers a WHERE a.session_id=s.id AND a.outcome='correct'),
		       (SELECT count(*) FROM dt_answers a WHERE a.session_id=s.id),
		       (SELECT count(*) FROM dt_answers a WHERE a.session_id=s.id AND a.outcome='expired'),
		       coalesce((SELECT round(avg(confidence)) FROM dt_answers a
		                  WHERE a.session_id=s.id AND confidence IS NOT NULL),0),
		       coalesce(round(extract(epoch FROM (s.finished_at - s.started_at))),0),
		       s.started_at,
		       s.review_status, s.review_reason, s.review_note, s.reviewed_at,
		       (SELECT count(*) FROM dt_block_reviews br
		         WHERE br.session_id = s.id AND br.status = 'do_not_use'),
		       s.recall_strategy, coalesce(s.baseline_rt_sd_ms,0), s.repeat_matched_by
		  FROM dt_sessions s
		  LEFT JOIN dt_participants p ON p.id = s.participant_id
		 WHERE ($1 OR NOT s.is_synthetic)
		   -- "unreviewed" rather than an empty string, so asking for the
		   -- queue is explicit and an empty filter still means "all".
		   AND ($2 = ''
		        OR ($2 = 'unreviewed' AND s.review_status = '')
		        OR s.review_status = $2)
		 ORDER BY s.id DESC
		 LIMIT 200`, includeSynthetic, reviewStatus)
	if err != nil {
		return nil, fmt.Errorf("decision test: list runs: %w", err)
	}
	defer rows.Close()

	var out []DTRun
	for rows.Next() {
		var v DTRun
		if err := rows.Scan(&v.SessionKey, &v.Status, &v.DisplayName, &v.AgeRange,
			&v.Education, &v.Occupation, &v.GaveEmail, &v.AudioMode, &v.DeviceClass,
			&v.TapCheckPassed, &v.BaselineRTMs, &v.IsRepeat, &v.IsSynthetic,
			&v.InstrumentVersion, &v.ItemSetVersion, &v.Correct, &v.Answered,
			&v.Expired, &v.MeanConfidence, &v.DurationS, &v.StartedAt,
			&v.ReviewStatus, &v.ReviewReason, &v.ReviewNote, &v.ReviewedAt,
			&v.BlocksExcluded, &v.RecallStrategy, &v.BaselineRTSDMs,
			&v.RepeatMatchedBy); err != nil {
			return nil, fmt.Errorf("decision test: scan run: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// DTGetRun returns one run with its blocks and answers.
func (r *Repo) DTGetRun(ctx context.Context, key string) (*DTRun, []DTBlockSummary, []DTAnswerRow, error) {
	runs, err := r.DTListRuns(ctx, true, "")
	if err != nil {
		return nil, nil, nil, err
	}
	var run *DTRun
	for i := range runs {
		if runs[i].SessionKey == key {
			run = &runs[i]
			break
		}
	}
	if run == nil {
		return nil, nil, nil, fmt.Errorf("decision test: no run %q", key)
	}

	// Blocks, from the view, with the recall joined on.
	brows, err := r.pool.Query(ctx, `
		SELECT v.block_no, v.block_load,
		       count(*) FILTER (WHERE v.is_correct),
		       count(*),
		       count(*) FILTER (WHERE v.is_lure),
		       count(*) FILTER (WHERE v.is_expired),
		       coalesce(round(avg(v.confidence)),0),
		       coalesce(round(avg(v.latency_ms)),0),
		       coalesce(max(rc.presented_digits),''),
		       coalesce(max(rc.expected_digits),''),
		       coalesce(max(rc.response_digits),''),
		       coalesce(max(rc.outcome),''),
		       coalesce(max(v.memory_failure), -1)
		  FROM v_dt_answers v
		  JOIN dt_sessions s  ON s.public_id::text = v.session_key::text
		  LEFT JOIN dt_recalls rc ON rc.session_id = s.id AND rc.block_no = v.block_no
		 WHERE v.session_key::text = $1
		 GROUP BY v.block_no, v.block_load
		 ORDER BY v.block_no`, key)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("decision test: blocks: %w", err)
	}
	defer brows.Close()
	var blocks []DTBlockSummary
	for brows.Next() {
		var b DTBlockSummary
		if err := brows.Scan(&b.BlockNo, &b.Load, &b.Correct, &b.Total, &b.Lure,
			&b.Expired, &b.MeanConfidence, &b.MeanLatencyMs,
			&b.PresentedDigits, &b.ExpectedDigits, &b.ResponseDigits, &b.RecallOutcome,
			&b.MemoryFailure); err != nil {
			return nil, nil, nil, fmt.Errorf("decision test: scan block: %w", err)
		}
		blocks = append(blocks, b)
	}
	if err := brows.Err(); err != nil {
		return nil, nil, nil, err
	}

	arows, err := r.pool.Query(ctx, `
		SELECT position_overall, block_no, item_code, item_family, outcome,
		       coalesce(confidence,0), coalesce(latency_ms,0)
		  FROM v_dt_answers WHERE session_key::text = $1
		 ORDER BY position_overall`, key)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("decision test: answers: %w", err)
	}
	defer arows.Close()
	var answers []DTAnswerRow
	for arows.Next() {
		var a DTAnswerRow
		if err := arows.Scan(&a.Position, &a.BlockNo, &a.ItemCode, &a.ItemFamily,
			&a.Outcome, &a.Confidence, &a.LatencyMs); err != nil {
			return nil, nil, nil, fmt.Errorf("decision test: scan answer: %w", err)
		}
		answers = append(answers, a)
	}
	return run, blocks, answers, arows.Err()
}
