// Package users holds domain types and Postgres queries for the identity
// tables. Hand-written pgx queries for now; a sqlc migration is a follow-up
// task once the query set stabilizes (FSD §8.2 calls for sqlc long-term).
package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound      = errors.New("user not found")
	ErrEmailInUse    = errors.New("email already in use")
	ErrTokenNotFound = errors.New("token not found")
	ErrTokenExpired  = errors.New("token expired or already used")
)

type Status string

const (
	StatusUnverified      Status = "unverified"
	StatusPendingApproval Status = "pending_approval"
	StatusActive          Status = "active"
	StatusDeclined        Status = "declined"
	StatusExpired         Status = "expired"
	StatusDisabled        Status = "disabled"
	StatusDeleted         Status = "deleted"
)

type Role string

const (
	RoleMember Role = "member"
	RoleAdmin  Role = "admin"
)

type User struct {
	ID             int64
	Email          string
	Name           string
	Organization   string
	StatedRole     string
	Status         Status
	Role           Role
	ConsentVersion string
	ConsentAt      *time.Time
	ExpiresAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	// Set by RecordNotification when a notification email is
	// dispatched. LastNotificationError is nil on success and holds
	// the truncated provider error on failure — the admin UI shows
	// a green pill when nil, a red one when set.
	LastNotificationKind  string
	LastNotificationAt    *time.Time
	LastNotificationError *string
}

type EmailTokenPurpose string

const (
	PurposeVerify      EmailTokenPurpose = "verify"
	PurposeReset       EmailTokenPurpose = "reset"
	PurposeChangeEmail EmailTokenPurpose = "change_email"
)

