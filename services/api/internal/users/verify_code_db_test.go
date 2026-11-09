package users

import (
	"bytes"
	"testing"
	"time"
)

// The code path resolves to the same token the link carries, so one
// consumption serves both; and five wrong codes burn the token, link
// included. Run against Postgres because the CASE in the UPDATE and
// the RETURNING are invisible to the compiler.
func TestVerifyCodeHelpersAgainstPostgres(t *testing.T) {
	r, ctx := dtTestRepo(t)
	var userID int64
	if err := r.pool.QueryRow(ctx, `
    INSERT INTO users (email, name, status, role)
    VALUES ('verify-' || gen_random_uuid()::text || '@example.test', 'Verify Tester', 'unverified', 'member')
    RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("make user: %v", err)
	}
	t.Cleanup(func() { _, _ = r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID) })

	tokenHash := []byte("token-hash-for-the-link-0123456789")
	codeHash := []byte("code-hash-for-the-digits-012345678")
	if _, err := r.CreateToken(ctx, userID, PurposeVerify, tokenHash, codeHash, time.Hour); err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	live, err := r.LiveVerifyTokens(ctx, userID)
	if err != nil || len(live) != 1 {
		t.Fatalf("LiveVerifyTokens: %d tokens, err %v", len(live), err)
	}
	if !bytes.Equal(live[0].TokenHash, tokenHash) || !bytes.Equal(live[0].CodeHash, codeHash) || live[0].Attempts != 0 {
		t.Fatalf("live token lacks its hashes: %+v", live[0])
	}
	for i := 1; i <= 4; i++ {
		remaining, err := r.FailVerifyAttempt(ctx, userID, 5)
		if err != nil || remaining != 1 {
			t.Fatalf("attempt %d: remaining=%d err=%v, want 1 live", i, remaining, err)
		}
	}
	remaining, err := r.FailVerifyAttempt(ctx, userID, 5)
	if err != nil || remaining != 0 {
		t.Fatalf("fifth attempt: remaining=%d err=%v, want 0", remaining, err)
	}
	if live, _ := r.LiveVerifyTokens(ctx, userID); len(live) != 0 {
		t.Fatalf("a burned token is still live: %+v", live)
	}
	// The link in the same email is dead too: that is the token.
	if _, err := r.ConsumeToken(ctx, tokenHash, PurposeVerify); err != ErrTokenExpired {
		t.Fatalf("ConsumeToken after burn: err = %v, want ErrTokenExpired", err)
	}
	// A fresh token resolved by code is consumed through the link's path.
	tokenHash2 := []byte("token-hash-two-0123456789abcdefghij")
	if _, err := r.CreateToken(ctx, userID, PurposeVerify, tokenHash2, codeHash, time.Hour); err != nil {
		t.Fatal(err)
	}
	live, _ = r.LiveVerifyTokens(ctx, userID)
	if len(live) != 1 {
		t.Fatalf("want one live token, got %d", len(live))
	}
	if got, err := r.ConsumeToken(ctx, live[0].TokenHash, PurposeVerify); err != nil || got != userID {
		t.Fatalf("ConsumeToken via the code's token hash: user %d err %v", got, err)
	}
}
