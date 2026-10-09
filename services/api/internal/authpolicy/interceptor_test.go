package authpolicy

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/gen/career/v1/careerv1connect"
)

// These run through a real Connect server rather than calling decide()
// directly, for two reasons. `Spec().Procedure` only exists on a request
// that came through Connect's plumbing, so a hand-built request would
// test the interceptor against an empty procedure and a policy lookup
// that always misses. And the thing being verified is the behaviour a
// caller sees: a code and a message, on the unary and the streaming
// path, with the session arriving as a cookie header the way a browser
// sends it.
//
// Three real procedures stand in for the three levels, so the policy
// being read is the actual one built from the descriptors:
//
//	PUBLIC  /career.v1.SystemService/GetVersion
//	MEMBER  /career.v1.MemberService/GetMe
//	ADMIN   /career.v1.AdminService/ListMembers
//	MEMBER, streaming  /career.v1.ChatService/SendMessage

// Session tokens the test resolver understands. The resolver is the
// seam the real one occupies in main, so these cover the states that
// matter rather than the ones that are easy.
const (
	tokenAdmin       = "admin-active"
	tokenMember      = "member-active"
	tokenSuspended   = "member-suspended"
	tokenSuspendedAd = "admin-suspended"
	tokenBroken      = "lookup-explodes"
)

func testResolver(calls *int) ResolveFunc {
	return func(_ context.Context, header http.Header) (Caller, error) {
		if calls != nil {
			*calls++
		}
		switch sessionToken(header) {
		case tokenAdmin:
			return Caller{Authenticated: true, Active: true, Admin: true}, nil
		case tokenMember:
			return Caller{Authenticated: true, Active: true}, nil
		case tokenSuspended:
			return Caller{Authenticated: true}, nil
		case tokenSuspendedAd:
			return Caller{Authenticated: true, Admin: true}, nil
		case tokenBroken:
			return Caller{}, errors.New("database is down")
		default:
			// No session. Not an error.
			return Caller{}, nil
		}
	}
}

func sessionToken(header http.Header) string {
	for _, line := range header.Values("Cookie") {
		for _, part := range strings.Split(line, ";") {
			name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
			if ok && name == "career_site_session" {
				return value
			}
		}
	}
	return ""
}

// stack is a running server with the interceptor installed.
type stack struct {
	admin   careerv1connect.AdminServiceClient
	member  careerv1connect.MemberServiceClient
	system  careerv1connect.SystemServiceClient
	chat    careerv1connect.ChatServiceClient
	logs    func() string
	served  func() int
	resolls func() int
}

// handlerBehaviour lets a test say what the stub handler does, which is
// how observe mode's comparison gets exercised: the interesting cases
// are a handler that serves a request the policy would refuse, and a
// handler that refuses one the policy allows.
type handlerBehaviour func(procedure string) error

func newStack(t *testing.T, mode Mode, behave handlerBehaviour) *stack {
	t.Helper()

	policy, err := Build()
	if err != nil {
		t.Fatalf("build the policy: %v", err)
	}

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	resolls := 0
	intc, err := NewInterceptor(policy, testResolver(&resolls), mode, log)
	if err != nil {
		t.Fatalf("new interceptor: %v", err)
	}

	if behave == nil {
		behave = func(string) error { return nil }
	}
	served := 0
	count := func(procedure string) error {
		served++
		return behave(procedure)
	}

	opts := connect.WithInterceptors(intc)
	mux := http.NewServeMux()
	for _, reg := range []func() (string, http.Handler){
		func() (string, http.Handler) {
			return careerv1connect.NewAdminServiceHandler(&stubAdmin{on: count}, opts)
		},
		func() (string, http.Handler) {
			return careerv1connect.NewMemberServiceHandler(&stubMember{on: count}, opts)
		},
		func() (string, http.Handler) {
			return careerv1connect.NewSystemServiceHandler(&stubSystem{on: count}, opts)
		},
		func() (string, http.Handler) {
			return careerv1connect.NewChatServiceHandler(&stubChat{on: count}, opts)
		},
	} {
		path, h := reg()
		mux.Handle(path, h)
	}

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := srv.Client()
	return &stack{
		admin:   careerv1connect.NewAdminServiceClient(c, srv.URL),
		member:  careerv1connect.NewMemberServiceClient(c, srv.URL),
		system:  careerv1connect.NewSystemServiceClient(c, srv.URL),
		chat:    careerv1connect.NewChatServiceClient(c, srv.URL),
		logs:    buf.String,
		served:  func() int { return served },
		resolls: func() int { return resolls },
	}
}

func withSession[T any](msg *T, token string) *connect.Request[T] {
	req := connect.NewRequest(msg)
	if token != "" {
		req.Header().Set("Cookie", "career_site_session="+token)
	}
	return req
}

