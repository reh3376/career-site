package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
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
