package users

import (
	"context"
	"fmt"
)

// SetPilotApproved opens or closes the pilot area to one member
// (FSD FR-PLT-01). Set from the whitelist entry when registration
// auto-approves, and by an admin on the member page.
func (r *Repo) SetPilotApproved(ctx context.Context, id int64, approved bool) error {
	tag, err := r.pool.Exec(ctx, `UPDATE users SET pilot_approved = $2 WHERE id = $1`, id, approved)
	if err != nil {
		return fmt.Errorf("set pilot_approved: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// EnsureAgentUser creates the agent account when it is absent and
// never touches an existing row (FSD FR-AUTH-20). The account has no
// password hash, so the ordinary password path cannot sign it in; only
// a one-time password from agent_passwords can. The role is admin
// because the owner's instruction was access to the entire site, and
// the one-time, fifteen-minute, single-live-password rules are what
// make that acceptable.
func (r *Repo) EnsureAgentUser(ctx context.Context, username, name string) error {
	const q = `
    INSERT INTO users (email, password_hash, name, role, status, consent_version, consent_at)
    VALUES ($1, NULL, $2, 'admin', 'active', 'bootstrap', now())
    ON CONFLICT (email) DO NOTHING
  `
	if _, err := r.pool.Exec(ctx, q, username, name); err != nil {
		return fmt.Errorf("ensure agent user: %w", err)
	}
	return nil
}

// LiveResetTokens returns a user's unused, unexpired reset tokens with
// their code hashes and attempt counts, newest first (FR-AUTH-19). The
// handler compares the submitted code against each in constant time;
// there is normally one.
func (r *Repo) LiveResetTokens(ctx context.Context, userID int64) ([]EmailToken, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT id, user_id, purpose, code_hash, attempts, expires_at, used_at, created_at
      FROM email_tokens
     WHERE user_id = $1 AND purpose = 'reset' AND used_at IS NULL AND expires_at > now()
     ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("live reset tokens: %w", err)
	}
	defer rows.Close()
	var out []EmailToken
	for rows.Next() {
		var t EmailToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.Purpose, &t.CodeHash, &t.Attempts,
			&t.ExpiresAt, &t.UsedAt, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan reset token: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ConsumeResetTokens marks one token used and burns every other live
// reset token of the same user, so a code sent earlier cannot be used
// after a later one succeeded. Returns ErrTokenExpired when the chosen
// token was no longer live.
func (r *Repo) ConsumeResetTokens(ctx context.Context, userID, id int64) error {
	tag, err := r.pool.Exec(ctx, `
    UPDATE email_tokens SET used_at = now()
     WHERE id = $1 AND user_id = $2 AND purpose = 'reset' AND used_at IS NULL AND expires_at > now()`,
		id, userID)
	if err != nil {
		return fmt.Errorf("consume reset token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTokenExpired
	}
	if _, err := r.pool.Exec(ctx, `
    UPDATE email_tokens SET used_at = now()
     WHERE user_id = $1 AND purpose = 'reset' AND used_at IS NULL`, userID); err != nil {
		return fmt.Errorf("burn other reset tokens: %w", err)
	}
	return nil
}

// FailResetAttempt counts a wrong code against every live reset token
// of the user and burns any that reach the cap (FR-AUTH-19: five).
// Returns how many tokens are still live afterwards, so the handler
// can tell the caller to request a new code when it is zero.
func (r *Repo) FailResetAttempt(ctx context.Context, userID int64, maxAttempts int) (int, error) {
	rows, err := r.pool.Query(ctx, `
    UPDATE email_tokens
       SET attempts = attempts + 1,
           used_at = CASE WHEN attempts + 1 >= $2 THEN now() ELSE used_at END
     WHERE user_id = $1 AND purpose = 'reset' AND used_at IS NULL AND expires_at > now()
    RETURNING used_at IS NULL`, userID, maxAttempts)
	if err != nil {
		return 0, fmt.Errorf("fail reset attempt: %w", err)
	}
	defer rows.Close()
	live := 0
	for rows.Next() {
		var stillLive bool
		if err := rows.Scan(&stillLive); err != nil {
			return 0, fmt.Errorf("scan reset attempt: %w", err)
		}
		if stillLive {
			live++
		}
	}
	return live, rows.Err()
}
