package handlers

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
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

// --- login ---

func loginReq(token, ip string) *connect.Request[v1.LoginRequest] {
	r := connect.NewRequest(&v1.LoginRequest{
		Email:          "someone@example.com",
		Password:       "whatever-it-does-not-get-that-far",
		TurnstileToken: token,
	})
	r.Header().Set("X-Forwarded-For", ip)
	return r
}

// A first sign-in is not challenged, whatever the verifier would say.
//
// This is the case that matters most and the one that was wrong for a
// few hours: always-on broke every non-browser sign-in, including the
// three scripts that verify production. The proof here is that the
// verifier is never called.
func TestAFirstSignInIsNotChallenged(t *testing.T) {
	fake := &fakeVerifier{verdict: turnstile.Verdict{OK: false}}
	h := newGateAuth(fake)
	defer func() { _ = recover() }() // reaches the nil repo, which is the point

	_, err := h.Login(context.Background(), loginReq("", "203.0.113.20"))
	if err != nil && strings.Contains(err.Error(), "bot check failed") {
		t.Fatal("a first sign-in was challenged; non-browser callers cannot solve one")
	}
	if fake.calls != 0 {
		t.Fatalf("a first sign-in cost %d siteverify calls, want 0", fake.calls)
	}
}

// After enough failures from one address, a token is required.
func TestSignInIsChallengedAfterRepeatedFailures(t *testing.T) {
	fake := &fakeVerifier{verdict: turnstile.Verdict{OK: false, ErrorCodes: []string{"invalid-input-response"}}}
	h := newGateAuth(fake)
	const ip = "203.0.113.21"

	// Record the failures directly: the handler's own failure path needs
	// a repository, and what is under test is the decision to challenge.
	for i := 0; i < challengeAfterFailures; i++ {
		h.loginFailures.record(ip)
	}

	_, err := h.Login(context.Background(), loginReq("bad-token", ip))
	if err == nil {
		t.Fatal("a challenged sign-in with a rejected token was allowed")
	}
	if !strings.Contains(err.Error(), "bot check failed") {
		t.Fatalf("error does not name the bot check: %v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("verifier called %d times, want 1", fake.calls)
	}
}

// The challenge is keyed on the address alone. Credential stuffing is
// many accounts from one source, so keying it on the email as well would
// hand an attacker a fresh allowance per guess.
func TestTheChallengeFollowsTheAddressNotTheAccount(t *testing.T) {
	h := newGateAuth(&fakeVerifier{verdict: turnstile.Verdict{OK: false}})
	for i := 0; i < challengeAfterFailures; i++ {
		h.loginFailures.record("203.0.113.22")
	}
	if h.loginFailures.count("203.0.113.22") < challengeAfterFailures {
		t.Fatal("failures were not recorded against the address")
	}
	if h.loginFailures.count("198.51.100.5") != 0 {
		t.Fatal("a different address inherited the first one's failures")
	}
}

// Proving the password clears the counter, so one mistyped password
// does not leave a challenge waiting for the next sign-in.
func TestASuccessfulPasswordClearsTheChallenge(t *testing.T) {
	h := newGateAuth(&fakeVerifier{verdict: turnstile.Verdict{OK: true}})
	const ip = "203.0.113.23"
	h.loginFailures.record(ip)
	h.loginFailures.record(ip)
	if h.loginFailures.count(ip) != 2 {
		t.Fatalf("count = %d, want 2", h.loginFailures.count(ip))
	}
	h.loginFailures.clear(ip)
	if h.loginFailures.count(ip) != 0 {
		t.Fatal("clear did not reset the counter")
	}
}

// Failing closed would lock every member out of their own account
// during an outage at a third party, which is worse than the thing the
// check prevents.
func TestAChallengedSignInFailsOpenWhenCloudflareIsUnreachable(t *testing.T) {
	h := newGateAuth(&fakeVerifier{err: errors.New("dial tcp: i/o timeout")})
	const ip = "203.0.113.24"
	for i := 0; i < challengeAfterFailures; i++ {
		h.loginFailures.record(ip)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("sign-in did not reach the credential check; it was blocked at the gate")
		}
	}()
	_, err := h.Login(context.Background(), loginReq("tok", ip))
	if err != nil && strings.Contains(err.Error(), "bot check failed") {
		t.Fatalf("an unreachable Cloudflare blocked a challenged sign-in: %v", err)
	}
}

// --- the decision test ---

func startReq(token string, synthetic bool) *connect.Request[v1.StartSessionRequest] {
	r := connect.NewRequest(&v1.StartSessionRequest{
		TurnstileToken: token,
		Synthetic:      synthetic,
	})
	r.Header().Set("X-Forwarded-For", "203.0.113.30")
	return r
}

func newGateDecisionTest(v turnstile.Verifier) *DecisionTest {
	h := NewDecisionTest(quietLog(), nil, newGateAuth(turnstile.Noop{}))
	h.SetTurnstile(v)
	return h
}

