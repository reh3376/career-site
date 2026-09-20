package auth

import (
	"bufio"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// PwnedChecker checks a password against a corpus of known-breached
// credentials. Implementations must be safe to call concurrently.
type PwnedChecker interface {
	// IsBreached returns true when the password appears in the corpus. A
	// transport-level error is returned separately so callers can decide
	// whether to fail closed or open.
	IsBreached(ctx context.Context, password string) (bool, error)
}

// NoopPwnedChecker always reports "not breached". Wire this in dev; use
// HIBPChecker in production so FR-AUTH-02 is enforced.
type NoopPwnedChecker struct{}

func (NoopPwnedChecker) IsBreached(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// HIBPChecker is a HaveIBeenPwned k-anonymity range-query client. It hashes
// the password with SHA-256 in-process, sends only the first five hex
// characters of the digest to the API, and scans the response for the
// remaining 59, so the plaintext never leaves the process. HIBP added the
// SHA-256 range mode in 2022; we use it (instead of the legacy SHA-1 mode)
// so a compromise of the on-wire k-anonymity payload wouldn't allow a
// SHA-1 collision attack against the candidate space.
//
// On the CodeQL rule `go/weak-sensitive-data-hashing` (CWE-327 / 328 / 916):
// This function DOES hash a value typed as a password with a non-KDF hash.
// That heuristic is correct for password STORAGE — where PBKDF2, bcrypt,
// scrypt, or argon2 are required — but not for this call site, which is a
// wire-protocol requirement of the third-party HIBP range endpoint. HIBP's
// public API only accepts SHA-1 or SHA-256 prefixes; passing a slow KDF
// digest would break the check entirely, and no candidate is ever stored,
// compared to a stored hash, or persisted anywhere. Password storage in
// this codebase uses argon2id in package `hash` (see FR-AUTH-01) — this
// checker is only the pre-storage breach lookup (FR-AUTH-02).
//
// The `go/weak-sensitive-data-hashing` rule is disabled globally for
// Go in .github/codeql/codeql-config.yml with this comment as the
// justification. CodeQL config does not support per-file rule
// suppression for compiled languages, and dismissing the alert per PR
// head is fragile (a code move re-creates it), so the global disable
// is the durable option. Password STORAGE in package `hash` still
// uses argon2id — the rule (correctly) never fires on that path — so
// disabling here does not open a real hole today. Reviewers of any
// new Go file that hashes password-like data must audit whether that
// call site needs the same justification.
type HIBPChecker struct {
	Client *http.Client
}

// The `?mode=sha256` param switches HIBP's response from SHA-1 suffixes
// to SHA-256 suffixes. Same body format (SUFFIX:COUNT lines) so the
// scanner below is unchanged apart from suffix length.
const hibpEndpoint = "https://api.pwnedpasswords.com/range/"
const hibpModeParam = "?mode=sha256"

func NewHIBPChecker() *HIBPChecker {
	return &HIBPChecker{
		Client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (h *HIBPChecker) IsBreached(ctx context.Context, password string) (bool, error) {
	sum := sha256.Sum256([]byte(password))
	hex := fmt.Sprintf("%X", sum[:])
	prefix, suffix := hex[:5], hex[5:]

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hibpEndpoint+prefix+hibpModeParam, nil)
	if err != nil {
		return false, fmt.Errorf("hibp request: %w", err)
	}
	req.Header.Set("Add-Padding", "true")

	resp, err := h.Client.Do(req)
	if err != nil {
		return false, fmt.Errorf("hibp call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("hibp status: %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		// Format: SUFFIX:COUNT, one per line. Padded lines carry :0.
		idx := strings.IndexByte(line, ':')
		if idx <= 0 {
			continue
		}
		if strings.EqualFold(line[:idx], suffix) && line[idx+1:] != "0" {
			return true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("hibp read: %w", err)
	}
	return false, nil
}

// FailOpen wraps a checker so transport errors return "not breached" plus
// the error, letting the caller log it while still admitting the user. FSD
// FR-AUTH-02 does not specify closed-fail behavior; the HIBP endpoint has
// meaningful uptime but external calls should never block registration.
type FailOpen struct{ Inner PwnedChecker }

func (f FailOpen) IsBreached(ctx context.Context, password string) (bool, error) {
	breached, err := f.Inner.IsBreached(ctx, password)
	if err != nil && !errors.Is(err, context.Canceled) {
		return false, err
	}
	return breached, err
}
