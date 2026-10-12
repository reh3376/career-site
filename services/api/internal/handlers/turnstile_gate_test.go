package handlers

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/turnstile"
)

// These tests cover the bot gate on the two public endpoints that create
// state without an authenticated caller: Register (FR-AUTH-09) and the
// anonymous path of SubmitContact (FR-CNT-22).
//
// They run without a database on purpose. Both gates sit ahead of every
// repository call, so a rejected request never reaches one — and if a
// later edit moves the gate below the DB work, these tests panic on the
// nil repo rather than passing quietly. The nil repo is the assertion
// that the ordering holds.

type fakeVerifier struct {
	verdict turnstile.Verdict
	err     error
	calls   int
	gotTok  string
	gotIP   string
}

func (f *fakeVerifier) Verify(_ context.Context, token, ip string) (turnstile.Verdict, error) {
	f.calls++
	f.gotTok = token
	f.gotIP = ip
	return f.verdict, f.err
}

func quietLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func registerReq(token string, ip string) *connect.Request[v1.RegisterRequest] {
	r := connect.NewRequest(&v1.RegisterRequest{
		TurnstileToken: token,
		// Deliberately false. Consent is the very next check after the
		// gate, so "consent_accepted must be true" is the signal that a
		// request got PAST the bot gate, with no DB in between.
		ConsentAccepted: false,
	})
	r.Header().Set("X-Forwarded-For", ip)
	return r
}

func newGateAuth(v turnstile.Verifier) *Auth {
	h := NewAuth(quietLog(), nil, nil, nil, AuthConfig{})
	h.SetTurnstile(v)
	return h
}

