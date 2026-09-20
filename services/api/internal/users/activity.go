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
