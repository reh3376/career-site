package auth

import (
	"strings"
	"testing"
)

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("hash missing argon2id prefix: %s", hash)
	}
	if err := VerifyPassword("correct-horse-battery-staple", hash); err != nil {
		t.Fatalf("VerifyPassword good: %v", err)
	}
	if err := VerifyPassword("wrong-horse", hash); err == nil {
		t.Fatalf("VerifyPassword bad: expected mismatch, got nil")
	}
}

func TestHashPasswordTooShort(t *testing.T) {
	if _, err := HashPassword("short"); err == nil {
		t.Fatalf("expected short-password error")
	}
}

func TestNewTokenLengthAndHashStability(t *testing.T) {
	tok, hash, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	if len(tok) < 40 {
		t.Fatalf("token too short: %d", len(tok))
	}
	if len(hash) != 32 {
		t.Fatalf("hash len = %d, want 32", len(hash))
	}
	got := HashToken(tok)
	for i := range hash {
		if hash[i] != got[i] {
			t.Fatalf("HashToken drift at byte %d", i)
		}
	}
}

func TestNewCodeIsSixDigits(t *testing.T) {
	code, hash, err := NewCode()
	if err != nil {
		t.Fatalf("NewCode: %v", err)
	}
	if len(code) != 6 {
		t.Fatalf("code = %q, want 6 chars", code)
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			t.Fatalf("code %q has non-digit", code)
		}
	}
	if len(hash) != 32 {
		t.Fatalf("hash len = %d, want 32", len(hash))
	}
}
