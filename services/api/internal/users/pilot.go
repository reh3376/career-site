package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/reh3376/career-site/services/api/internal/tenant"
)

// Storage for the employer-side pilot (FSD §5.16, migration 00065).
//
// Everything here is keyed by the participant's user id and the
// instrument version; a participant has one session per version and
// may leave and return. Nothing is ever deleted: a comment set to an
// empty body is removed, which is the one exception, because an empty
// comment is not data.

// PilotSession is one sitting.
type PilotSession struct {
	ID                  string
	UserID              int64
	InstrumentVersion   string
	RoleBranch          string
	Status              string
	ConsentAt           time.Time
	RecordingOK         bool
	SurveyFinishedAt    *time.Time
	RecordFinishedAt    *time.Time
	WorkflowFinishedAt  *time.Time
	FinishedAt          *time.Time
	SelfReportedMinutes *int32
	MostWantedToSkip    string
	CreatedAt           time.Time
}

// PilotAnswer is one answered item.
type PilotAnswer struct {
	Section    string
	ItemID     string
	ValueJSON  string
	Flagged    bool
	AnsweredAt time.Time
}

// PilotComment is one comment on a target.
type PilotComment struct {
	TargetKind string
	TargetID   string
	Body       string
	UpdatedAt  time.Time
}

// PilotSectionEvent is an open or submit instant.
type PilotSectionEvent struct {
	Section string
	Event   string
	At      time.Time
}

// PilotSessionSummary is the admin list row.
type PilotSessionSummary struct {
	Session      PilotSession
	Name         string
	Email        string
	FirstOpened  *time.Time
	LastSubmit   *time.Time
	CommentCount int32
	FlagCount    int32
}

const pilotSessionCols = `
    s.id::text, s.user_id, s.instrument_version, s.role_branch, s.status, s.consent_at,
    s.recording_ok, s.survey_finished_at, s.record_finished_at, s.workflow_finished_at,
    s.finished_at, s.self_reported_minutes, s.most_wanted_to_skip, s.created_at
`

func scanPilotSession(row pgx.Row) (*PilotSession, error) {
	s := &PilotSession{}
	err := row.Scan(
		&s.ID, &s.UserID, &s.InstrumentVersion, &s.RoleBranch, &s.Status, &s.ConsentAt,
		&s.RecordingOK, &s.SurveyFinishedAt, &s.RecordFinishedAt, &s.WorkflowFinishedAt,
		&s.FinishedAt, &s.SelfReportedMinutes, &s.MostWantedToSkip, &s.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan pilot session: %w", err)
	}
	return s, nil
}

// EnsurePilotInstrument records a version's definition the first time
// it is fielded and leaves an existing copy alone, so the file in the
// binary can change and a past session still reads what it answered.
func (r *Repo) EnsurePilotInstrument(ctx context.Context, version string, definition []byte) error {
	_, err := r.pool.Exec(ctx, `
    INSERT INTO pilot_instruments (version, tenant_id, definition)
    VALUES ($1, $2, $3::jsonb)
    ON CONFLICT (version) DO NOTHING`,
		version, tenant.FromContext(ctx).Int64(), string(definition))
	if err != nil {
		return fmt.Errorf("ensure pilot instrument: %w", err)
	}
	return nil
}

// GetPilotSession returns the participant's session for a version.
func (r *Repo) GetPilotSession(ctx context.Context, userID int64, version string) (*PilotSession, error) {
	return scanPilotSession(r.pool.QueryRow(ctx, `
    SELECT `+pilotSessionCols+` FROM pilot_sessions s
     WHERE s.user_id = $1 AND s.instrument_version = $2`, userID, version))
}

// GetPilotSessionByID returns one session.
func (r *Repo) GetPilotSessionByID(ctx context.Context, id string) (*PilotSession, error) {
	return scanPilotSession(r.pool.QueryRow(ctx, `
    SELECT `+pilotSessionCols+` FROM pilot_sessions s WHERE s.id = $1::uuid`, id))
}

// StartPilotSession opens a session or returns the existing one. The
// consent instant is written once; a second start does not move it.
func (r *Repo) StartPilotSession(ctx context.Context, userID int64, version string, recordingOK bool) (*PilotSession, error) {
	_, err := r.pool.Exec(ctx, `
    INSERT INTO pilot_sessions (tenant_id, user_id, instrument_version, recording_ok)
    VALUES ($1, $2, $3, $4)
    ON CONFLICT (user_id, instrument_version) DO NOTHING`,
		tenant.FromContext(ctx).Int64(), userID, version, recordingOK)
	if err != nil {
		return nil, fmt.Errorf("start pilot session: %w", err)
	}
	return r.GetPilotSession(ctx, userID, version)
}

