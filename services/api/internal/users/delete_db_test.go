package users

import (
	"testing"
	"time"
)

// The cascades are what make DeleteUser one statement: a session row,
// a verification token and a pilot sitting go with the account. An
// admin row stays. Run against Postgres because ON DELETE is invisible
// to the compiler.
func TestDeleteUserCascadesAgainstPostgres(t *testing.T) {
	r, ctx := dtTestRepo(t)
	var userID, adminID int64
	if err := r.pool.QueryRow(ctx, `
    INSERT INTO users (email, name, status, role, pilot_approved)
    VALUES ('delete-me-' || gen_random_uuid()::text || '@example.test', 'To Delete', 'active', 'member', true)
    RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("make user: %v", err)
	}
	t.Cleanup(func() { _, _ = r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID) })
	if err := r.pool.QueryRow(ctx, `
    INSERT INTO users (email, name, status, role)
    VALUES ('keep-admin-' || gen_random_uuid()::text || '@example.test', 'Kept Admin', 'active', 'admin')
    RETURNING id`).Scan(&adminID); err != nil {
		t.Fatalf("make admin: %v", err)
	}
	t.Cleanup(func() { _, _ = r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, adminID) })

	if _, err := r.CreateToken(ctx, userID, PurposeVerify, []byte("tok-delete-0123456789abcdef"), []byte("code-delete-0123"), time.Hour); err != nil {
		t.Fatal(err)
	}
	const version = "test-delete-v0.0"
	if err := r.EnsurePilotInstrument(ctx, version, []byte(`{"version":"test-delete-v0.0","sections":[]}`)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = r.pool.Exec(ctx, `DELETE FROM pilot_instruments WHERE version = $1`, version) })
	s, err := r.StartPilotSession(ctx, userID, version, false)
	if err != nil {
		t.Fatal(err)
	}

	email, err := r.DeleteUser(ctx, userID)
	if err != nil || email == "" {
		t.Fatalf("DeleteUser: email=%q err=%v", email, err)
	}
	var n int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE id = $1`, userID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("user row survived: n=%d err=%v", n, err)
	}
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM email_tokens WHERE user_id = $1`, userID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("tokens survived: n=%d err=%v", n, err)
	}
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM pilot_sessions WHERE id = $1::uuid`, s.ID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("pilot session survived: n=%d err=%v", n, err)
	}
	if _, err := r.DeleteUser(ctx, userID); err != ErrNotFound {
		t.Fatalf("second delete: err = %v, want ErrNotFound", err)
	}
	if _, err := r.DeleteUser(ctx, adminID); err != ErrNotFound {
		t.Fatalf("deleting an admin must be refused by the query: err = %v", err)
	}
}
