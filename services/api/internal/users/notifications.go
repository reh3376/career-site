package users

import (
	"context"
	"fmt"
	"time"

	"github.com/reh3376/career-site/services/api/internal/email"
)

// NotificationDelivery is one audited email attempt for a member.
type NotificationDelivery struct {
	ID          int64
	Kind        string
	Recipient   string
	Provider    string
	TriggeredBy string
	DurationMs  int64
	Error       *string // nil on success
	CreatedAt   time.Time
}

// RecordDelivery implements email.DeliverySink. Inserts the audit row
// and, when the mail is tied to a member, stamps users.last_notification_*
// so the cheap single-row read on the member list stays accurate.
func (r *Repo) RecordDelivery(ctx context.Context, d email.Delivery) error {
	var userID *int64
	if d.UserID != 0 {
		userID = &d.UserID
	}
	const ins = `
    INSERT INTO notification_deliveries
      (user_id, kind, recipient, provider, triggered_by, duration_ms, error)
    VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''))
  `
	if _, err := r.pool.Exec(ctx, ins,
		userID, d.Kind, d.Recipient, d.Provider, d.TriggeredBy, d.DurationMs, d.Error,
	); err != nil {
		return fmt.Errorf("insert delivery: %w", err)
	}
	if userID == nil {
		return nil
	}
	const stamp = `
    UPDATE users
    SET last_notification_kind  = $2,
        last_notification_at    = now(),
        last_notification_error = NULLIF($3, '')
    WHERE id = $1
  `
	if _, err := r.pool.Exec(ctx, stamp, d.UserID, d.Kind, d.Error); err != nil {
		return fmt.Errorf("stamp last notification: %w", err)
	}
	return nil
}

// ListDeliveries returns a member's most recent email attempts, newest
// first, capped at limit.
func (r *Repo) ListDeliveries(ctx context.Context, userID int64, limit int) ([]NotificationDelivery, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	const q = `
    SELECT id, kind, recipient, provider, triggered_by, duration_ms, error, created_at
    FROM notification_deliveries
    WHERE user_id = $1
    ORDER BY created_at DESC, id DESC
    LIMIT $2
  `
	rows, err := r.pool.Query(ctx, q, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list deliveries: %w", err)
	}
	defer rows.Close()
	var out []NotificationDelivery
	for rows.Next() {
		var d NotificationDelivery
		if err := rows.Scan(&d.ID, &d.Kind, &d.Recipient, &d.Provider, &d.TriggeredBy,
			&d.DurationMs, &d.Error, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan delivery: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
