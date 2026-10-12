package users

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// The audit_log table has existed since migration 00001 and received its
// first row on 2026-10-12, when FR-ADM-09 was finally built. Nothing in
// here alters the schema; the columns were always right, there was just
// never a writer.
//
// Rows are append-only. There is no update and no delete, deliberately:
// an audit trail that can be edited answers a different question from
// the one anybody asks it. The same reasoning is written out at length
// in migration 00056 for dt_review_events, which is the working
// precedent this follows.

// AuditEntry is one recorded admin action.
type AuditEntry struct {
	ID int64
	// ActorUserID is the admin who made the call. Nil for a system
	// action. The column is ON DELETE SET NULL, so a row outlives the
	// person: the action still happened.
	ActorUserID *int64
	ActorName   string
	// Action is the Connect procedure, e.g.
	// "/career.v1.AdminService/SetMemberStatus". Stored whole rather
	// than prettified so it can be matched back to the contract without
	// a translation table that could drift.
	Action     string
	TargetType string
	TargetID   string
	Payload    map[string]any
	IPHash     []byte
	OccurredAt time.Time
}

// WriteAuditEntry appends one row.
//
// Callers are expected to log and continue on error rather than fail the
// request: the interceptor writes this after the handler has returned,
// so the action has already happened and failing the call now would tell
// the admin a change did not land when it did, inviting a retry that
// does it twice. The cost of that choice is that a database problem
// loses audit rows silently unless somebody reads the error log, which
// is why the interceptor logs it at ERROR with the action name.
func (r *Repo) WriteAuditEntry(ctx context.Context, e AuditEntry) error {
	payload := e.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	blob, err := json.Marshal(payload)
	if err != nil {
		// Should not happen for the shapes the interceptor builds, but
		// a payload is never worth losing the row over.
		blob = []byte(`{"_payload_error":"marshal failed"}`)
	}
	const q = `
    INSERT INTO audit_log
      (actor_user_id, action, target_type, target_id, payload_json, ip_hash)
    VALUES ($1, $2, $3, $4, $5, $6)
  `
	if _, err := r.pool.Exec(ctx, q,
		e.ActorUserID, e.Action, e.TargetType, e.TargetID, blob, e.IPHash,
	); err != nil {
		return fmt.Errorf("insert audit_log: %w", err)
	}
	return nil
}

// AuditQuery filters ListAuditEntries.
type AuditQuery struct {
	ActorUserID  *int64
	ActionPrefix string
	TargetID     string
	From         *time.Time
	To           *time.Time
	Limit        int
	Offset       int
}

// ListAuditEntries returns rows newest first, with the actor's name
// joined in so the console does not re-query per row.
func (r *Repo) ListAuditEntries(ctx context.Context, q AuditQuery) ([]AuditEntry, int, error) {
	if q.Limit <= 0 || q.Limit > 200 {
		q.Limit = 50
	}
	const base = `
    FROM audit_log a
    LEFT JOIN users u ON u.id = a.actor_user_id
    WHERE ($1::bigint IS NULL OR a.actor_user_id = $1)
      AND ($2 = '' OR a.action LIKE $2 || '%')
      AND ($3 = '' OR a.target_id = $3)
      AND ($4::timestamptz IS NULL OR a.occurred_at >= $4)
      AND ($5::timestamptz IS NULL OR a.occurred_at <= $5)
  `
	args := []any{q.ActorUserID, q.ActionPrefix, q.TargetID, q.From, q.To}

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) `+base, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit_log: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
    SELECT a.id, a.actor_user_id, COALESCE(u.name, ''), a.action,
           a.target_type, a.target_id, a.payload_json, a.occurred_at `+
		base+` ORDER BY a.occurred_at DESC, a.id DESC LIMIT $6 OFFSET $7`,
		append(args, q.Limit, q.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("select audit_log: %w", err)
	}
	defer rows.Close()

	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var blob []byte
		if err := rows.Scan(&e.ID, &e.ActorUserID, &e.ActorName, &e.Action,
			&e.TargetType, &e.TargetID, &blob, &e.OccurredAt); err != nil {
			return nil, 0, fmt.Errorf("scan audit_log: %w", err)
		}
		if len(blob) > 0 {
			_ = json.Unmarshal(blob, &e.Payload)
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}
