package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Session struct {
	ID           int64
	UserID       int64
	ExpiresAt    time.Time
	LastActiveAt time.Time
	RevokedAt    *time.Time
	CreatedAt    time.Time
}

// CreateSession stores a session row keyed by the SHA-256 of the plaintext
// token. Callers keep the plaintext for the Set-Cookie header and never
// persist it.
func (r *Repo) CreateSession(
	ctx context.Context,
	userID int64,
	tokenHash []byte,
	ttl time.Duration,
	ipHash []byte,
	userAgent string,
) (*Session, error) {
	const q = `
    INSERT INTO sessions (user_id, token_hash, expires_at, ip_hash, user_agent)
    VALUES ($1, $2, now() + $3::interval, $4, $5)
    RETURNING id, expires_at, last_active_at, created_at
  `
	s := &Session{UserID: userID}
	err := r.pool.QueryRow(ctx, q, userID, tokenHash, ttl.String(), ipHash, userAgent).
		Scan(&s.ID, &s.ExpiresAt, &s.LastActiveAt, &s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}
	return s, nil
}

// LookupSessionUser resolves a session token to its owning user. Returns
// ErrNotFound when the token has no matching live row (missing, expired,
// or revoked). Bumps last_active_at as a side effect.
func (r *Repo) LookupSessionUser(ctx context.Context, tokenHash []byte) (*User, error) {
	const q = `
    UPDATE sessions
    SET last_active_at = now()
    WHERE token_hash = $1
      AND revoked_at IS NULL
      AND expires_at > now()
    RETURNING user_id
  `
	var userID int64
	err := r.pool.QueryRow(ctx, q, tokenHash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("session lookup: %w", err)
	}
	return r.GetByID(ctx, userID)
}

// RevokeSession marks a single session revoked. No-op when the token has
// no matching row (already revoked or never existed).
func (r *Repo) RevokeSession(ctx context.Context, tokenHash []byte) error {
	const q = `UPDATE sessions SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`
	_, err := r.pool.Exec(ctx, q, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}