type EmailToken struct {
	ID        int64
	UserID    int64
	Purpose   EmailTokenPurpose
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

type CreateInput struct {
	Email          string
	PasswordHash   string
	Name           string
	Organization   string
	StatedRole     string
	ConsentVersion string
}

type Repo struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// Create inserts a new user in `unverified` state.
func (r *Repo) Create(ctx context.Context, in CreateInput) (*User, error) {
	const q = `
    INSERT INTO users (email, password_hash, name, organization, stated_role,
                       consent_version, consent_at, status)
    VALUES ($1, $2, $3, $4, $5, $6, now(), 'unverified')
    RETURNING id, created_at, updated_at
  `
	u := &User{
		Email:          in.Email,
		Name:           in.Name,
		Organization:   in.Organization,
		StatedRole:     in.StatedRole,
		ConsentVersion: in.ConsentVersion,
		Status:         StatusUnverified,
		Role:           RoleMember,
	}
	err := r.pool.QueryRow(ctx, q,
		in.Email, in.PasswordHash, in.Name,
		in.Organization, in.StatedRole, in.ConsentVersion,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err, "users_email_key") {
			return nil, ErrEmailInUse
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}
	now := time.Now().UTC()
	u.ConsentAt = &now
	return u, nil
}

const selectCols = `
    id, email, name, organization, stated_role, status, role,
    consent_version, consent_at, expires_at, created_at, updated_at,
    last_notification_kind, last_notification_at, last_notification_error
`

func scanUser(row pgx.Row) (*User, error) {
	u := &User{}
	// pgx returns a NULL text column as an empty string when we scan
	// into a *string via sql.NullString semantics. Wrap the three
	// nullable notification columns so the caller can distinguish
	// "never sent" from "sent successfully".
	var (
		kind    *string
		at      *time.Time
		errText *string
	)
	err := row.Scan(
		&u.ID, &u.Email, &u.Name, &u.Organization, &u.StatedRole,
		&u.Status, &u.Role,
		&u.ConsentVersion, &u.ConsentAt, &u.ExpiresAt, &u.CreatedAt, &u.UpdatedAt,
		&kind, &at, &errText,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	if kind != nil {
		u.LastNotificationKind = *kind
	}
	u.LastNotificationAt = at
	u.LastNotificationError = errText
	return u, nil
}

func (r *Repo) GetByEmail(ctx context.Context, email string) (*User, error) {
	return scanUser(r.pool.QueryRow(ctx, "SELECT "+selectCols+" FROM users WHERE email = $1", email))
}

func (r *Repo) GetByID(ctx context.Context, id int64) (*User, error) {
	return scanUser(r.pool.QueryRow(ctx, "SELECT "+selectCols+" FROM users WHERE id = $1", id))
}

// EnsureAdmin upserts a row for the given email in role=admin,
// status=active. On insert, the passwordHash is stored as-is. On subsequent
// boots, the hash is refreshed when it differs so a rotated password takes
// effect after a redeploy. Every other field is left alone so notes and
// activity on the admin's own row survive re-runs.
func (r *Repo) EnsureAdmin(ctx context.Context, email, name, passwordHash string) error {
	const q = `
    INSERT INTO users (email, password_hash, name, role, status, consent_version, consent_at)
    VALUES ($1, $2, $3, 'admin', 'active', 'bootstrap', now())
    ON CONFLICT (email) DO UPDATE
      SET password_hash = EXCLUDED.password_hash,
          role          = 'admin',
          status        = 'active'
      WHERE users.password_hash IS DISTINCT FROM EXCLUDED.password_hash
         OR users.role          <> 'admin'
         OR users.status        <> 'active'
  `
	_, err := r.pool.Exec(ctx, q, email, passwordHash, name)
	if err != nil {
		return fmt.Errorf("ensure admin: %w", err)
	}
	return nil
}

// PasswordHash returns the stored Argon2 PHC-encoded hash for a user, or
// an empty string when the user has no password (OAuth-only account).
// Used only by the Login handler; never exposed outside the package.
func (r *Repo) PasswordHash(ctx context.Context, id int64) string {
	var hash *string
	if err := r.pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, id).Scan(&hash); err != nil {
		return ""
	}
	if hash == nil {
		return ""
	}
	return *hash
}

func (r *Repo) SetStatus(ctx context.Context, id int64, status Status) error {
	const q = `UPDATE users SET status = $2 WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id, string(status))
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetPasswordHash overwrites a user's password hash. Used by the
// reset flow after the caller has presented a valid single-use
// reset token.
func (r *Repo) SetPasswordHash(ctx context.Context, id int64, hash string) error {
	const q = `UPDATE users SET password_hash = $2 WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id, hash)
	if err != nil {
		return fmt.Errorf("set password hash: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RecordNotification stamps the last-notification columns for a user
// after we hand a message to the mail provider. Kind is a short slug
// (e.g. "user_approved"). errText should be the truncated provider
// error on failure, or empty on success. The write is best-effort:
// callers should not fail the request if this returns an error, since
// the email itself has already gone (or already failed) — the audit
// info is a nice-to-have.
func (r *Repo) RecordNotification(
	ctx context.Context, userID int64, kind string, errText string,
) error {
	const q = `
    UPDATE users
    SET last_notification_kind  = $2,
        last_notification_at    = now(),
        last_notification_error = NULLIF($3, '')
    WHERE id = $1
  `
	_, err := r.pool.Exec(ctx, q, userID, kind, errText)
	if err != nil {
		return fmt.Errorf("record notification: %w", err)
	}
	return nil
}

// CreateToken inserts an email token and returns the persisted row.
func (r *Repo) CreateToken(
	ctx context.Context,
	userID int64,
	purpose EmailTokenPurpose,
	tokenHash, codeHash []byte,
	ttl time.Duration,
) (*EmailToken, error) {
	const q = `
    INSERT INTO email_tokens (user_id, purpose, token_hash, code_hash, expires_at)
    VALUES ($1, $2, $3, $4, now() + $5::interval)
    RETURNING id, expires_at, created_at
  `
	t := &EmailToken{UserID: userID, Purpose: purpose}
	err := r.pool.QueryRow(ctx, q, userID, string(purpose), tokenHash, codeHash, ttl.String()).
		Scan(&t.ID, &t.ExpiresAt, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert token: %w", err)
	}
	return t, nil
}

// ConsumeToken looks up a token by its hash, checks purpose + expiry + used,
// and atomically marks it used. Returns the associated user id.
func (r *Repo) ConsumeToken(
	ctx context.Context,
	tokenHash []byte,
	purpose EmailTokenPurpose,
) (int64, error) {
	const q = `
    UPDATE email_tokens
    SET used_at = now()
    WHERE token_hash = $1
      AND purpose = $2
      AND used_at IS NULL
      AND expires_at > now()
    RETURNING user_id
  `
	var userID int64
	err := r.pool.QueryRow(ctx, q, tokenHash, string(purpose)).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrTokenExpired
	}
	if err != nil {
		return 0, fmt.Errorf("consume token: %w", err)
	}
	return userID, nil
}

// isUniqueViolation checks whether err is a Postgres unique-constraint
// violation matching the given constraint name.
func isUniqueViolation(err error, constraint string) bool {
	// pgx returns *pgconn.PgError; imported lazily to avoid a hard dep here.
	type sqlErr interface {
		SQLState() string
	}
	if e, ok := err.(sqlErr); ok && e.SQLState() == "23505" {
		return true
	}
	// Fallback: string match on constraint name in the error text.
	return constraint != "" && errorContains(err, constraint)
}

func errorContains(err error, sub string) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for i := 0; i+len(sub) <= len(msg); i++ {
		if msg[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ListMembersFilter narrows a ListMembers call. Zero values disable
// each dimension.
type ListMembersFilter struct {
	Query  string // ILIKE substring on name, email, or organization
	Status Status // "" (or StatusUnspecified) = any
	Limit  int32  // 1..200 (clamped)
	Offset int32  // >= 0
}

// ListMembersResult is a page of users plus aggregate counts per
// status so the admin console can render a summary bar. Total is the
// filtered count (without limit/offset); the counts are unfiltered so
// the tabs always show the true totals.
type ListMembersResult struct {
	Members     []User
	Total       int32
	CountByStat map[Status]int32
}

// ListMembers returns a page of users plus per-status aggregate
// counts. Ordering: NEWEST first (created_at DESC). Filters are
// parameter-bound so there's no injection surface.
func (r *Repo) ListMembers(ctx context.Context, f ListMembersFilter) (*ListMembersResult, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	// Aggregate counts per status, always across the full table so
	// the header tabs show real totals regardless of the filter.
	countRows, err := r.pool.Query(ctx, `
    SELECT status::text, COUNT(*) FROM users GROUP BY status
  `)
	if err != nil {
		return nil, fmt.Errorf("count members: %w", err)
	}
	defer countRows.Close()
	counts := map[Status]int32{}
	for countRows.Next() {
		var s string
		var n int32
		if err := countRows.Scan(&s, &n); err != nil {
			return nil, fmt.Errorf("scan count: %w", err)
		}
		counts[Status(s)] = n
	}
	if err := countRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate counts: %w", err)
	}

	// Build filter WHERE
	where := "WHERE 1=1"
	args := []any{}
	next := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	if f.Query != "" {
		p := next("%" + f.Query + "%")
		where += " AND (name ILIKE " + p + " OR email::text ILIKE " + p +
			" OR organization ILIKE " + p + ")"
	}
	if f.Status != "" {
		where += " AND status = " + next(string(f.Status)) + "::user_status"
	}

	// Filtered total (for pagination header).
	var total int32
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM users "+where, args...,
	).Scan(&total); err != nil {
		return nil, fmt.Errorf("count filtered: %w", err)
	}

	// Paged select.
	limArg := next(f.Limit)
	offArg := next(f.Offset)
	q := "SELECT " + selectCols + " FROM users " + where +
		" ORDER BY created_at DESC LIMIT " + limArg + " OFFSET " + offArg
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("select members: %w", err)
	}
	defer rows.Close()

	members := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, *u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate members: %w", err)
	}
	return &ListMembersResult{
		Members:     members,
		Total:       total,
		CountByStat: counts,
	}, nil
}