// ---------------------------------------------------------------
// Enforce mode: the codes a caller actually receives
// ---------------------------------------------------------------

// The level definitions the owner settled on 2026-10-09: MEMBER is a
// session on an active account, ADMIN adds the admin role. The two
// suspended cases are the behaviour change that decision accepted, and
// they are the reason this table is worth having: they are stricter than
// 72 of the 105 gated handlers are today.
func TestEnforceModeDecisions(t *testing.T) {
	cases := []struct {
		name     string
		token    string
		call     func(*stack, string) error
		wantCode connect.Code // 0 means the call should succeed
		wantMsg  string
	}{
		// PUBLIC is open to everyone, which is what keeps the decision
		// test and sign-in working.
		{"public, anonymous", "", callPublic, 0, ""},
		{"public, member", tokenMember, callPublic, 0, ""},

		{"member procedure, anonymous", "", callMember,
			connect.CodeUnauthenticated, "not signed in"},
		{"member procedure, active member", tokenMember, callMember, 0, ""},
		{"member procedure, active admin", tokenAdmin, callMember, 0, ""},
		// Suspended mid-session. requireMember already refuses this;
		// the three bare-lookup handlers do not, and this is the
		// decision to match the stricter one.
		{"member procedure, suspended member", tokenSuspended, callMember,
			connect.CodePermissionDenied, "account is not active"},

		{"admin procedure, anonymous", "", callAdmin,
			connect.CodeUnauthenticated, "not signed in"},
		{"admin procedure, active member", tokenMember, callAdmin,
			connect.CodePermissionDenied, "admin role required"},
		{"admin procedure, active admin", tokenAdmin, callAdmin, 0, ""},
		// Status is checked before the role, so a suspended admin is
		// refused for being suspended rather than for lacking a role.
		// requireAdmin does not check this today; requireChatAdmin does.
		{"admin procedure, suspended admin", tokenSuspendedAd, callAdmin,
			connect.CodePermissionDenied, "account is not active"},

		// A resolver failure is not "no session": it is unknown, and
		// unknown fails closed with the same code the handlers use.
		{"member procedure, resolver fails", tokenBroken, callMember,
			connect.CodeInternal, "session lookup failed"},

		// The streaming path takes the same decision.
		{"streaming member procedure, anonymous", "", callChat,
			connect.CodeUnauthenticated, "not signed in"},
		{"streaming member procedure, active member", tokenMember, callChat, 0, ""},
		{"streaming member procedure, suspended", tokenSuspended, callChat,
			connect.CodePermissionDenied, "account is not active"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newStack(t, ModeEnforce, nil)
			err := c.call(s, c.token)

			if c.wantCode == 0 {
				if err != nil {
					t.Fatalf("expected the call to succeed, got %v", err)
				}
				if s.served() != 1 {
					t.Errorf("the handler ran %d times, expected 1", s.served())
				}
				return
			}

			if err == nil {
				t.Fatalf("expected %s, the call succeeded", c.wantCode)
			}
			if got := connect.CodeOf(err); got != c.wantCode {
				t.Errorf("code = %s, want %s (%v)", got, c.wantCode, err)
			}
			if !strings.Contains(err.Error(), c.wantMsg) {
				t.Errorf("message = %q, want it to contain %q", err.Error(), c.wantMsg)
			}
			// The whole point of an interceptor: the handler is never
			// reached. If it ran, the handler's own gate is what denied
			// the call and this test proves nothing about enforcement.
			if s.served() != 0 {
				t.Errorf("the handler ran despite the denial, so the "+
					"interceptor is not deciding before it (%d calls)", s.served())
			}
		})
	}
}

// A public procedure must not cost a session lookup. The decision test
// is public and is the busiest surface on the site; resolving a session
// it does not have would add a database round trip per answer.
func TestPublicProceduresResolveNoSession(t *testing.T) {
	s := newStack(t, ModeEnforce, nil)

	if err := callPublic(s, ""); err != nil {
		t.Fatalf("public call failed: %v", err)
	}
	if s.resolls() != 0 {
		t.Errorf("the resolver ran %d times for a public procedure, expected 0",
			s.resolls())
	}

	// And a cookie being present does not change that.
	if err := callPublic(s, tokenMember); err != nil {
		t.Fatalf("public call with a session failed: %v", err)
	}
	if s.resolls() != 0 {
		t.Errorf("the resolver ran %d times for a public procedure carrying a "+
			"session, expected 0", s.resolls())
	}

	// A member procedure does resolve, so the count above means
	// "skipped" rather than "the resolver is never called".
	_ = callMember(s, tokenMember)
	if s.resolls() == 0 {
		t.Error("the resolver never ran for a member procedure either, so the " +
			"previous assertions do not show that public skips it")
	}
}

// ---------------------------------------------------------------
// Observe mode
// ---------------------------------------------------------------

