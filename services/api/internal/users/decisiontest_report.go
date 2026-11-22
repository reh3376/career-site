package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/reh3376/career-site/services/api/internal/dtreport"
)

// The decision test's results report: reading what it is built from,
// and keeping what was built (FSD FR-DT-21 to 24).
//
// Nothing here decides what a report says. The rows go to the dtreport
// package, which works out every figure and chooses every sentence, and
// this file stores what comes back.

// DTReportMinPeers is how many other first-time participants a ranking
// needs. Below it a report covers the run on its own terms and says
// that a comparison is not possible yet. Ten is the owner's decision
// (2026-11-21): a percentile from five people moves seventeen points a
// place.
const DTReportMinPeers = 10

// DTReportMaxAttempts is how often a report is tried before the
// participant is sent their results without it.
const DTReportMaxAttempts = 3

// DTReportRetryAfter is how long a failed build waits before the sweep
// tries it again. The usual cause is the renderer restarting during a
// deploy, which is over in a couple of minutes.
const DTReportRetryAfter = 2 * time.Minute

// ErrDTRunIncomplete marks a run that was closed without all of its
// answers. Retrying cannot fix it, so the sweep stops trying.
var ErrDTRunIncomplete = errors.New("decision test: the run is not complete")

// DTReportInput gathers everything one run's report is built from: the
// run, and every other finished, non-synthetic run on the same question
// pool and the same time limits.
//
// The peers are read through v_dt_answers, the projection boundary
// (ADR 0030), so "usable", the sitting number and each outcome mean
// here exactly what they mean in the console and the export. Who among
// them is comparable is decided by dtreport, in one tested place.
func (r *Repo) DTReportInput(ctx context.Context, sessionID int64) (*dtreport.Input, error) {
	run := dtreport.Run{Usable: true}
	var status string
	err := r.pool.QueryRow(ctx, `
		SELECT s.public_id::text, s.status, s.started_at, coalesce(s.finished_at, s.started_at),
		       s.device_class, s.audio_mode, s.timer_visible,
		       s.instrument_version, s.item_set_version, s.is_synthetic, s.recall_strategy,
		       dt_attempt_no(s.prior_by_account, s.prior_by_email, s.prior_by_cookie)
		  FROM dt_sessions s WHERE s.id = $1`, sessionID,
	).Scan(&run.Key, &status, &run.Started, &run.Finished, &run.Device, &run.Audio, &run.TimerVisible,
		&run.InstrumentVersion, &run.ItemSetVersion, &run.Synthetic, &run.RecallStrategy, &run.AttemptNo)
	if err != nil {
		return nil, fmt.Errorf("decision test report: session %d: %w", sessionID, err)
	}
	if status != "completed" {
		return nil, fmt.Errorf("%w: status is %s", ErrDTRunIncomplete, status)
	}

	// The run's own answers. The view for everything it defines, the
	// base tables only for what it withholds: the question, what was
	// picked and what was right, which go no further than the owner's
	// copy.
	rows, err := r.pool.Query(ctx, `
		SELECT v.position_overall, v.block_no, v.block_load, v.item_code, v.item_family, v.outcome,
		       coalesce(v.confidence, -1), coalesce(v.latency_ms, 0),
		       coalesce(jsonb_array_length(i.options), 0),
		       coalesce(i.prompt, ''),
		       coalesce(i.options ->> a.chosen_index, ''),
		       coalesce(i.options ->> i.correct_index, '')
		  FROM v_dt_answers v
		  JOIN dt_answers a ON a.id = v.answer_id
		  JOIN dt_items i   ON i.id = a.item_id
		 WHERE a.session_id = $1
		 ORDER BY v.position_overall`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("decision test report: answers: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var a dtreport.Answer
		if err := rows.Scan(&a.Position, &a.BlockNo, &a.Load, &a.ItemCode, &a.Family, &a.Outcome,
			&a.Confidence, &a.LatencyMs, &a.Options, &a.Prompt, &a.ChosenText, &a.CorrectText); err != nil {
			return nil, fmt.Errorf("decision test report: scan answer: %w", err)
		}
		run.Answers = append(run.Answers, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// A report describes a whole test. A run closed early has no fifth
	// block to check for tiring and no score out of thirty, and a report
	// built on it would state both.
	if want := DTBlockCount * DTQuestionsPerBlock; len(run.Answers) != want {
		return nil, fmt.Errorf("%w: %d of %d answers", ErrDTRunIncomplete, len(run.Answers), want)
	}

	rrows, err := r.pool.Query(ctx, `
		SELECT block_no, block_load, presented_digits, expected_digits, response_digits, outcome,
		       coalesce(digits_held, 0)
		  FROM dt_recalls WHERE session_id = $1 ORDER BY block_no`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("decision test report: recalls: %w", err)
	}
	defer rrows.Close()
	for rrows.Next() {
		var rc dtreport.Recall
		if err := rrows.Scan(&rc.BlockNo, &rc.Load, &rc.Presented, &rc.Expected, &rc.Response,
			&rc.Outcome, &rc.DigitsHeld); err != nil {
			return nil, fmt.Errorf("decision test report: scan recall: %w", err)
		}
		run.Recalls = append(run.Recalls, rc)
	}
	if err := rrows.Err(); err != nil {
		return nil, err
	}

	in := &dtreport.Input{Run: run, MinPeers: DTReportMinPeers, Now: time.Now().UTC()}
	if err := r.pool.QueryRow(ctx, `SELECT dt_confident_threshold()`).Scan(&in.ConfidentThreshold); err != nil {
		return nil, fmt.Errorf("decision test report: threshold: %w", err)
	}

	// Everybody else who finished under the same questions and the same
	// time limits. Runs under different timings are a different
	// instrument and are never pooled (FR-DT-15); the generation prefix
	// is left to dtreport, which knows which generations sit as the same
	// test.
	prows, err := r.pool.Query(ctx, `
		SELECT v.session_key::text, v.instrument_version, v.attempt_no, v.usable, v.started_at,
		       v.block_no, v.block_load, v.position_overall, v.item_code, v.item_family, v.outcome,
		       coalesce(v.confidence, -1), coalesce(v.latency_ms, 0), coalesce(v.recall_outcome, '')
		  FROM v_dt_answers v
		 WHERE v.status = 'completed'
		   AND NOT v.is_synthetic
		   AND v.item_set_version = $1
		   AND substring(v.instrument_version from position('-' in v.instrument_version) + 1) = $2
		   AND v.session_key::text <> $3
		 ORDER BY v.session_key, v.position_overall`,
		run.ItemSetVersion, run.Timings(), run.Key)
	if err != nil {
		return nil, fmt.Errorf("decision test report: peers: %w", err)
	}
	defer prows.Close()
	index := map[string]int{}
	recalled := map[string]map[int]bool{}
	for prows.Next() {
		var (
			key, version, recall string
			attempt              *int
			usable               bool
			started              time.Time
			a                    dtreport.Answer
		)
		if err := prows.Scan(&key, &version, &attempt, &usable, &started,
			&a.BlockNo, &a.Load, &a.Position, &a.ItemCode, &a.Family, &a.Outcome,
			&a.Confidence, &a.LatencyMs, &recall); err != nil {
			return nil, fmt.Errorf("decision test report: scan peer: %w", err)
		}
		i, ok := index[key]
		if !ok {
			i = len(in.Peers)
			index[key] = i
			recalled[key] = map[int]bool{}
			in.Peers = append(in.Peers, dtreport.Run{Key: key, InstrumentVersion: version,
				ItemSetVersion: run.ItemSetVersion, AttemptNo: attempt, Usable: true, Started: started})
		}
		p := &in.Peers[i]
		// One block set aside on review makes the whole run unusable for
		// a comparison of scores out of thirty.
		p.Usable = p.Usable && usable
		p.Answers = append(p.Answers, a)
		if recall != "" && !recalled[key][a.BlockNo] {
			recalled[key][a.BlockNo] = true
			p.Recalls = append(p.Recalls, dtreport.Recall{BlockNo: a.BlockNo, Load: a.Load, Outcome: recall})
		}
	}
	if err := prows.Err(); err != nil {
		return nil, err
	}

	// The figures the results email states, from the function that
	// computes them for the email. The report is attached to that email
	// and must not arrive at a different number for the same thing.
	res, err := r.DTResultFor(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	in.Email = dtreport.EmailFigures{
		EarlyAccuracy: res.EarlyAccuracy, EarlyConfidence: res.EarlyConfidence,
		HardAccuracy: res.HardAccuracy, HardConfidence: res.HardConfidence,
		Reading: res.Reading(), FatigueHeld: res.FatigueHeld(),
	}
	return in, nil
}

// DTReportRecord is one stored build of a report, without the PDFs.
type DTReportRecord struct {
	Version          int
	Template         string
	Engine           string
	Compared         bool
	Reason           string
	Others           int
	Summary          dtreport.Summary
	ParticipantPages int
	OwnerPages       int
	TriggeredBy      string
	CreatedAt        time.Time
}

// DTReportBuild is a finished build on its way into storage.
type DTReportBuild struct {
	Template         string
	Engine           string
	Summary          dtreport.Summary
	ParticipantPDF   []byte
	OwnerPDF         []byte
	ParticipantPages int
	OwnerPages       int
	TriggeredBy      string
}

// DTReportSave stores a build as the run's next version and marks the
// retry ledger done.
//
// The version is taken inside the insert, under the unique constraint,
// so two builds racing for the same run cannot both become version 1:
// one of them fails and is retried as version 2.
func (r *Repo) DTReportSave(ctx context.Context, sessionID int64, b DTReportBuild) (int, error) {
	summary, err := json.Marshal(b.Summary)
	if err != nil {
		return 0, fmt.Errorf("decision test report: summary: %w", err)
	}
	by := b.TriggeredBy
	if by == "" {
		by = "system"
	}
	var version int
	err = pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			INSERT INTO dt_reports
			  (session_id, version, template, engine, compared, no_ranking_reason, others, summary,
			   participant_pdf, owner_pdf, participant_pages, owner_pages, triggered_by)
			SELECT $1, coalesce(max(version), 0) + 1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
			  FROM dt_reports WHERE session_id = $1
			RETURNING version`,
			sessionID, b.Template, b.Engine, b.Summary.Compared, b.Summary.Reason, b.Summary.Others, summary,
			b.ParticipantPDF, b.OwnerPDF, b.ParticipantPages, b.OwnerPages, by,
		).Scan(&version); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO dt_report_jobs (session_id, attempts, last_attempt_at, done_at)
			VALUES ($1, 1, now(), now())
			ON CONFLICT (session_id) DO UPDATE
			   SET attempts = dt_report_jobs.attempts + 1, last_attempt_at = now(), done_at = now(),
			       last_error = '', permanent = false`, sessionID)
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("decision test report: save: %w", err)
	}
	return version, nil
}

// DTReportFailed records an attempt that did not produce a report.
func (r *Repo) DTReportFailed(ctx context.Context, sessionID int64, cause error, permanent bool) error {
	msg := cause.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO dt_report_jobs (session_id, attempts, last_error, last_attempt_at, permanent)
		VALUES ($1, 1, $2, now(), $3)
		ON CONFLICT (session_id) DO UPDATE
		   SET attempts = dt_report_jobs.attempts + 1, last_error = $2, last_attempt_at = now(),
		       permanent = dt_report_jobs.permanent OR $3`, sessionID, msg, permanent)
	if err != nil {
		return fmt.Errorf("decision test report: record failure: %w", err)
	}
	return nil
}

// DTReportsDue lists finished runs that still need work: a report to
// build, or a results email that is owed and not yet settled.
//
// settle is how long a run is left alone after it closes. The run
// screen closes a run and then asks how the participant held the
// number, and the report quotes the answer, so a build that started the
// instant the run closed would miss it for everybody. A participant who
// answers triggers the build directly; this is for the ones who shut
// the tab, and for retries.
func (r *Repo) DTReportsDue(ctx context.Context, settle time.Duration, limit int) ([]int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id
		  FROM dt_sessions s
		  LEFT JOIN dt_report_jobs j ON j.session_id = s.id
		 WHERE s.status = 'completed'
		   AND s.finished_at < now() - make_interval(secs => $1)
		   AND (
		         -- A report to build: none yet, still worth trying, and
		         -- the last attempt long enough ago to be worth another.
		         (j.done_at IS NULL
		          AND NOT coalesce(j.permanent, false)
		          AND coalesce(j.attempts, 0) < $2
		          AND (j.last_attempt_at IS NULL
		               OR j.last_attempt_at < now() - make_interval(secs => $3)))
		         -- Or the build is settled, one way or the other, and
		         -- the email that waited on it is not.
		         OR ((j.done_at IS NOT NULL
		              OR coalesce(j.permanent, false)
		              OR coalesce(j.attempts, 0) >= $2)
		             AND NOT EXISTS (SELECT 1 FROM dt_result_emails e
		                              WHERE e.session_id = s.id AND e.source = 'system'))
		       )
		 ORDER BY s.finished_at
		 LIMIT $4`, settle.Seconds(), DTReportMaxAttempts, DTReportRetryAfter.Seconds(), limit)
	if err != nil {
		return nil, fmt.Errorf("decision test report: due: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DTReportState is where one run's report stands.
type DTReportState struct {
	SessionID  int64
	SessionKey string
	StartedAt  time.Time
	Status     string

	// Nil when no report has been built.
	Report *DTReportRecord

	// The retry ledger.
	Attempts  int
	Permanent bool
	LastError string

	// The most recent email that went out, automatic or from the
	// console.
	EmailedAt      *time.Time
	EmailedBy      string
	EmailedVersion int

	// Whether the participant asked for results and left an address.
	CanSend bool
}

// Exhausted reports whether the build has been given up on, so the
// participant is owed their results without it.
func (s *DTReportState) Exhausted() bool {
	return s.Report == nil && (s.Permanent || s.Attempts >= DTReportMaxAttempts)
}

// DTReportStateByID reads a run's report state by session id.
func (r *Repo) DTReportStateByID(ctx context.Context, sessionID int64) (*DTReportState, error) {
	return r.dtReportState(ctx, `s.id = $1`, sessionID)
}

// DTReportStateByKey reads it by the public key the console uses.
func (r *Repo) DTReportStateByKey(ctx context.Context, key string) (*DTReportState, error) {
	return r.dtReportState(ctx, `s.public_id::text = $1`, key)
}

func (r *Repo) dtReportState(ctx context.Context, where string, arg any) (*DTReportState, error) {
	st := &DTReportState{}
	var wants bool
	var email string
	err := r.pool.QueryRow(ctx, `
		SELECT s.id, s.public_id::text, s.started_at, s.status,
		       coalesce(j.attempts, 0), coalesce(j.permanent, false), coalesce(j.last_error, ''),
		       coalesce(p.wants_results, false), coalesce(p.email, '')
		  FROM dt_sessions s
		  LEFT JOIN dt_report_jobs j ON j.session_id = s.id
		  LEFT JOIN dt_participants p ON p.id = s.participant_id
		 WHERE `+where, arg,
	).Scan(&st.SessionID, &st.SessionKey, &st.StartedAt, &st.Status,
		&st.Attempts, &st.Permanent, &st.LastError, &wants, &email)
	if err != nil {
		return nil, fmt.Errorf("decision test report: state: %w", err)
	}
	st.CanSend = wants && email != ""

	rec := &DTReportRecord{}
	var summary []byte
	err = r.pool.QueryRow(ctx, `
		SELECT version, template, engine, compared, no_ranking_reason, others, summary,
		       participant_pages, owner_pages, triggered_by, created_at
		  FROM dt_reports WHERE session_id = $1
		 ORDER BY version DESC LIMIT 1`, st.SessionID,
	).Scan(&rec.Version, &rec.Template, &rec.Engine, &rec.Compared, &rec.Reason, &rec.Others, &summary,
		&rec.ParticipantPages, &rec.OwnerPages, &rec.TriggeredBy, &rec.CreatedAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return nil, fmt.Errorf("decision test report: latest: %w", err)
	default:
		if err := json.Unmarshal(summary, &rec.Summary); err != nil {
			return nil, fmt.Errorf("decision test report: summary: %w", err)
		}
		st.Report = rec
	}

	var version *int
	err = r.pool.QueryRow(ctx, `
		SELECT e.sent_at, CASE WHEN e.source = 'system' THEN 'system' ELSE coalesce(u.name, 'admin') END,
		       e.report_version
		  FROM dt_result_emails e
		  LEFT JOIN users u ON u.id = e.sent_by
		 WHERE e.session_id = $1 AND e.outcome = 'sent'
		 ORDER BY e.sent_at DESC NULLS LAST, e.id DESC LIMIT 1`, st.SessionID,
	).Scan(&st.EmailedAt, &st.EmailedBy, &version)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return nil, fmt.Errorf("decision test report: last email: %w", err)
	default:
		if version != nil {
			st.EmailedVersion = *version
		}
	}
	return st, nil
}

// DTReportPDF returns one copy of a run's latest report.
func (r *Repo) DTReportPDF(ctx context.Context, sessionID int64, owner bool) (pdf []byte, version int, err error) {
	col := "participant_pdf"
	if owner {
		col = "owner_pdf"
	}
	err = r.pool.QueryRow(ctx, `
		SELECT `+col+`, version FROM dt_reports
		 WHERE session_id = $1 ORDER BY version DESC LIMIT 1`, sessionID).Scan(&pdf, &version)
	if err != nil {
		return nil, 0, fmt.Errorf("decision test report: pdf: %w", err)
	}
	return pdf, version, nil
}

// DTReportFilename names a copy of a run's report after the run: the
// whole session id, not a shortened one.
//
// The owner asked for a file that can be traced to its test taker from
// the name alone. The eight-character form the console prints is unique
// only as long as no two runs share a prefix, which the console checks
// for and widens around (abbreviate.ts); a filename has nothing beside
// it to be checked against, so it carries the id in full.
func DTReportFilename(sessionKey string, owner bool) string {
	name := "decision-test-results-" + sessionKey
	if owner {
		name += "-owner-copy"
	}
	return name + ".pdf"
}

// DTResultEmailClaim takes the one automatic results email a run is
// allowed. It reports false when it has already been taken, which is
// every call after the first: the unique index decides, not a check
// that two callers could both pass.
func (r *Repo) DTResultEmailClaim(ctx context.Context, sessionID int64) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO dt_result_emails (session_id, source, outcome)
		VALUES ($1, 'system', 'claimed')
		ON CONFLICT (session_id) WHERE source = 'system' DO NOTHING`, sessionID)
	if err != nil {
		return false, fmt.Errorf("decision test: claim result email: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// DTResultEmailSettle records how the automatic email ended.
func (r *Repo) DTResultEmailSettle(ctx context.Context, sessionID int64, outcome string, reportVersion int, cause error) error {
	var version *int
	if reportVersion > 0 {
		version = &reportVersion
	}
	msg := ""
	if cause != nil {
		msg = cause.Error()
		if len(msg) > 500 {
			msg = msg[:500]
		}
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE dt_result_emails
		   SET outcome = $2, report_version = $3, error = $4,
		       sent_at = CASE WHEN $2 = 'sent' THEN now() ELSE sent_at END
		 WHERE session_id = $1 AND source = 'system'`, sessionID, outcome, version, msg)
	if err != nil {
		return fmt.Errorf("decision test: settle result email: %w", err)
	}
	return nil
}

// DTResultEmailRecordAdmin records a send made from the console.
func (r *Repo) DTResultEmailRecordAdmin(ctx context.Context, sessionID, adminID int64, reportVersion int, cause error) error {
	outcome, msg := "sent", ""
	if cause != nil {
		outcome, msg = "failed", cause.Error()
		if len(msg) > 500 {
			msg = msg[:500]
		}
	}
	var by *int64
	if adminID > 0 {
		by = &adminID
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO dt_result_emails (session_id, source, sent_by, outcome, report_version, error, sent_at)
		VALUES ($1, 'admin', $2, $3, $4, $5, CASE WHEN $3 = 'sent' THEN now() END)`,
		sessionID, by, outcome, reportVersion, msg)
	if err != nil {
		return fmt.Errorf("decision test: record admin send: %w", err)
	}
	return nil
}
