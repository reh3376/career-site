// Package turnstile verifies Cloudflare Turnstile tokens against the
// siteverify endpoint. It protects the two public endpoints that create
// state without an authenticated caller: registration (FR-AUTH-09) and
// anonymous contact submission (FR-CNT-22).
//
// The failure policy here has two halves, and they are deliberately
// different, because the two failures are not the same kind of event:
//
//   - A missing or malformed secret is a CONFIGURATION error, and the
//     API refuses to boot in production rather than running with the
//     check silently inert (config.Load). That is ADR 0031's rule: a
//     declared control is an enforced control. The precedent is
//     AUTH_INTERCEPTOR_MODE, which sat in .env.prod for a week without
//     ever reaching the container because docker-compose enumerates its
//     environment explicitly, and nothing noticed.
//
//   - An unreachable siteverify endpoint at request time is a TRANSPORT
//     error, and FailOpen admits the request with a logged error. This
//     matches auth.FailOpen around the HaveIBeenPwned lookup: an
//     external call should never be the reason a person cannot register.
//     The exposure that buys is bounded (see docs/fsd-items) — these
//     accounts still land unverified and approval-gated.
package turnstile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SiteVerifyURL is Cloudflare's token validation endpoint.
const SiteVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// Verdict is the outcome of one verification.
type Verdict struct {
	// OK reports whether the request may proceed.
	OK bool
	// ErrorCodes carries Cloudflare's `error-codes` array verbatim when
	// it rejects a token. Useful in logs to tell "the visitor never
	// solved the challenge" (missing-input-response) from "our secret is
	// wrong" (invalid-input-secret), which look identical to the user.
	ErrorCodes []string
	// FailedOpen is true when OK was granted because siteverify could
	// not be reached, rather than because Cloudflare validated the
	// token. Callers log on this; it must never be silent.
	FailedOpen bool
}

// InvalidSecret reports whether Cloudflare rejected our secret rather
// than the visitor's token. This is an operator error, not a visitor
// error, and the caller logs it at a louder level.
func (v Verdict) InvalidSecret() bool {
	for _, c := range v.ErrorCodes {
		if c == "invalid-input-secret" || c == "missing-input-secret" {
			return true
		}
	}
	return false
}

// Verifier checks a Turnstile token. Implementations must be safe to
// call concurrently.
type Verifier interface {
	// Verify reports whether the token is a valid, unspent solution for
	// our site. A transport-level error is returned alongside the
	// verdict so the caller can decide whether to fail closed or open;
	// FailOpen is the wrapper this service uses.
	Verify(ctx context.Context, token, remoteIP string) (Verdict, error)
}

// Noop always reports OK without calling out. It is what dev gets when
// no secret is configured, so `docker compose up` works without a
// Cloudflare account. Production cannot reach this path: config.Load
// refuses to start without a usable secret.
type Noop struct{}

func (Noop) Verify(_ context.Context, _, _ string) (Verdict, error) {
	return Verdict{OK: true}, nil
}

// Client is a siteverify client.
type Client struct {
	secret string
	http   *http.Client
	url    string
}

// New returns a Client posting to Cloudflare with the given secret. The
// timeout is deliberately short: this call sits in the request path of a
// form submission, and FailOpen turns a timeout into an admitted request,
// so a long timeout buys nothing but a slower form.
func New(secret string) *Client {
	return &Client{
		secret: secret,
		url:    SiteVerifyURL,
		http:   &http.Client{Timeout: 5 * time.Second},
	}
}

// WithURL points the client at a different endpoint. Tests use it; there
// is no production reason to call it.
func (c *Client) WithURL(u string) *Client {
	c.url = u
	return c
}

// WithHTTPClient replaces the transport. Tests use it.
func (c *Client) WithHTTPClient(h *http.Client) *Client {
	c.http = h
	return c
}

type siteVerifyResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

// Verify posts the token to siteverify. A nil error means Cloudflare
// answered; Verdict.OK carries its answer. A non-nil error means we
// never got an answer, and Verdict.OK is false — FailOpen is what turns
// that into an admitted request.
func (c *Client) Verify(ctx context.Context, token, remoteIP string) (Verdict, error) {
	if strings.TrimSpace(token) == "" {
		// No round trip for a token the visitor never supplied.
		// Cloudflare would answer missing-input-response; saying so
		// ourselves keeps the handler's two rejection paths identical.
		return Verdict{OK: false, ErrorCodes: []string{"missing-input-response"}}, nil
	}

	form := url.Values{}
	form.Set("secret", c.secret)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, strings.NewReader(form.Encode()))
	if err != nil {
		return Verdict{}, fmt.Errorf("turnstile request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return Verdict{}, fmt.Errorf("turnstile siteverify: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Cap the read: this is a small JSON object, and an unbounded read
	// from a third party in a request path is how a slow endpoint turns
	// into memory pressure.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return Verdict{}, fmt.Errorf("turnstile read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return Verdict{}, fmt.Errorf("turnstile siteverify: http %d", resp.StatusCode)
	}

	var out siteVerifyResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return Verdict{}, fmt.Errorf("turnstile decode: %w", err)
	}
	return Verdict{OK: out.Success, ErrorCodes: out.ErrorCodes}, nil
}

// FailOpen wraps a Verifier so that a transport error admits the request
// instead of blocking it, returning the error alongside so the caller
// logs it. A verdict Cloudflare actually issued is passed through
// untouched: a token Cloudflare rejected is still a rejection.
//
// This mirrors auth.FailOpen around the breach lookup. The two external
// calls in the registration path now answer the question "the third
// party is unreachable, now what?" the same way, which is the point —
// one answer, in one place, rather than a per-call-site judgment.
type FailOpen struct{ Inner Verifier }

func (f FailOpen) Verify(ctx context.Context, token, remoteIP string) (Verdict, error) {
	v, err := f.Inner.Verify(ctx, token, remoteIP)
	if err != nil && !errors.Is(err, context.Canceled) {
		return Verdict{OK: true, FailedOpen: true}, err
	}
	return v, err
}