// Observe mode must change nothing. Every call that the enforce table
// above refuses has to succeed here, or S3 is not the inert step the
// plan says it is.
func TestObserveModeEnforcesNothing(t *testing.T) {
	for _, c := range []struct {
		name  string
		token string
		call  func(*stack, string) error
	}{
		{"admin procedure, anonymous", "", callAdmin},
		{"admin procedure, active member", tokenMember, callAdmin},
		{"admin procedure, suspended admin", tokenSuspendedAd, callAdmin},
		{"member procedure, anonymous", "", callMember},
		{"member procedure, suspended", tokenSuspended, callMember},
		{"member procedure, resolver fails", tokenBroken, callMember},
		{"streaming, anonymous", "", callChat},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := newStack(t, ModeObserve, nil)
			if err := c.call(s, c.token); err != nil {
				t.Fatalf("observe mode denied the call: %v", err)
			}
			if s.served() != 1 {
				t.Errorf("the handler ran %d times, expected 1", s.served())
			}
		})
	}
}

// would_close: the handler served a request the interceptor would have
// refused. This is the finding that matters most, because it is a real
// behaviour change waiting at S4.
func TestObserveLogsWouldClose(t *testing.T) {
	s := newStack(t, ModeObserve, nil) // handler always succeeds

	if err := callAdmin(s, ""); err != nil {
		t.Fatalf("observe mode denied the call: %v", err)
	}

	out := s.logs()
	for _, want := range []string{
		"auth policy disagreement",
		"would_close",
		"/career.v1.AdminService/ListMembers",
		"AUTH_LEVEL_ADMIN",
		"served the request",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the log is missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "level=WARN") {
		t.Errorf("a would_close disagreement was not logged at warn:\n%s", out)
	}
}

// would_open: the handler demanded a session the interceptor did not.
// This is the dangerous direction and should never appear in production.
func TestObserveLogsWouldOpen(t *testing.T) {
	s := newStack(t, ModeObserve, func(procedure string) error {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("not signed in"))
	})

	// The interceptor allows this: an active member calling a member
	// procedure. The handler refuses anyway.
	_ = callMember(s, tokenMember)

	out := s.logs()
	for _, want := range []string{"would_open", "/career.v1.MemberService/GetMe"} {
		if !strings.Contains(out, want) {
			t.Errorf("the log is missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "level=WARN") {
		t.Errorf("a would_open disagreement was not logged at warn:\n%s", out)
	}
}

// A handler refusing with permission_denied where the policy allows is
// ambiguous: it may be authorisation the auth level does not describe,
// such as "not your resource". Logged at info so it can be triaged
// without burying the two findings that are unambiguous.
func TestObserveSeparatesAmbiguousPermissionDenials(t *testing.T) {
	s := newStack(t, ModeObserve, func(procedure string) error {
		return connect.NewError(connect.CodePermissionDenied, errors.New("not your record"))
	})

	_ = callMember(s, tokenMember)

	out := s.logs()
	if !strings.Contains(out, "handler_denied_permission") {
		t.Errorf("the ambiguous case was not logged:\n%s", out)
	}
	if strings.Contains(out, "would_open") {
		t.Errorf("an ambiguous permission_denied was reported as would_open, "+
			"which is the finding that is supposed to mean something:\n%s", out)
	}
	if strings.Contains(out, "level=WARN") {
		t.Errorf("the ambiguous case was logged at warn, which is what makes "+
			"the real findings hard to see:\n%s", out)
	}
}

// A public procedure whose handler refuses is not a policy
// disagreement. Login rejecting a declined account returns
// permission_denied, and warning on every failed sign-in would make
// this mode's output worthless.
func TestObserveIgnoresDenialsOnPublicProcedures(t *testing.T) {
	s := newStack(t, ModeObserve, func(procedure string) error {
		return connect.NewError(connect.CodePermissionDenied,
			errors.New("this account is not permitted to sign in"))
	})

	_ = callPublic(s, "")

	if out := s.logs(); strings.Contains(out, "disagreement") ||
		strings.Contains(out, "handler_denied_permission") {
		t.Errorf("a denial on a public procedure was reported as a policy "+
			"disagreement:\n%s", out)
	}
}

// Agreement is silent. A mode that logged on every request would be
// turned off long before the week of observation was up.
func TestObserveIsQuietWhenTheTwoAgree(t *testing.T) {
	s := newStack(t, ModeObserve, nil)

	if err := callAdmin(s, tokenAdmin); err != nil {
		t.Fatalf("admin call failed: %v", err)
	}
	if err := callMember(s, tokenMember); err != nil {
		t.Fatalf("member call failed: %v", err)
	}
	if err := callPublic(s, ""); err != nil {
		t.Fatalf("public call failed: %v", err)
	}

	if out := s.logs(); strings.Contains(out, "disagreement") {
		t.Errorf("a disagreement was logged for three calls that agree:\n%s", out)
	}
}

// ---------------------------------------------------------------
// Construction
// ---------------------------------------------------------------

// Both of these would present as a total outage with no obvious cause,
// so they are refused where the message can say which one it was.
func TestNewInterceptorRefusesAnUnusableConfiguration(t *testing.T) {
	policy, err := Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	log := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	resolve := testResolver(nil)

	for _, c := range []struct {
		name    string
		policy  Map
		resolve ResolveFunc
		log     *slog.Logger
		want    string
	}{
		{"empty policy", Map{}, resolve, log, "non-empty policy"},
		{"nil policy", nil, resolve, log, "non-empty policy"},
		{"nil resolver", policy, nil, log, "session resolver"},
		{"nil logger", policy, resolve, nil, "logger"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := NewInterceptor(c.policy, c.resolve, ModeEnforce, c.log)
			if err == nil {
				t.Fatalf("accepted %s and returned an interceptor", c.name)
			}
			if got != nil {
				t.Errorf("returned both an error and an interceptor")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q does not mention %q", err, c.want)
			}
		})
	}
}

