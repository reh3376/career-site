// Package secrets encrypts values that have to survive in the database
// and still be usable later.
//
// Everything the application has kept until now was either hashed
// (passwords, which are only ever compared) or signed (decision tokens,
// which are only ever verified). A Google refresh token is neither: it
// must come back out in plaintext to be sent to Google, so it has to be
// encrypted rather than hashed, and that is a different problem with
// sharper edges.
//
// AES-256-GCM, which authenticates as well as encrypts, so a row edited
// in the database fails to open rather than decrypting to something
// else. Random nonce per seal, stored alongside the ciphertext, because
// reusing a nonce with the same key in GCM is catastrophic rather than
// merely weak.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// Errors callers distinguish.
var (
	// ErrNoKey means no key is configured, so nothing can be sealed or
	// opened. Separate from a decrypt failure because the fix is
	// different: set the key, rather than suspect the data.
	ErrNoKey = errors.New("no encryption key is configured")
	// ErrKeySize means the key was not 32 bytes after decoding.
	ErrKeySize = errors.New("the encryption key must be 32 bytes (AES-256)")
	// ErrMalformed means the stored value is not in the expected shape.
	ErrMalformed = errors.New("the sealed value is malformed")
	// ErrOpen means authentication failed: wrong key, or the row was
	// tampered with. Deliberately the same error for both, since
	// telling them apart is useful to an attacker and to nobody else.
	ErrOpen = errors.New("the sealed value could not be opened")
)

// sealedPrefix version-tags the format. A later change (a different
// cipher, a rotated key scheme) can be recognised rather than guessed
// at, and an unversioned blob is rejected instead of being fed to the
// wrong opener.
const sealedPrefix = "v1"

// Key is an AES-256 key.
type Key []byte

// ParseKey decodes a base64 (standard or url, padded or not) key and
// checks its length. Returns nil and no error when s is empty, which is
// the "not configured" case a caller may legitimately allow.
func ParseKey(s string) (Key, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	var raw []byte
	var err error
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding,
		base64.URLEncoding, base64.RawURLEncoding,
	} {
		if raw, err = enc.DecodeString(s); err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("decode the encryption key: %w", err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("%w, got %d", ErrKeySize, len(raw))
	}
	return Key(raw), nil
}

// NewKey generates a key, for `openssl rand`-free local setup and for
// tests. The caller is responsible for storing it somewhere the
// application can read and a repository cannot.
func NewKey() (Key, error) {
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		return nil, fmt.Errorf("generate an encryption key: %w", err)
	}
	return Key(k), nil
}

// String renders the key for configuration. Deliberately not a Stringer
// on Key itself: an accidental %v of a key in a log line is exactly the
// accident this package exists to avoid, so exporting it has to be a
// thing the caller typed on purpose.
func Encode(k Key) string { return base64.StdEncoding.EncodeToString(k) }

// Sealer seals and opens values under one key.
type Sealer struct {
	aead cipher.AEAD
}

// New builds a Sealer. A nil or empty key returns a usable zero Sealer
// whose methods report ErrNoKey, so a caller can wire it
// unconditionally and fail at the point of use with a clear reason
// rather than at boot with a nil pointer.
func New(k Key) (*Sealer, error) {
	if len(k) == 0 {
		return &Sealer{}, nil
	}
	if len(k) != 32 {
		return nil, fmt.Errorf("%w, got %d", ErrKeySize, len(k))
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, fmt.Errorf("build the cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("build GCM: %w", err)
	}
	return &Sealer{aead: aead}, nil
}

// Enabled reports whether a key is configured.
func (s *Sealer) Enabled() bool { return s != nil && s.aead != nil }

// Seal encrypts plaintext and returns a storable string:
//
//	v1.<base64url nonce>.<base64url ciphertext+tag>
//
// context is authenticated but not encrypted. Passing something stable
// and specific, such as "google_refresh_token", binds the ciphertext to
// its purpose: a value lifted from one column and dropped into another
// fails to open rather than being silently accepted somewhere it was
// never meant to be used.
func (s *Sealer) Seal(plaintext []byte, context string) (string, error) {
	if !s.Enabled() {
		return "", ErrNoKey
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate a nonce: %w", err)
	}
	ct := s.aead.Seal(nil, nonce, plaintext, []byte(context))
	return strings.Join([]string{
		sealedPrefix,
		base64.RawURLEncoding.EncodeToString(nonce),
		base64.RawURLEncoding.EncodeToString(ct),
	}, "."), nil
}

// Open reverses Seal. The context must match the one used to seal.
func (s *Sealer) Open(sealed, context string) ([]byte, error) {
	if !s.Enabled() {
		return nil, ErrNoKey
	}
	parts := strings.Split(sealed, ".")
	if len(parts) != 3 || parts[0] != sealedPrefix {
		return nil, ErrMalformed
	}
	nonce, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(nonce) != s.aead.NonceSize() {
		return nil, ErrMalformed
	}
	ct, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrMalformed
	}
	out, err := s.aead.Open(nil, nonce, ct, []byte(context))
	if err != nil {
		// Never wrapped: the underlying error distinguishes a bad key
		// from a tampered ciphertext, and that distinction is useful
		// only to someone attacking the store.
		return nil, ErrOpen
	}
	return out, nil
}
