package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/big"
)

// NewToken returns a URL-safe random token (32 bytes → 43 base64url chars).
// Callers store only the SHA-256 of the returned string; the plaintext
// travels once, in the URL or code sent to the user.
func NewToken() (plaintext string, hash []byte, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("token: %w", err)
	}
	plaintext = base64.RawURLEncoding.EncodeToString(buf)
	h := sha256.Sum256([]byte(plaintext))
	return plaintext, h[:], nil
}

// HashToken hashes a token the same way NewToken does; used to look up
// tokens by their stored hash.
func HashToken(plaintext string) []byte {
	h := sha256.Sum256([]byte(plaintext))
	return h[:]
}

// NewCode returns a 6-digit numeric code and its SHA-256 hash. Delivered
// alongside the link so a user can paste the code manually.
func NewCode() (plaintext string, hash []byte, err error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("code: %w", err)
	}
	n := (uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])) % 1_000_000
	plaintext = fmt.Sprintf("%06d", n)
	h := sha256.Sum256([]byte(plaintext))
	return plaintext, h[:], nil
}

// NewDigitCode returns an n-digit numeric code and its SHA-256 hash,
// drawn uniformly with crypto/rand. The forgot-password flow uses five
// digits (FSD FR-AUTH-19): short enough to type from a phone while
// reading it on a laptop, which is why the server burns it after five
// wrong attempts and fifteen minutes rather than trusting its length.
func NewDigitCode(n int) (plaintext string, hash []byte, err error) {
	if n < 4 || n > 9 {
		return "", nil, fmt.Errorf("code: %d digits is outside 4..9", n)
	}
	limit := big.NewInt(1)
	for i := 0; i < n; i++ {
		limit.Mul(limit, big.NewInt(10))
	}
	v, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return "", nil, fmt.Errorf("code: %w", err)
	}
	plaintext = fmt.Sprintf("%0*d", n, v.Int64())
	h := sha256.Sum256([]byte(plaintext))
	return plaintext, h[:], nil
}

// agentPasswordAlphabet leaves out the characters that look alike in a
// console font (0 O 1 l I), because the owner reads this once and the
// agent types it once.
//
// The gitleaks marker: the secret scanner's generic rule reads a long
// high-entropy literal assigned near the word "password" as a leaked
// key. This is the alphabet passwords are drawn from, not a password,
// and it is the same 57 characters in every checkout.
const agentPasswordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789" // gitleaks:allow

// NewAgentPassword returns a 20-character one-time password for the
// agent account (FSD FR-AUTH-20): about 115 bits from a 57-character
// alphabet, hashed with argon2 like any other password before storage.
func NewAgentPassword() (string, error) {
	out := make([]byte, 20)
	limit := big.NewInt(int64(len(agentPasswordAlphabet)))
	for i := range out {
		v, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", fmt.Errorf("agent password: %w", err)
		}
		out[i] = agentPasswordAlphabet[v.Int64()]
	}
	return string(out), nil
}