// An unrecognised mode is an error, never a default. A typo selecting
// enforce would deny the admin console; a typo selecting observe would
// leave enforcement off while the deploy meant to turn it on reported
// success.
func TestParseMode(t *testing.T) {
	for _, c := range []struct {
		in      string
		want    Mode
		wantErr bool
	}{
		{"observe", ModeObserve, false},
		{"enforce", ModeEnforce, false},
		{"", ModeObserve, true},
		{"Enforce", ModeObserve, true},
		{"on", ModeObserve, true},
		{"true", ModeObserve, true},
	} {
		got, err := ParseMode(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseMode(%q) accepted it, returning %s", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseMode(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseMode(%q) = %s, want %s", c.in, got, c.want)
		}
	}
	if ModeObserve.String() != "observe" || ModeEnforce.String() != "enforce" {
		t.Errorf("mode names are %q and %q; they appear in the boot log",
			ModeObserve, ModeEnforce)
	}
}

// ---------------------------------------------------------------
// Call helpers and stub handlers
// ---------------------------------------------------------------

func callPublic(s *stack, token string) error {
	_, err := s.system.GetVersion(context.Background(),
		withSession(&v1.GetVersionRequest{}, token))
	return err
}

func callMember(s *stack, token string) error {
	_, err := s.member.GetMe(context.Background(),
		withSession(&v1.GetMeRequest{}, token))
	return err
}

func callAdmin(s *stack, token string) error {
	_, err := s.admin.ListMembers(context.Background(),
		withSession(&v1.ListMembersRequest{}, token))
	return err
}

// callChat drives the server-streaming path. The stream has to be read
// to completion, because a denial raised before the handler runs
// surfaces on the first Receive rather than at the call.
func callChat(s *stack, token string) error {
	st, err := s.chat.SendMessage(context.Background(),
		withSession(&v1.SendMessageRequest{}, token))
	if err != nil {
		return err
	}
	defer st.Close()
	for st.Receive() {
	}
	return st.Err()
}

type stubAdmin struct {
	careerv1connect.UnimplementedAdminServiceHandler
	on handlerBehaviour
}

func (s *stubAdmin) ListMembers(
	_ context.Context,
	req *connect.Request[v1.ListMembersRequest],
) (*connect.Response[v1.ListMembersResponse], error) {
	if err := s.on(req.Spec().Procedure); err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.ListMembersResponse{}), nil
}

type stubMember struct {
	careerv1connect.UnimplementedMemberServiceHandler
	on handlerBehaviour
}

func (s *stubMember) GetMe(
	_ context.Context,
	req *connect.Request[v1.GetMeRequest],
) (*connect.Response[v1.GetMeResponse], error) {
	if err := s.on(req.Spec().Procedure); err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.GetMeResponse{}), nil
}

type stubSystem struct {
	careerv1connect.UnimplementedSystemServiceHandler
	on handlerBehaviour
}

func (s *stubSystem) GetVersion(
	_ context.Context,
	req *connect.Request[v1.GetVersionRequest],
) (*connect.Response[v1.GetVersionResponse], error) {
	if err := s.on(req.Spec().Procedure); err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.GetVersionResponse{}), nil
}

type stubChat struct {
	careerv1connect.UnimplementedChatServiceHandler
	on handlerBehaviour
}

func (s *stubChat) SendMessage(
	_ context.Context,
	req *connect.Request[v1.SendMessageRequest],
	stream *connect.ServerStream[v1.SendMessageResponse],
) error {
	if err := s.on(req.Spec().Procedure); err != nil {
		return err
	}
	return stream.Send(&v1.SendMessageResponse{})
}
