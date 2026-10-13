package users

import (
	"context"
	"fmt"
	"time"
)

// ActivityEvent is one row of activity_events. Fields not populated
// by a given kind stay at their zero value.
type ActivityEvent struct {
	ID             int64
	UserID         int64
	Kind           string
	ContentID      string
	ConversationID string
	SessionID      *int64
	Query          string
	Variant        string
	DwellMs        int32
	ClientEventID  string
	OccurredAt     time.Time
	CreatedAt      time.Time
}

// ActivityCounts is the aggregate shape rendered on the member card
// in the admin console. All fields are 0 when the user has no
// recorded activity.
type ActivityCounts struct {
	Views        int32
	Downloads    int32
	ChatMessages int32
	Escalations  int32
	Saved        int32
}

// RecordActivity inserts one event. When ClientEventID is non-empty
// and a row with that (user_id, client_event_id) already exists, the
// insert is a no-op (ON CONFLICT DO NOTHING) and returned=false —
// callers get idempotent client-side retries for free.
//
// Server-recorded events (login, logout, chat) generally pass an
// empty ClientEventID; the partial unique index skips them so they
// always insert.
func (r *Repo) RecordActivity(ctx context.Context, e ActivityEvent) (bool, error) {
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	const q = `
    INSERT INTO activity_events (
      user_id, kind, content_id, conversation_id, session_id,
      query, variant, dwell_ms, client_event_id, occurred_at
    ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''), $10)
    ON CONFLICT (user_id, client_event_id)
      WHERE client_event_id IS NOT NULL
      DO NOTHING
    RETURNING id
  `
	var id int64
	err := r.pool.QueryRow(ctx, q,
		e.UserID, e.Kind, nullIfEmpty(e.ContentID), nullIfEmpty(e.ConversationID),
		e.SessionID, nullIfEmpty(e.Query), nullIfEmpty(e.Variant),
		nullIfZero32(e.DwellMs), e.ClientEventID, e.OccurredAt,
	).Scan(&id)
	if err != nil {
		// pgx returns ErrNoRows when the ON CONFLICT DO NOTHING path
		// suppressed the RETURNING. That's a de-dup hit, not a
		// failure.
		if err.Error() == "no rows in result set" {
			return false, nil
		}
		return false, fmt.Errorf("insert activity: %w", err)
	}
	return true, nil
}

