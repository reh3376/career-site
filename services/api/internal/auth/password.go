// Package auth handles password hashing, token minting, and breached-password
// checks. Parameters follow OWASP guidance for Argon2id as of 2024:
//
//	time = 2, memory = 64 MiB, threads = 1, salt = 16 B, key = 32 B.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    = 2
	argonMemory  = 64 * 1024
	argonThreads = 1
	argonSaltLen = 16
	argonKeyLen  = 32

	// MinPasswordLen matches FSD FR-AUTH-02.
	MinPasswordLen = 12
)

var ErrBadPassword = errors.New("password does not match")

// HashPassword returns the PHC-string encoding of an Argon2id hash of pw.
// Format: $argon2id$v=19$m=65536,t=2,p=1$<salt-b64>$<hash-b64>.
func HashPassword(pw string) (string, error) {
	if len(pw) < MinPasswordLen {
		return "", fmt.Errorf("password shorter than %d characters", MinPasswordLen)
	}
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("salt: %w", err)
	}
	hash := argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// VerifyPassword compares pw against a stored PHC-encoded hash in constant
// time. Returns ErrBadPassword when the two do not match.
func VerifyPassword(pw, encoded string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return errors.New("hash format not recognised")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return fmt.Errorf("version: %w", err)
	}
	var memory uint32
	var time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return fmt.Errorf("params: %w", err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return fmt.Errorf("salt: %w", err)
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return fmt.Errorf("hash: %w", err)
	}
	got := argon2.IDKey([]byte(pw), salt, time, memory, threads, uint32(len(want)))
	if subtle.ConstantTimeCompare(want, got) != 1 {
		return ErrBadPassword
	}
	return nil
}
