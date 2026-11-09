package users

import (
	"context"
	"fmt"
)

// The code half of email verification (FSD FR-AUTH-03). The email has
// always carried a six-digit code beside the link; until 2026-11-09
// nothing on the server resolved it, and the owner's live review found
// the page had nowhere to enter it. These mirror the reset-by-code
// helpers in pilot_access.go for purpose 'verify', and return the
// token hash as well, so a matched code is consumed through the same
// ConsumeToken the link uses and the two paths cannot diverge.

// LiveVerifyTokens returns a user's unused, unexpired verification
// tokens with their token and code hashes, newest first. There is
// normally one; a re-sent verification adds another.
func (r *Repo) LiveVerifyTokens(ctx context.Context, userID int64) ([]EmailToken, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT id, user_id, purpose, token_hash, code_hash, attempts, expires_at, used_at, created_at
      FROM email_tokens
     WHERE user_id = $1 AND purpose = 'verify' AND used_at IS NULL AND expires_at > now()
     ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("live verify tokens: %w", err)
	}
	defer rows.Close()
	var out []EmailToken
	for rows.Next() {
		var t EmailToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.Purpose, &t.TokenHash, &t.CodeHash, &t.Attempts,
			&t.ExpiresAt, &t.UsedAt, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan verify token: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// FailVerifyAttempt counts a wrong code against every live verification
// token of the user and burns any that reach the cap. Burning the token
// also kills the link in the same email, which is the point: a code
// being guessed at is a token that should stop working. Returns how
// many tokens are still live afterwards.
func (r *Repo) FailVerifyAttempt(ctx context.Context, userID int64, maxAttempts int) (int, error) {
	rows, err := r.pool.Query(ctx, `
    UPDATE email_tokens
       SET attempts = attempts + 1,
           used_at = CASE WHEN attempts + 1 >= $2 THEN now() ELSE used_at END
     WHERE user_id = $1 AND purpose = 'verify' AND used_at IS NULL AND expires_at > now()
    RETURNING used_at IS NULL`, userID, maxAttempts)
	if err != nil {
		return 0, fmt.Errorf("fail verify attempt: %w", err)
	}
	defer rows.Close()
	live := 0
	for rows.Next() {
		var stillLive bool
		if err := rows.Scan(&stillLive); err != nil {
			return 0, fmt.Errorf("scan verify attempt: %w", err)
		}
		if stillLive {
			live++
		}
	}
	return live, rows.Err()
}