// SetPilotRoleBranch records the branch A1 routed to.
func (r *Repo) SetPilotRoleBranch(ctx context.Context, sessionID, branch string) error {
	_, err := r.pool.Exec(ctx, `UPDATE pilot_sessions SET role_branch = $2 WHERE id = $1::uuid`, sessionID, branch)
	if err != nil {
		return fmt.Errorf("set pilot branch: %w", err)
	}
	return nil
}

// UpsertPilotAnswers replaces a section's answers in one statement per
// item. Items not in the list are left as they were, so a client that
// omits an optional item does not erase an earlier answer to it.
func (r *Repo) UpsertPilotAnswers(ctx context.Context, sessionID, section string, answers []PilotAnswer) error {
	t := tenant.FromContext(ctx).Int64()
	for _, a := range answers {
		if _, err := r.pool.Exec(ctx, `
    INSERT INTO pilot_answers (tenant_id, session_id, section, item_id, value, flagged)
    VALUES ($1, $2::uuid, $3, $4, $5::jsonb, $6)
    ON CONFLICT (session_id, item_id) DO UPDATE
      SET value = EXCLUDED.value, flagged = EXCLUDED.flagged, section = EXCLUDED.section,
          answered_at = now()`,
			t, sessionID, section, a.ItemID, a.ValueJSON, a.Flagged); err != nil {
			return fmt.Errorf("upsert pilot answer %s: %w", a.ItemID, err)
		}
	}
	return nil
}

