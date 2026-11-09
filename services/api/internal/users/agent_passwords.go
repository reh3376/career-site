package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/reh3376/career-site/services/api/internal/tenant"
)

// AgentPassword is one one-time password for the agent account
// (FSD FR-AUTH-20, migration 00064). Only the argon2 hash is stored; the
// plaintext is shown to the admin once, in the response that created
// it, and never again.
type AgentPassword struct {
	ID           int64
	PasswordHash string
	Reason       string
	CreatedBy    *int64
	CreatedAt    time.Time
	ExpiresAt    time.Time
	UsedAt       *time.Time
	SessionID    *int64
	RevokedAt    *time.Time
	RevokedBy    *int64
	// SessionLive says whether the session this password minted is
	// still usable. Read from a join on sessions; false when unused.
	SessionLive bool
}

// State is what the console shows: revoked beats used beats expired
// beats live, because a revoked password is unusable whatever else is
// true of it.
func (p AgentPassword) State(now time.Time) string {
	switch {
	case p.RevokedAt != nil:
		return "revoked"
	case p.UsedAt != nil:
		return "used"
	case !now.Before(p.ExpiresAt):
		return "expired"
	default:
		return "live"
	}
}

const agentPasswordCols = `
    ap.id, ap.password_hash, ap.reason, ap.created_by, ap.created_at, ap.expires_at,
    ap.used_at, ap.session_id, ap.revoked_at, ap.revoked_by,
    coalesce(s.revoked_at IS NULL AND s.expires_at > now(), false) AS session_live
`

func scanAgentPassword(row pgx.Row) (*AgentPassword, error) {
	p := &AgentPassword{}
	err := row.Scan(
		&p.ID, &p.PasswordHash, &p.Reason, &p.CreatedBy, &p.CreatedAt, &p.ExpiresAt,
		&p.UsedAt, &p.SessionID, &p.RevokedAt, &p.RevokedBy, &p.SessionLive,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan agent password: %w", err)
	}
	return p, nil
}

// CreateAgentPassword stores a new one-time password and revokes every
// live one first, so at most one works at a time: the owner's rule is
// that creating a password restarts the fifteen-minute clock and
// nothing older can still be used.
//
// Two statements rather than a transaction. The only race is two admins
// creating at the same instant, which leaves two live passwords for one
// window; the next creation closes both, and the record shows what
// happened. Not worth a transaction on a table this small.
func (r *Repo) CreateAgentPassword(
	ctx context.Context,
	passwordHash, reason string,
	createdBy int64,
	ttl time.Duration,
) (*AgentPassword, error) {
	t := tenant.FromContext(ctx).Int64()
	if _, err := r.pool.Exec(ctx, `
    UPDATE agent_passwords
       SET revoked_at = now(), revoked_by = $2
     WHERE tenant_id = $1 AND used_at IS NULL AND revoked_at IS NULL AND expires_at > now()`,
		t, createdBy); err != nil {
		return nil, fmt.Errorf("revoke live agent passwords: %w", err)
	}
	row := r.pool.QueryRow(ctx, `
    WITH ins AS (
      INSERT INTO agent_passwords (tenant_id, password_hash, reason, created_by, expires_at)
      VALUES ($1, $2, $3, $4, now() + $5::interval)
      RETURNING *
    )
    SELECT `+agentPasswordCols+`
      FROM ins ap LEFT JOIN sessions s ON s.id = ap.session_id`,
		t, passwordHash, reason, createdBy, ttl.String())
	p, err := scanAgentPassword(row)
	if err != nil {
		return nil, fmt.Errorf("create agent password: %w", err)
	}
	return p, nil
}

// LiveAgentPasswords returns the passwords that could still sign an
// agent in: unused, unrevoked, unexpired. Normally zero or one rows.
// The hashes are included because this is what the login compares
// against; nothing else reads them.
func (r *Repo) LiveAgentPasswords(ctx context.Context) ([]AgentPassword, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT `+agentPasswordCols+`
      FROM agent_passwords ap LEFT JOIN sessions s ON s.id = ap.session_id
     WHERE ap.tenant_id = $1 AND ap.used_at IS NULL AND ap.revoked_at IS NULL AND ap.expires_at > now()
     ORDER BY ap.created_at DESC`, tenant.FromContext(ctx).Int64())
	if err != nil {
		return nil, fmt.Errorf("live agent passwords: %w", err)
	}
	defer rows.Close()
	var out []AgentPassword
	for rows.Next() {
		p, err := scanAgentPassword(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// UseAgentPassword consumes a password for the session it minted.
// Returns ErrNotFound when the row was no longer live, which is how a
// second sign-in racing the first learns it lost.
func (r *Repo) UseAgentPassword(ctx context.Context, id, sessionID int64, ipHash []byte) error {
	tag, err := r.pool.Exec(ctx, `
    UPDATE agent_passwords
       SET used_at = now(), session_id = $2, used_ip_hash = $3
     WHERE id = $1 AND used_at IS NULL AND revoked_at IS NULL AND expires_at > now()`,
		id, sessionID, ipHash)
	if err != nil {
		return fmt.Errorf("use agent password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RevokeAgentPassword ends a live password by hand. A password that is
// already used, expired or revoked is left as it is and returned so
// the console can say which.
func (r *Repo) RevokeAgentPassword(ctx context.Context, id, by int64) (*AgentPassword, error) {
	if _, err := r.pool.Exec(ctx, `
    UPDATE agent_passwords
       SET revoked_at = now(), revoked_by = $2
     WHERE id = $1 AND used_at IS NULL AND revoked_at IS NULL`, id, by); err != nil {
		return nil, fmt.Errorf("revoke agent password: %w", err)
	}
	return r.GetAgentPassword(ctx, id)
}

// GetAgentPassword reads one row.
func (r *Repo) GetAgentPassword(ctx context.Context, id int64) (*AgentPassword, error) {
	return scanAgentPassword(r.pool.QueryRow(ctx, `
    SELECT `+agentPasswordCols+`
      FROM agent_passwords ap LEFT JOIN sessions s ON s.id = ap.session_id
     WHERE ap.id = $1`, id))
}

// ListAgentPasswords returns the most recent passwords, newest first,
// with whether each one's session is still live.
func (r *Repo) ListAgentPasswords(ctx context.Context, limit int) ([]AgentPassword, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
    SELECT `+agentPasswordCols+`
      FROM agent_passwords ap LEFT JOIN sessions s ON s.id = ap.session_id
     WHERE ap.tenant_id = $1
     ORDER BY ap.created_at DESC
     LIMIT $2`, tenant.FromContext(ctx).Int64(), limit)
	if err != nil {
		return nil, fmt.Errorf("list agent passwords: %w", err)
	}
	defer rows.Close()
	var out []AgentPassword
	for rows.Next() {
		p, err := scanAgentPassword(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// CountLiveSessions says how many usable sessions a user holds. The
// agent console shows it beside the revoke-all button.
func (r *Repo) CountLiveSessions(ctx context.Context, userID int64) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
    SELECT count(*) FROM sessions
     WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now()`, userID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count live sessions: %w", err)
	}
	return n, nil
}
