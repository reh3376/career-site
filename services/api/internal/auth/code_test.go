package auth

import (
	"strings"
	"testing"
)

// The forgot-password code is five digits (FSD FR-AUTH-19) and must be
// exactly that: a leading zero kept, nothing but digits, and a hash
// that matches HashToken on the same string so the handler can compare
// what the member typed against what was stored.
func TestNewDigitCodeIsExactlyNDigits(t *testing.T) {
	for i := 0; i < 200; i++ {
		code, hash, err := NewDigitCode(5)
		if err != nil {
			t.Fatalf("NewDigitCode: %v", err)
		}
		if len(code) != 5 {
			t.Fatalf("code %q is not 5 characters", code)
		}
		for _, r := range code {
			if r < '0' || r > '9' {
				t.Fatalf("code %q has a non-digit", code)
			}
		}
		if string(hash) != string(HashToken(code)) {
			t.Fatalf("hash of %q does not match HashToken", code)
		}
	}
}

func TestNewDigitCodeRefusesSillyLengths(t *testing.T) {
	for _, n := range []int{0, 3, 10} {
		if _, _, err := NewDigitCode(n); err == nil {
			t.Errorf("NewDigitCode(%d) accepted", n)
		}
	}
}

// The agent password is read once by a person and typed once by an
// agent, so it avoids the characters that look alike, and it has to be
// long enough that argon2 accepts it as a password at all.
func TestNewAgentPasswordShapeAndAlphabet(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		pw, err := NewAgentPassword()
		if err != nil {
			t.Fatalf("NewAgentPassword: %v", err)
		}
		if len(pw) != 20 {
			t.Fatalf("password %q is not 20 characters", pw)
		}
		if len(pw) < MinPasswordLen {
			t.Fatalf("password shorter than MinPasswordLen")
		}
		for _, r := range pw {
			if !strings.ContainsRune(agentPasswordAlphabet, r) {
				t.Fatalf("password %q uses %q, outside the alphabet", pw, r)
			}
			if strings.ContainsRune("0O1lI", r) {
				t.Fatalf("password %q uses a look-alike character %q", pw, r)
			}
		}
		if seen[pw] {
			t.Fatalf("password repeated: %q", pw)
		}
		seen[pw] = true
		if _, err := HashPassword(pw); err != nil {
			t.Fatalf("HashPassword rejected an agent password: %v", err)
		}
	}
}