// ListPilotAnswers returns every answer of a session, oldest first.
func (r *Repo) ListPilotAnswers(ctx context.Context, sessionID string) ([]PilotAnswer, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT section, item_id, value::text, flagged, answered_at
      FROM pilot_answers WHERE session_id = $1::uuid ORDER BY answered_at, item_id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list pilot answers: %w", err)
	}
	defer rows.Close()
	var out []PilotAnswer
	for rows.Next() {
		var a PilotAnswer
		if err := rows.Scan(&a.Section, &a.ItemID, &a.ValueJSON, &a.Flagged, &a.AnsweredAt); err != nil {
			return nil, fmt.Errorf("scan pilot answer: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// RecordPilotSectionEvent writes an open or submit instant.
func (r *Repo) RecordPilotSectionEvent(ctx context.Context, sessionID, section, event string) error {
	// One row per real instant. A section page renders more than once
	// per visit (a prefetch, a re-render after the submit that leaves
	// it), and each render asks for the section, so a bare insert wrote
	// two or three "opened" rows a few hundred milliseconds apart. The
	// window is short enough that a participant who genuinely reopens
	// a section is still recorded.
	_, err := r.pool.Exec(ctx, `
    INSERT INTO pilot_section_events (tenant_id, session_id, section, event)
    SELECT $1, $2::uuid, $3, $4
     WHERE NOT EXISTS (
       SELECT 1 FROM pilot_section_events
        WHERE session_id = $2::uuid AND section = $3 AND event = $4
          AND at > now() - interval '10 seconds')`,
		tenant.FromContext(ctx).Int64(), sessionID, section, event)
	if err != nil {
		return fmt.Errorf("record pilot section event: %w", err)
	}
	return nil
}

// ListPilotSectionEvents returns a session's instants, oldest first.
func (r *Repo) ListPilotSectionEvents(ctx context.Context, sessionID string) ([]PilotSectionEvent, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT section, event, at FROM pilot_section_events
     WHERE session_id = $1::uuid ORDER BY at`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list pilot section events: %w", err)
	}
	defer rows.Close()
	var out []PilotSectionEvent
	for rows.Next() {
		var e PilotSectionEvent
		if err := rows.Scan(&e.Section, &e.Event, &e.At); err != nil {
			return nil, fmt.Errorf("scan pilot section event: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// SubmittedPilotSections returns the ids of sections with a submit
// instant, so the next section can be computed from the record rather
// than from client state.
func (r *Repo) SubmittedPilotSections(ctx context.Context, sessionID string) (map[string]bool, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT DISTINCT section FROM pilot_section_events
     WHERE session_id = $1::uuid AND event = 'submitted'`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("submitted pilot sections: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("scan submitted section: %w", err)
		}
		out[s] = true
	}
	return out, rows.Err()
}

// MarkPilotSurveyFinished stamps the survey as complete and records the
// two pilot-only closing answers on the session for the admin list.
func (r *Repo) MarkPilotSurveyFinished(ctx context.Context, sessionID string, minutes *int32, skip string) error {
	_, err := r.pool.Exec(ctx, `
    UPDATE pilot_sessions
       SET survey_finished_at = coalesce(survey_finished_at, now()),
           self_reported_minutes = coalesce($2, self_reported_minutes),
           most_wanted_to_skip = CASE WHEN $3 <> '' THEN $3 ELSE most_wanted_to_skip END
     WHERE id = $1::uuid`, sessionID, minutes, skip)
	if err != nil {
		return fmt.Errorf("mark pilot survey finished: %w", err)
	}
	return nil
}

// MarkPilotPart stamps the record or the workflow part done; a second
// mark leaves the first instant in place.
func (r *Repo) MarkPilotPart(ctx context.Context, sessionID, part string) error {
	var q string
	switch part {
	case "record":
		q = `UPDATE pilot_sessions SET record_finished_at = coalesce(record_finished_at, now()) WHERE id = $1::uuid`
	case "workflow":
		q = `UPDATE pilot_sessions SET workflow_finished_at = coalesce(workflow_finished_at, now()) WHERE id = $1::uuid`
	default:
		return fmt.Errorf("mark pilot part: unknown part %q", part)
	}
	if _, err := r.pool.Exec(ctx, q, sessionID); err != nil {
		return fmt.Errorf("mark pilot part: %w", err)
	}
	return nil
}

// FinishPilotSession closes a session.
func (r *Repo) FinishPilotSession(ctx context.Context, sessionID string) error {
	_, err := r.pool.Exec(ctx, `
    UPDATE pilot_sessions SET status = 'finished', finished_at = coalesce(finished_at, now())
     WHERE id = $1::uuid`, sessionID)
	if err != nil {
		return fmt.Errorf("finish pilot session: %w", err)
	}
	return nil
}

// UpsertPilotComment saves or replaces a comment; an empty body removes
// it. Returns the stored row, or nil when removed.
func (r *Repo) UpsertPilotComment(ctx context.Context, sessionID, kind, target, body string) (*PilotComment, error) {
	if body == "" {
		if _, err := r.pool.Exec(ctx, `
    DELETE FROM pilot_comments WHERE session_id = $1::uuid AND target_kind = $2 AND target_id = $3`,
			sessionID, kind, target); err != nil {
			return nil, fmt.Errorf("delete pilot comment: %w", err)
		}
		return nil, nil
	}
	c := &PilotComment{TargetKind: kind, TargetID: target, Body: body}
	err := r.pool.QueryRow(ctx, `
    INSERT INTO pilot_comments (tenant_id, session_id, target_kind, target_id, body)
    VALUES ($1, $2::uuid, $3, $4, $5)
    ON CONFLICT (session_id, target_kind, target_id) DO UPDATE SET body = EXCLUDED.body
    RETURNING updated_at`, tenant.FromContext(ctx).Int64(), sessionID, kind, target, body).Scan(&c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert pilot comment: %w", err)
	}
	return c, nil
}

// ListPilotComments returns a session's comments, by kind then target.
func (r *Repo) ListPilotComments(ctx context.Context, sessionID string) ([]PilotComment, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT target_kind, target_id, body, updated_at FROM pilot_comments
     WHERE session_id = $1::uuid ORDER BY target_kind, target_id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list pilot comments: %w", err)
	}
	defer rows.Close()
	var out []PilotComment
	for rows.Next() {
		var c PilotComment
		if err := rows.Scan(&c.TargetKind, &c.TargetID, &c.Body, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan pilot comment: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListPilotSessionSummaries is the admin list: every session, newest
// first, with the participant and the counts the list shows.
func (r *Repo) ListPilotSessionSummaries(ctx context.Context) ([]PilotSessionSummary, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT `+pilotSessionCols+`,
           u.name, u.email,
           (SELECT min(at) FROM pilot_section_events e WHERE e.session_id = s.id AND e.event = 'opened'),
           (SELECT max(at) FROM pilot_section_events e WHERE e.session_id = s.id AND e.event = 'submitted'),
           (SELECT count(*) FROM pilot_comments c WHERE c.session_id = s.id)::int,
           (SELECT count(*) FROM pilot_answers a WHERE a.session_id = s.id AND a.flagged)::int
      FROM pilot_sessions s JOIN users u ON u.id = s.user_id
     WHERE s.tenant_id = $1
     ORDER BY s.created_at DESC`, tenant.FromContext(ctx).Int64())
	if err != nil {
		return nil, fmt.Errorf("list pilot sessions: %w", err)
	}
	defer rows.Close()
	var out []PilotSessionSummary
	for rows.Next() {
		var m PilotSessionSummary
		s := &m.Session
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.InstrumentVersion, &s.RoleBranch, &s.Status, &s.ConsentAt,
			&s.RecordingOK, &s.SurveyFinishedAt, &s.RecordFinishedAt, &s.WorkflowFinishedAt,
			&s.FinishedAt, &s.SelfReportedMinutes, &s.MostWantedToSkip, &s.CreatedAt,
			&m.Name, &m.Email, &m.FirstOpened, &m.LastSubmit, &m.CommentCount, &m.FlagCount,
		); err != nil {
			return nil, fmt.Errorf("scan pilot summary: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