// RecentActivity returns up to `limit` most-recent events for a
// user, newest first. Used by the admin member detail page.
func (r *Repo) RecentActivity(ctx context.Context, userID int64, limit int) ([]ActivityEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	const q = `
    SELECT id, user_id, kind,
           COALESCE(content_id, ''), COALESCE(conversation_id, ''),
           session_id,
           COALESCE(query, ''), COALESCE(variant, ''),
           COALESCE(dwell_ms, 0),
           COALESCE(client_event_id, ''),
           occurred_at, created_at
    FROM activity_events
    WHERE user_id = $1
    ORDER BY occurred_at DESC
    LIMIT $2
  `
	rows, err := r.pool.Query(ctx, q, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent activity: %w", err)
	}
	defer rows.Close()
	var out []ActivityEvent
	for rows.Next() {
		var e ActivityEvent
		if err := rows.Scan(
			&e.ID, &e.UserID, &e.Kind,
			&e.ContentID, &e.ConversationID,
			&e.SessionID,
			&e.Query, &e.Variant,
			&e.DwellMs,
			&e.ClientEventID,
			&e.OccurredAt, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan activity: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ActivityCountsFor returns per-kind counts across all time for a
// user. Populates the MemberCounts pill on /admin/registrations.
func (r *Repo) ActivityCountsFor(ctx context.Context, userID int64) (*ActivityCounts, error) {
	const q = `
    SELECT
      COUNT(*) FILTER (WHERE kind = 'view')     ::int AS views,
      COUNT(*) FILTER (WHERE kind = 'download') ::int AS downloads,
      COUNT(*) FILTER (WHERE kind = 'chat')     ::int AS chat_messages,
      COUNT(*) FILTER (WHERE kind = 'escalate') ::int AS escalations,
      COUNT(*) FILTER (WHERE kind = 'save')     ::int AS saved
    FROM activity_events
    WHERE user_id = $1
  `
	c := &ActivityCounts{}
	if err := r.pool.QueryRow(ctx, q, userID).Scan(
		&c.Views, &c.Downloads, &c.ChatMessages, &c.Escalations, &c.Saved,
	); err != nil {
		return nil, fmt.Errorf("counts: %w", err)
	}
	return c, nil
}

// MemberActivitySummary is one row of the /admin/activity aggregate
// view: everything a sortable overview table needs to render a user's
// engagement without a per-user round-trip.
type MemberActivitySummary struct {
	UserID          int64
	Name            string
	Email           string
	Status          Status
	TotalSessions   int32
	TotalActiveSecs int64 // sum of (LEAST(revoked_at, last_active_at, now) − created_at)
	AskRogerCount   int32
	TotalEvents     int32
	LastKind        string     // empty when the user has no events yet
	LastEventAt     *time.Time // NULL when no events
}

// ActivitySort picks the ORDER BY column for ListMemberActivity.
type ActivitySort int

const (
	ActivitySortName ActivitySort = iota
	ActivitySortLastEventDesc
	ActivitySortSessionsDesc
	ActivitySortActiveSecsDesc
	ActivitySortAskRogerDesc
)

// ListMemberActivity returns the /admin/activity aggregate: one row
// per user with counts + last-event + active-time, JOINed across
// users, sessions, and activity_events. LEFT JOIN so a member with
// zero events / sessions still appears (with zeros).
//
// The active-time expression sums per-session (LEAST(revoked_at,
// last_active_at, now()) − created_at) which is a good proxy for
// wall-clock time the user was reachable in that session.
func (r *Repo) ListMemberActivity(ctx context.Context, sort ActivitySort) ([]MemberActivitySummary, error) {
	order := "u.name COLLATE \"C\" ASC"
	switch sort {
	case ActivitySortLastEventDesc:
		order = "last_event_at DESC NULLS LAST, u.name ASC"
	case ActivitySortSessionsDesc:
		order = "total_sessions DESC, u.name ASC"
	case ActivitySortActiveSecsDesc:
		order = "total_active_secs DESC, u.name ASC"
	case ActivitySortAskRogerDesc:
		order = "ask_roger_count DESC, u.name ASC"
	}
	// The two aggregates over sessions / events are computed in
	// subqueries so the outer LEFT JOIN doesn't multiply rows.
	// COALESCE keeps the zero-event / zero-session case clean.
	q := `
    SELECT
      u.id, u.name, u.email, u.status,
      COALESCE(s.total_sessions,   0)::int    AS total_sessions,
      COALESCE(s.total_active_secs, 0)::bigint AS total_active_secs,
      COALESCE(a.ask_roger_count,  0)::int    AS ask_roger_count,
      COALESCE(a.total_events,     0)::int    AS total_events,
      COALESCE(a.last_kind, '')                AS last_kind,
      a.last_event_at
    FROM users u
    LEFT JOIN LATERAL (
      SELECT
        COUNT(*)                                                                             AS total_sessions,
        EXTRACT(EPOCH FROM SUM(LEAST(COALESCE(revoked_at, last_active_at, now()), now()) - created_at))
          AS total_active_secs
      FROM sessions
      WHERE user_id = u.id
    ) s ON true
    LEFT JOIN LATERAL (
      SELECT
        COUNT(*)                                          AS total_events,
        COUNT(*) FILTER (WHERE kind = 'chat')             AS ask_roger_count,
        (ARRAY_AGG(kind        ORDER BY occurred_at DESC))[1] AS last_kind,
        MAX(occurred_at)                                  AS last_event_at
      FROM activity_events
      WHERE user_id = u.id
    ) a ON true
    ORDER BY ` + order + `
    LIMIT 500
  `
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list member activity: %w", err)
	}
	defer rows.Close()
	var out []MemberActivitySummary
	for rows.Next() {
		var m MemberActivitySummary
		if err := rows.Scan(
			&m.UserID, &m.Name, &m.Email, &m.Status,
			&m.TotalSessions, &m.TotalActiveSecs,
			&m.AskRogerCount, &m.TotalEvents,
			&m.LastKind, &m.LastEventAt,
		); err != nil {
			return nil, fmt.Errorf("scan member activity: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullIfZero32(n int32) any {
	if n == 0 {
		return nil
	}
	return int(n)
}

// WelcomeBack is the summary /home shows a returning member
// (FR-HIST-02).
type WelcomeBack struct {
	// LastVisitAt is when the previous visit ended, nil on a first
	// visit or when the member has no earlier session.
	LastVisitAt *time.Time
	// PreviousVisits counts visits before this one, where a visit is a
	// run of activity with no gap longer than 30 minutes. See
	// WelcomeBackFor on why this is not activity_events.session_id.
	PreviousVisits int32
	// RecentlyViewed holds up to three content ids, most recent first.
	// These are the `id` from each content file's front matter, so the
	// web app resolves titles and links from its own content; the API
	// deliberately does not learn what an article is called.
	RecentlyViewed []string
}

// homeContentID is the beacon's id for the member home page itself.
// It is excluded from "recently viewed" because telling somebody the
// last thing they looked at was the page they are reading is noise, and
// for a long time it was the only instrumented page, so including it
// would have made the panel show nothing else.
const homeContentID = "/home"

// WelcomeBackFor computes the panel in one round trip.
//
// A visit is a run of activity with no gap longer than visitGap, worked
// out from occurred_at. It is NOT activity_events.session_id, which was
// the obvious choice and the wrong one: that column exists on the table
// and **nothing has ever written to it**. All 272 production rows carry
// NULL, so counting distinct session_id returned zero visits and the
// panel rendered nothing at all.
//
// That bug shipped on 2026-10-12 and was caught the same day by looking
// at the live surface. It passed its own test because the test inserted
// rows with session_id set, which is to say it was verified against
// data shaped to match the assumption rather than against the data that
// exists. Recording it here because the mistake is easy to repeat: a
// column being present is not evidence that it is populated.
//
// Counting login events instead would over-count anyone who signs in
// twice in one sitting. The gap rule treats that as one visit, which is
// what a person means by the word.
func (r *Repo) WelcomeBackFor(ctx context.Context, userID int64) (WelcomeBack, error) {
	var out WelcomeBack

	// 30 minutes is the usual sessionisation window and matches how the
	// quiet check already reasons about a decision test being "in
	// flight". The current visit is the newest run; "previous" is every
	// other one, and "last visit" is when the newest of those ended.
	const visits = `
    WITH e AS (
      SELECT occurred_at,
             lag(occurred_at) OVER (ORDER BY occurred_at) AS prev
        FROM activity_events
       WHERE user_id = $1
    ), marked AS (
      SELECT occurred_at,
             CASE WHEN prev IS NULL OR occurred_at - prev > interval '30 minutes'
                  THEN 1 ELSE 0 END AS is_start
        FROM e
    ), grouped AS (
      SELECT occurred_at, sum(is_start) OVER (ORDER BY occurred_at) AS visit_no
        FROM marked
    ), visits AS (
      SELECT visit_no, max(occurred_at) AS ended_at FROM grouped GROUP BY visit_no
    ), ranked AS (
      SELECT ended_at, row_number() OVER (ORDER BY ended_at DESC) AS rn FROM visits
    )
    SELECT
      (SELECT count(*) FROM ranked WHERE rn > 1),
      (SELECT ended_at FROM ranked WHERE rn = 2)`
	if err := r.pool.QueryRow(ctx, visits, userID).
		Scan(&out.PreviousVisits, &out.LastVisitAt); err != nil {
		return WelcomeBack{}, fmt.Errorf("welcome back visits: %w", err)
	}

	const viewed = `
    SELECT content_id
      FROM activity_events
     WHERE user_id = $1 AND kind = 'view'
       AND content_id IS NOT NULL AND content_id <> '' AND content_id <> $2
     GROUP BY content_id
     ORDER BY max(occurred_at) DESC
     LIMIT 3`
	rows, err := r.pool.Query(ctx, viewed, userID, homeContentID)
	if err != nil {
		return WelcomeBack{}, fmt.Errorf("welcome back viewed: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return WelcomeBack{}, fmt.Errorf("scan viewed: %w", err)
		}
		out.RecentlyViewed = append(out.RecentlyViewed, id)
	}
	return out, rows.Err()
}