func TestRegisterRefusesATokenCloudflareRejected(t *testing.T) {
	fake := &fakeVerifier{verdict: turnstile.Verdict{OK: false, ErrorCodes: []string{"invalid-input-response"}}}
	h := newGateAuth(fake)

	_, err := h.Register(context.Background(), registerReq("bad-token", "203.0.113.1"))
	if err == nil {
		t.Fatal("a rejected Turnstile token was allowed to register")
	}
	if got := connect.CodeOf(err); got != connect.CodeInvalidArgument {
		t.Fatalf("code = %v, want invalid_argument", got)
	}
	if !strings.Contains(err.Error(), "bot check failed") {
		t.Fatalf("error does not name the bot check: %v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("verifier called %d times, want 1", fake.calls)
	}
	if fake.gotTok != "bad-token" {
		t.Fatalf("token not forwarded to the verifier: %q", fake.gotTok)
	}
	if fake.gotIP != "203.0.113.1" {
		t.Fatalf("client IP not forwarded to the verifier: %q", fake.gotIP)
	}
}

// The empty-token case is the one that matters most: before this change
// Register never read TurnstileToken at all, so a request with no token
// sailed through. If the gate is ever removed, this is what catches it.
func TestRegisterRefusesAMissingToken(t *testing.T) {
	h := newGateAuth(turnstile.New("0xTESTSECRET")) // real client, empty token short-circuits
	_, err := h.Register(context.Background(), registerReq("", "203.0.113.2"))
	if err == nil {
		t.Fatal("registration with no Turnstile token at all was accepted")
	}
	if !strings.Contains(err.Error(), "bot check failed") {
		t.Fatalf("a missing token did not fail the bot check: %v", err)
	}
}

// An unreachable Cloudflare admits the request. The proof that it got
// past the gate is that it reached the next check (consent) instead of
// being rejected for the bot check.
func TestRegisterFailsOpenWhenCloudflareIsUnreachable(t *testing.T) {
	fake := &fakeVerifier{err: errors.New("dial tcp: i/o timeout")}
	h := newGateAuth(fake)

	_, err := h.Register(context.Background(), registerReq("tok", "203.0.113.3"))
	if err == nil {
		t.Fatal("expected the consent check to reject this request")
	}
	if strings.Contains(err.Error(), "bot check failed") {
		t.Fatalf("an unreachable Cloudflare blocked registration; the decision was to fail open: %v", err)
	}
	if !strings.Contains(err.Error(), "consent_accepted") {
		t.Fatalf("request did not reach the check after the gate: %v", err)
	}
}

// The rate limit is the control that still stands during a Cloudflare
// outage, which is the only reason failing open is tolerable.
func TestRegisterIsRateLimitedPerIP(t *testing.T) {
	h := newGateAuth(&fakeVerifier{verdict: turnstile.Verdict{OK: true}})

	var limited bool
	for i := 0; i < 12; i++ {
		_, err := h.Register(context.Background(), registerReq("tok", "203.0.113.4"))
		if err != nil && connect.CodeOf(err) == connect.CodeResourceExhausted {
			limited = true
			break
		}
	}
	if !limited {
		t.Fatal("12 registrations from one address were never rate limited")
	}

	// A different address must not be caught in the first one's bucket.
	_, err := h.Register(context.Background(), registerReq("tok", "198.51.100.9"))
	if err != nil && connect.CodeOf(err) == connect.CodeResourceExhausted {
		t.Fatal("a second address was rate limited by the first address's attempts")
	}
}

// The limit runs before the Turnstile call, so a flood costs us nothing
// outbound. If the order flips, a burst becomes a burst of siteverify
// calls against our Cloudflare quota.
func TestRegisterRateLimitRunsBeforeTheTurnstileCall(t *testing.T) {
	fake := &fakeVerifier{verdict: turnstile.Verdict{OK: true}}
	h := newGateAuth(fake)

	for i := 0; i < 30; i++ {
		_, _ = h.Register(context.Background(), registerReq("tok", "203.0.113.5"))
	}
	if fake.calls >= 30 {
		t.Fatalf("every one of 30 attempts reached siteverify (%d calls); "+
			"the rate limit is not ahead of the outbound call", fake.calls)
	}
}

// --- anonymous contact ---

func contactReq(token string) *connect.Request[v1.SubmitContactRequest] {
	r := connect.NewRequest(&v1.SubmitContactRequest{
		Subject:        "hello",
		Message:        "a message",
		Category:       v1.SupportCategory_SUPPORT_CATEGORY_GENERAL_QUESTION,
		Name:           "A Visitor",
		Email:          "visitor@example.com",
		TurnstileToken: token,
	})
	r.Header().Set("X-Forwarded-For", "203.0.113.6")
	return r
}

func newGateContact(v turnstile.Verifier) *Contact {
	// The Auth is only used for the session-cookie lookup, which
	// returns (nil, nil) without touching the repo when no cookie is
	// present - so these requests take the anonymous path.
	h := NewContact(quietLog(), nil, newGateAuth(turnstile.Noop{}), nil, "", "", "")
	h.SetTurnstile(v)
	return h
}

// blockedAtTheGate reports a bypassed gate as a test failure rather
// than a panic. Past the gate the handler reaches the nil repository, so
// without this the whole test binary dies and the message is a stack
// trace instead of the reason.
func blockedAtTheGate(t *testing.T) {
	t.Helper()
	if r := recover(); r != nil {
		t.Fatalf("the submission got past the bot gate and reached the "+
			"repository: the Turnstile check is missing or no longer ahead "+
			"of the database work (panic: %v)", r)
	}
}

func TestAnonymousContactRefusesARejectedToken(t *testing.T) {
	defer blockedAtTheGate(t)
	fake := &fakeVerifier{verdict: turnstile.Verdict{OK: false, ErrorCodes: []string{"invalid-input-response"}}}
	h := newGateContact(fake)

	_, err := h.SubmitContact(context.Background(), contactReq("bad-token"))
	if err == nil {
		t.Fatal("an anonymous submission with a rejected token was accepted")
	}
	if got := connect.CodeOf(err); got != connect.CodeFailedPrecondition {
		t.Fatalf("code = %v, want failed_precondition", got)
	}
	if fake.calls != 1 {
		t.Fatalf("verifier called %d times, want 1", fake.calls)
	}
}

// turnstileEnforced() used to return false unconditionally, so the
// existing check at this call site never fired. A missing token is the
// shape that bug produced.
func TestAnonymousContactRefusesAMissingToken(t *testing.T) {
	defer blockedAtTheGate(t)
	h := newGateContact(turnstile.New("0xTESTSECRET"))
	_, err := h.SubmitContact(context.Background(), contactReq(""))
	if err == nil {
		t.Fatal("an anonymous submission with no Turnstile token was accepted")
	}
	if got := connect.CodeOf(err); got != connect.CodeFailedPrecondition {
		t.Fatalf("code = %v, want failed_precondition", got)
	}
}

// Fail open here means the submission proceeds to the database, which
// this test has no connection for - so a panic on the nil repo is the
// evidence it got past the gate, and a returned bot-check error is the
// evidence it did not.
func TestAnonymousContactFailsOpenWhenCloudflareIsUnreachable(t *testing.T) {
	h := newGateContact(&fakeVerifier{err: errors.New("dial tcp: i/o timeout")})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("submission did not reach the repository; it was blocked at the gate")
		}
	}()
	_, err := h.SubmitContact(context.Background(), contactReq("tok"))
	if err != nil {
		t.Fatalf("an unreachable Cloudflare blocked anonymous contact; the decision was to fail open: %v", err)
	}
}
