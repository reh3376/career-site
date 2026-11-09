package users

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// DeleteUser removes an account outright (FSD FR-ADM-25). Every
// foreign key to users is ON DELETE CASCADE or SET NULL (checked
// 2026-11-09 across all 36 of them), so one statement takes the
// sessions, tokens and pilot sitting with it and leaves a null where
// the member was in rows that record history. An admin row is never
// deleted by this path; the handler refuses first and the WHERE clause
// refuses again. Returns the address the account had.
func (r *Repo) DeleteUser(ctx context.Context, id int64) (string, error) {
	var email string
	err := r.pool.QueryRow(ctx, `
    DELETE FROM users WHERE id = $1 AND role <> 'admin' RETURNING email`, id).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("delete user: %w", err)
	}
	return email, nil
}
