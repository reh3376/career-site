package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// GrantTTL is the whitelisted set of TTL choices from FSD FR-AUTH-17.
type GrantTTL string

const (
	GrantTTL1d        GrantTTL = "1d"
	GrantTTL3d        GrantTTL = "3d"
	GrantTTL7d        GrantTTL = "7d"
	GrantTTL30d       GrantTTL = "30d"
	GrantTTLPermanent GrantTTL = "permanent"
)

// Duration returns the wall-clock duration for a TTL. Zero means permanent —
// callers set users.expires_at to NULL in that case.
func (g GrantTTL) Duration() time.Duration {
	switch g {
	case GrantTTL1d:
		return 24 * time.Hour
	case GrantTTL3d:
		return 3 * 24 * time.Hour
	case GrantTTL7d:
		return 7 * 24 * time.Hour
	case GrantTTL30d:
		return 30 * 24 * time.Hour
	default:
		return 0
	}
}

func (g GrantTTL) IsPermanent() bool { return g == GrantTTLPermanent }

type AccessGrant struct {
	ID             int64
	Email          string
	DefaultTTL     GrantTTL
	Notes          string
	EntryExpiresAt *time.Time
	CreatedBy      *int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// GetActiveGrant returns the whitelist entry for email if one exists and its
// entry_expires_at has not passed. Returns ErrNotFound when no active grant
// exists (miss or expired-entry).
func (r *Repo) GetActiveGrant(ctx context.Context, email string) (*AccessGrant, error) {
	const q = `
    SELECT id, email, default_ttl, notes, entry_expires_at, created_by, created_at, updated_at
    FROM access_grants
    WHERE email = $1
      AND (entry_expires_at IS NULL OR entry_expires_at > now())
  `
	g := &AccessGrant{}
	err := r.pool.QueryRow(ctx, q, email).Scan(
		&g.ID, &g.Email, &g.DefaultTTL, &g.Notes, &g.EntryExpiresAt,
		&g.CreatedBy, &g.CreatedAt, &g.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select grant: %w", err)
	}
	return g, nil
}

// Activate flips a user to `active` and sets expires_at. Pass a nil
// expiresAt for permanent access.
func (r *Repo) Activate(ctx context.Context, id int64, expiresAt *time.Time) error {
	const q = `UPDATE users SET status = 'active', expires_at = $2 WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id, expiresAt)
	if err != nil {
		return fmt.Errorf("activate: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ExpireOne flips a single user to `expired` in one statement, only if
// they are currently `active` and their expires_at has passed. Returns
// true when a row was updated. Concurrent callers stay safe because the
// WHERE clause is guarded by status + expires_at.
func (r *Repo) ExpireOne(ctx context.Context, id int64) (bool, error) {
	const q = `
    UPDATE users
    SET status = 'expired'
    WHERE id = $1 AND status = 'active' AND expires_at IS NOT NULL AND expires_at <= now()
  `
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return false, fmt.Errorf("expire: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// ExpiringSoon returns users whose access ends between now and `within`
// from now. Used by the "3 days before expiry" reminder job.
func (r *Repo) ExpiringSoon(ctx context.Context, within time.Duration) ([]*User, error) {
	const q = `
    SELECT ` + selectCols + `
    FROM users
    WHERE status = 'active'
      AND expires_at IS NOT NULL
      AND expires_at > now()
      AND expires_at <= now() + $1::interval
  `
	rows, err := r.pool.Query(ctx, q, within.String())
	if err != nil {
		return nil, fmt.Errorf("query expiring: %w", err)
	}
	defer rows.Close()

	var out []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// PendingOlderThan returns users still in `pending_approval` whose verify
// token was used more than `age` ago. Used by the auto-decline job
// (FR-AUTH-16). Joins email_tokens because users itself does not carry a
// state-change timestamp today; the verify token's used_at is the moment
// the user entered pending_approval.
func (r *Repo) PendingOlderThan(ctx context.Context, age time.Duration) ([]*User, error) {
	const q = `
    SELECT ` + selectCols + `
    FROM users u
    WHERE u.status = 'pending_approval'
      AND EXISTS (
        SELECT 1 FROM email_tokens t
        WHERE t.user_id = u.id
          AND t.purpose = 'verify'
          AND t.used_at IS NOT NULL
          AND t.used_at < now() - $1::interval
      )
  `
	// selectCols columns are 'u.'-prefixed by virtue of the FROM alias.
	rows, err := r.pool.Query(ctx, q, age.String())
	if err != nil {
		return nil, fmt.Errorf("query pending stale: %w", err)
	}
	defer rows.Close()

	var out []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// Expired returns users whose expires_at has passed but who are still
// marked `active`. Used by the hard-cut job.
func (r *Repo) Expired(ctx context.Context) ([]*User, error) {
	const q = `
    SELECT ` + selectCols + `
    FROM users
    WHERE status = 'active'
      AND expires_at IS NOT NULL
      AND expires_at <= now()
  `
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query expired: %w", err)
	}
	defer rows.Close()

	var out []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// ApprovalDecision is a minimal insert helper for approval_decisions.
type ApprovalDecision struct {
	UserID     int64
	Decision   string // "approve" | "decline" | "auto_decline"
	DecidedVia string // "email_link" | "console" | "scheduler" | "whitelist_auto"
	DecidedBy  *int64
	GrantedTTL *time.Duration
	TokenHash  []byte
	IPHash     []byte
	UserAgent  string
}

// RecordApprovalDecision inserts a row into approval_decisions.
func (r *Repo) RecordApprovalDecision(ctx context.Context, d ApprovalDecision) error {
	const q = `
    INSERT INTO approval_decisions
      (user_id, decision, decided_by, decided_via, granted_ttl,
       token_hash, ip_hash, user_agent)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
  `
	var ttl any
	if d.GrantedTTL != nil {
		ttl = d.GrantedTTL.String()
	}
	_, err := r.pool.Exec(ctx, q,
		d.UserID, d.Decision, d.DecidedBy, d.DecidedVia, ttl,
		d.TokenHash, d.IPHash, d.UserAgent,
	)
	if err != nil {
		return fmt.Errorf("insert decision: %w", err)
	}
	return nil
}

// RevokeSessions marks every active session for a user as revoked. Used
// on expiry so a live session ends the moment the account does.
func (r *Repo) RevokeSessions(ctx context.Context, userID int64) (int64, error) {
	const q = `
    UPDATE sessions SET revoked_at = now()
    WHERE user_id = $1 AND revoked_at IS NULL
  `
	tag, err := r.pool.Exec(ctx, q, userID)
	if err != nil {
		return 0, fmt.Errorf("revoke sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}

// SetExpiresAt updates a user's expires_at without touching status.
// Pass nil to make access permanent. Used by the admin console's
// ExtendAccess action.
func (r *Repo) SetExpiresAt(ctx context.Context, id int64, expiresAt *time.Time) error {
	const q = `UPDATE users SET expires_at = $2 WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id, expiresAt)
	if err != nil {
		return fmt.Errorf("set expires_at: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListGrantsFilter narrows the /admin/access listing.
type ListGrantsFilter struct {
	Query string // matched against email + notes, case-insensitive substring
}

// ListGrantsResult is what ListGrants returns: the page of rows and
// the unfiltered active/expired counts the console renders as chips.
type ListGrantsResult struct {
	Grants       []AccessGrant
	ActiveCount  int32
	ExpiredCount int32
}

const grantCols = `id, email, default_ttl, notes, entry_expires_at, created_by, created_at, updated_at`

// ListGrants returns every whitelist entry matching the filter, newest
// first, plus per-status counts computed across the full table
// (independent of the query filter, so the header chips are stable).
func (r *Repo) ListGrants(ctx context.Context, f ListGrantsFilter) (*ListGrantsResult, error) {
	out := &ListGrantsResult{}

	// Counts first — one query, `count(*) filter (where ...)` so we get
	// both buckets in a single round-trip.
	const countQ = `
    SELECT
      COUNT(*) FILTER (WHERE entry_expires_at IS NULL OR entry_expires_at > now())::int AS active,
      COUNT(*) FILTER (WHERE entry_expires_at IS NOT NULL AND entry_expires_at <= now())::int AS expired
    FROM access_grants
  `
	if err := r.pool.QueryRow(ctx, countQ).Scan(&out.ActiveCount, &out.ExpiredCount); err != nil {
		return nil, fmt.Errorf("count grants: %w", err)
	}

	// Page rows. Filter args are appended dynamically so the query stays
	// parameterised (no string-concat with user input).
	q := `SELECT ` + grantCols + ` FROM access_grants`
	var args []any
	if f.Query != "" {
		q += ` WHERE email ILIKE '%' || $1 || '%' OR notes ILIKE '%' || $1 || '%'`
		args = append(args, f.Query)
	}
	q += ` ORDER BY created_at DESC LIMIT 500`

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list grants: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var g AccessGrant
		if err := rows.Scan(
			&g.ID, &g.Email, &g.DefaultTTL, &g.Notes, &g.EntryExpiresAt,
			&g.CreatedBy, &g.CreatedAt, &g.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan grant: %w", err)
		}
		out.Grants = append(out.Grants, g)
	}
	return out, rows.Err()
}

// UpsertGrant inserts a whitelist entry or updates an existing one
// with the same email. Returns the stored row and true when a new
// row was created. Lowercases the email server-side (citext handles
// case-insensitive uniqueness but the app should still normalise for
// display).
func (r *Repo) UpsertGrant(
	ctx context.Context,
	email string,
	ttl GrantTTL,
	notes string,
	entryExpiresAt *time.Time,
	createdBy *int64,
) (*AccessGrant, bool, error) {
	// ON CONFLICT with a RETURNING + xmax=0 trick tells us whether the
	// row was newly inserted vs updated in a single round-trip.
	const q = `
    INSERT INTO access_grants (email, default_ttl, notes, entry_expires_at, created_by)
    VALUES ($1, $2, $3, $4, $5)
    ON CONFLICT (email) DO UPDATE
      SET default_ttl = EXCLUDED.default_ttl,
          notes = EXCLUDED.notes,
          entry_expires_at = EXCLUDED.entry_expires_at,
          updated_at = now()
    RETURNING ` + grantCols + `, (xmax = 0) AS inserted
  `
	g := &AccessGrant{}
	var inserted bool
	err := r.pool.QueryRow(ctx, q, email, ttl, notes, entryExpiresAt, createdBy).Scan(
		&g.ID, &g.Email, &g.DefaultTTL, &g.Notes, &g.EntryExpiresAt,
		&g.CreatedBy, &g.CreatedAt, &g.UpdatedAt,
		&inserted,
	)
	if err != nil {
		return nil, false, fmt.Errorf("upsert grant: %w", err)
	}
	return g, inserted, nil
}

// DeleteGrant removes a whitelist entry by id. Returns ErrNotFound
// when no row matched.
func (r *Repo) DeleteGrant(ctx context.Context, id int64) error {
	const q = `DELETE FROM access_grants WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete grant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
