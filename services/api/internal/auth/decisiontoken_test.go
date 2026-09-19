package auth

import (
	"strings"
	"testing"
	"time"
)

func testSecret(t *testing.T) []byte {
	t.Helper()
	// Deterministic 32-byte secret; content is irrelevant, only length.
	return []byte("0123456789abcdef0123456789abcdef")
}

func TestSignVerifyRoundTrip(t *testing.T) {
	secret := testSecret(t)
	tok, err := SignDecision(secret, 42, DecisionApprove, time.Hour)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if strings.Count(tok, ".") != 4 {
		t.Fatalf("token shape wrong: %q", tok)
	}
	got, err := VerifyDecision(secret, tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.UserID != 42 || got.Decision != DecisionApprove {
		t.Fatalf("Verify payload wrong: %+v", got)
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	secret := testSecret(t)
	tok, err := SignDecision(secret, 1, DecisionApprove, time.Hour)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	// Flip the decision to "decline" without re-signing.
	tampered := strings.Replace(tok, "approve", "decline", 1)
	if _, err := VerifyDecision(secret, tampered); err == nil {
		t.Fatal("tampered token accepted")
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	tok, err := SignDecision(testSecret(t), 1, DecisionApprove, time.Hour)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	other := []byte("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")
	if _, err := VerifyDecision(other, tok); err == nil {
		t.Fatal("token accepted under a different secret")
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	tok, err := SignDecision(testSecret(t), 1, DecisionApprove, -time.Second)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if _, err := VerifyDecision(testSecret(t), tok); err == nil {
		t.Fatal("expired token accepted")
	}
}

func TestSignRejectsShortSecret(t *testing.T) {
	if _, err := SignDecision([]byte("tooshort"), 1, DecisionApprove, time.Hour); err == nil {
		t.Fatal("short-secret Sign should error")
	}
}