// StartSession is the public decision-test RPC that creates state: a
// dt_sessions row, a draw from the item bank, and a slot in a sample
// whose size is the whole point. Volunteers are the scarce resource, so
// machine runs cost more than the rows do.
//
// Challenged on the first call, unlike sign-in: there is no password to
// get wrong here, so there are no failures to count, and nothing
// legitimate starts a test without a browser.
func TestDecisionTestStartRefusesARejectedToken(t *testing.T) {
	fake := &fakeVerifier{verdict: turnstile.Verdict{OK: false, ErrorCodes: []string{"invalid-input-response"}}}
	h := newGateDecisionTest(fake)

	_, err := h.StartSession(context.Background(), startReq("bad-token", false))
	if err == nil {
		t.Fatal("a rejected token was allowed to start a decision test")
	}
	if got := connect.CodeOf(err); got != connect.CodeFailedPrecondition {
		t.Fatalf("code = %v, want failed_precondition", got)
	}
	if fake.calls != 1 {
		t.Fatalf("verifier called %d times, want 1", fake.calls)
	}
}

func TestDecisionTestStartRefusesAMissingToken(t *testing.T) {
	h := newGateDecisionTest(turnstile.New("0xTESTSECRET"))
	_, err := h.StartSession(context.Background(), startReq("", false))
	if err == nil {
		t.Fatal("a start with no token was accepted")
	}
	if got := connect.CodeOf(err); got != connect.CodeFailedPrecondition {
		t.Fatalf("code = %v, want failed_precondition", got)
	}
}

// A synthetic run is the owner's own agent driving the UI, never a
// browser. Challenging it would mean the agent has a widget to get
// stuck on, and the rows are excluded from analysis anyway.
func TestDecisionTestSyntheticRunIsNotChallenged(t *testing.T) {
	fake := &fakeVerifier{verdict: turnstile.Verdict{OK: false}}
	h := newGateDecisionTest(fake)

	_, err := h.StartSession(context.Background(), startReq("", true))
	if err != nil && connect.CodeOf(err) == connect.CodeFailedPrecondition {
		t.Fatal("a synthetic run was refused by the bot check")
	}
	if fake.calls != 0 {
		t.Fatalf("a synthetic run cost %d siteverify calls, want 0", fake.calls)
	}
}

// Only the start is challenged. The other four decision-test RPCs run
// inside a timed fifteen-minute sitting with no resume, so a challenge
// in front of SubmitAnswer would end a volunteer's run to stop a bot
// that already got past the start.
func TestOnlyTheStartOfTheDecisionTestIsChallenged(t *testing.T) {
	src, err := os.ReadFile("decisiontest.go")
	if err != nil {
		t.Fatalf("read decisiontest.go: %v", err)
	}
	body := string(src)
	if strings.Count(body, "turnstile.FailOpen{") != 1 {
		t.Errorf("decisiontest.go has %d Turnstile checks, want exactly 1 (StartSession). "+
			"A check on SubmitAnswer or SubmitRecall would interrupt a timed run.",
			strings.Count(body, "turnstile.FailOpen{"))
	}
}

// --- the address every per-visitor control is keyed on ---

// X-Real-IP wins, because Caddy sets it from the actual remote host and
// replaces whatever arrived. X-Forwarded-For is client-supplied until a
// proxy rewrites it, so reading that first let a caller choose its own
// rate-limit bucket by sending a header.
func TestClientIPPrefersTheHeaderTheProxySets(t *testing.T) {
	req := connect.NewRequest(&v1.LoginRequest{})
	req.Header().Set("X-Real-IP", "198.51.100.7")
	req.Header().Set("X-Forwarded-For", "203.0.113.1")
	if got := ClientIP(req); got != "198.51.100.7" {
		t.Fatalf("ClientIP = %q, want the X-Real-IP value: a caller must not be "+
			"able to pick its own bucket by sending X-Forwarded-For", got)
	}
}

// Without a proxy there is no X-Real-IP and nothing to spoof it to, so
// the forwarded header is still read. This is the local-development
// path.
func TestClientIPFallsBackToForwardedFor(t *testing.T) {
	req := connect.NewRequest(&v1.LoginRequest{})
	req.Header().Set("X-Forwarded-For", "203.0.113.1, 10.0.0.1")
	if got := ClientIP(req); got != "203.0.113.1" {
		t.Fatalf("ClientIP = %q, want the first forwarded address", got)
	}
}

// The production Caddyfile must set the header this now depends on. A
// deploy that drops the directive puts every per-visitor control back on
// one constant, which is how two failed sign-ins locked out the admin
// scripts site-wide.
func TestTheProductionProxySetsTheRealIPHeader(t *testing.T) {
	conf, err := os.ReadFile("../../../../deploy/caddy/Caddyfile.prod")
	if err != nil {
		t.Fatalf("read Caddyfile.prod: %v", err)
	}
	if !strings.Contains(string(conf), "header_up X-Real-IP {remote_host}") {
		t.Fatal("Caddyfile.prod no longer sets X-Real-IP on the api proxy. " +
			"Without it ClientIP falls through to the peer address, which is " +
			"one constant for all traffic, and every per-IP limit becomes global.")
	}
}
