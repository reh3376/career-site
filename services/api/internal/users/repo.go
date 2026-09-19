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
	StatusDisabled        Status = "disabled"
	StatusDeleted         Status = "deleted"
)

type Role string

const (
	RoleMember Role = "member"
	RoleAdmin  Role = "admin"
)

type User struct {
	ID              int64
	Email           string
	Name            string
	Organization    string
	StatedRole      string
	Status          Status
	Role            Role
	ConsentVersion  string
	ConsentAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type EmailTokenPurpose string

const (
	PurposeVerify       EmailTokenPurpose = "verify"
	PurposeReset        EmailTokenPurpose = "reset"
	PurposeChangeEmail  EmailTokenPurpose = "change_email"
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
    consent_version, consent_at, created_at, updated_at
`

func scanUser(row pgx.Row) (*User, error) {
	u := &User{}
	err := row.Scan(
		&u.ID, &u.Email, &u.Name, &u.Organization, &u.StatedRole,
		&u.Status, &u.Role,
		&u.ConsentVersion, &u.ConsentAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return u, nil
}

func (r *Repo) GetByEmail(ctx context.Context, email string) (*User, error) {
	return scanUser(r.pool.QueryRow(ctx, "SELECT "+selectCols+" FROM users WHERE email = $1", email))
}

func (r *Repo) GetByID(ctx context.Context, id int64) (*User, error) {
	return scanUser(r.pool.QueryRow(ctx, "SELECT "+selectCols+" FROM users WHERE id = $1", id))
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
