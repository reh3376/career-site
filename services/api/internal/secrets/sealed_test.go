package secrets

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func mustSealer(t *testing.T) *Sealer {
	t.Helper()
	k, err := NewKey()
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	s, err := New(k)
	if err != nil {
		t.Fatalf("sealer: %v", err)
	}
	return s
}

func TestRoundTrip(t *testing.T) {
	s := mustSealer(t)
	secret := []byte("1//0gRefreshTokenFromGoogle")

	sealed, err := s.Seal(secret, "google_refresh_token")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	// The whole point: it must come back out, unlike a password hash.
	got, err := s.Open(sealed, "google_refresh_token")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if string(got) != string(secret) {
		t.Errorf("got %q, want %q", got, secret)
	}
}

// The stored form must not contain the plaintext anywhere, which is the
// one property a reader of the database cares about.
func TestSealedValueDoesNotLeakPlaintext(t *testing.T) {
	s := mustSealer(t)
	secret := "1//0gRefreshTokenFromGoogle"
	sealed, err := s.Seal([]byte(secret), "google_refresh_token")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if strings.Contains(sealed, secret) {
		t.Fatal("the sealed value contains the plaintext")
	}
	if !strings.HasPrefix(sealed, "v1.") {
		t.Errorf("missing the version tag: %q", sealed)
	}
}

// Reusing a nonce under one key is the classic way GCM is broken, so
// two seals of the same value must differ.
func TestEachSealUsesAFreshNonce(t *testing.T) {
	s := mustSealer(t)
	a, err := s.Seal([]byte("same"), "ctx")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	b, err := s.Seal([]byte("same"), "ctx")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if a == b {
		t.Fatal("two seals of the same plaintext were identical, so the nonce is being reused")
	}
}

// GCM authenticates, so an edited row must fail to open rather than
// decrypt to something else.
func TestTamperedCiphertextFailsToOpen(t *testing.T) {
	s := mustSealer(t)
	sealed, err := s.Seal([]byte("refresh-token"), "ctx")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	parts := strings.Split(sealed, ".")
	// Flip a character in the ciphertext.
	ct := []byte(parts[2])
	if ct[0] == 'A' {
		ct[0] = 'B'
	} else {
		ct[0] = 'A'
	}
	parts[2] = string(ct)

	if _, err := s.Open(strings.Join(parts, "."), "ctx"); !errors.Is(err, ErrOpen) {
		t.Errorf("got %v, want ErrOpen", err)
	}
}

// The context binds a ciphertext to its purpose, so a value lifted from
// one column into another must not open.
func TestWrongContextFailsToOpen(t *testing.T) {
	s := mustSealer(t)
	sealed, err := s.Seal([]byte("refresh-token"), "google_refresh_token")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if _, err := s.Open(sealed, "some_other_secret"); !errors.Is(err, ErrOpen) {
		t.Errorf("got %v, want ErrOpen: the context is not being authenticated", err)
	}
}

func TestAnotherKeyCannotOpen(t *testing.T) {
	a, b := mustSealer(t), mustSealer(t)
	sealed, err := a.Seal([]byte("refresh-token"), "ctx")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if _, err := b.Open(sealed, "ctx"); !errors.Is(err, ErrOpen) {
		t.Errorf("got %v, want ErrOpen", err)
	}
}

func TestMalformedValuesAreRejected(t *testing.T) {
	s := mustSealer(t)
	for _, bad := range []string{
		"", "nonsense", "v1.only-two", "v2.abc.def",
		"v1.!!!.abc", "v1.abc.!!!",
	} {
		if _, err := s.Open(bad, "ctx"); err == nil {
			t.Errorf("%q opened; want a refusal", bad)
		}
	}
}

// No key is a different situation from a failed decrypt: the fix is to
// configure a key, not to suspect the data.
func TestWithoutAKeyBothDirectionsSayWhy(t *testing.T) {
	s, err := New(nil)
	if err != nil {
		t.Fatalf("New(nil): %v", err)
	}
	if s.Enabled() {
		t.Error("reports enabled with no key")
	}
	if _, err := s.Seal([]byte("x"), "ctx"); !errors.Is(err, ErrNoKey) {
		t.Errorf("seal: got %v, want ErrNoKey", err)
	}
	if _, err := s.Open("v1.a.b", "ctx"); !errors.Is(err, ErrNoKey) {
		t.Errorf("open: got %v, want ErrNoKey", err)
	}
}

func TestKeysOfTheWrongSizeAreRefused(t *testing.T) {
	if _, err := New(Key(make([]byte, 16))); !errors.Is(err, ErrKeySize) {
		t.Errorf("got %v, want ErrKeySize", err)
	}
	if _, err := ParseKey("c2hvcnQ="); !errors.Is(err, ErrKeySize) {
		t.Errorf("ParseKey: got %v, want ErrKeySize", err)
	}
}

// An unset key is allowed to parse as "not configured", so the api can
// boot without one and refuse only at the point of use.
func TestEmptyKeyParsesAsNotConfigured(t *testing.T) {
	k, err := ParseKey("   ")
	if err != nil {
		t.Fatalf("ParseKey: %v", err)
	}
	if k != nil {
		t.Errorf("got %v, want nil", k)
	}
}

// Configuration is copied by hand, so every base64 flavour a person
// might paste has to work.
func TestParseKeyAcceptsEveryBase64Flavour(t *testing.T) {
	k, err := NewKey()
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	for name, encoded := range map[string]string{
		"std":     base64.StdEncoding.EncodeToString(k),
		"raw std": base64.RawStdEncoding.EncodeToString(k),
		"url":     base64.URLEncoding.EncodeToString(k),
		"raw url": base64.RawURLEncoding.EncodeToString(k),
	} {
		got, err := ParseKey(encoded)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if string(got) != string(k) {
			t.Errorf("%s: round trip changed the key", name)
		}
	}
}
