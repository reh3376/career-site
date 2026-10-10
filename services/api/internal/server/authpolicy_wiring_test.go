package server

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/authpolicy"
	"github.com/reh3376/career-site/services/api/internal/config"
	"github.com/reh3376/career-site/services/api/internal/handlers"
)

// Is the interceptor actually attached to every mounted service?
//
// `routes()` applies it per mount. That is eleven separate places it can
// be left off, and a service mounted without it is precisely the hole
// this sprint exists to close: the policy would be built, logged and
// reported as covering 133 procedures while one service quietly enforced
// nothing but its own handler checks. Counting mounts does not catch
// that, and neither does reading the code, because the next service
// added is the one that gets missed.
//
// So this drives real HTTP at every mounted service and requires a
// refusal. It derives the procedures from the policy map rather than
// listing them, so a newly mounted service is covered the day it appears
// without anyone remembering to come here.
//
// The handlers are zero values, which is safe *because* of what is being
// tested: in enforce mode the interceptor refuses an anonymous caller
// before the handler runs, so nothing ever reaches a nil dependency. If
// the interceptor were missing, the call would reach a zero-value
// handler and return anything but 401, which is the failure this wants.
func TestTheInterceptorIsAttachedToEveryMountedService(t *testing.T) {
	policy, err := authpolicy.Build()
	if err != nil {
		t.Fatalf("build the policy: %v", err)
	}

	// Anonymous: no session resolves, so every non-public procedure must
	// be refused with unauthenticated.
	resolve := func(_ context.Context, _ http.Header) (authpolicy.Caller, error) {
		return authpolicy.Caller{}, nil
	}
	var logBuf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logBuf, nil))
	intc, err := authpolicy.NewInterceptor(policy, resolve, authpolicy.ModeEnforce, log)
	if err != nil {
		t.Fatalf("new interceptor: %v", err)
	}

	handler := New(config.Config{Addr: ":0"}, log, Deps{
		AuthPolicy:      policy,
		AuthInterceptor: intc,
		Auth:            &handlers.Auth{},
		Member:          &handlers.Member{},
		Contact:         &handlers.Contact{},
		Admin:           &handlers.Admin{},
		Activity:        &handlers.Activity{},
		Jd:              &handlers.Jd{},
		Meetings:        &handlers.Meetings{},
		Chat:            &handlers.Chat{},
		Events:          &handlers.Events{},
		DecisionTest:    &handlers.DecisionTest{},
	}).routes()

	srv := httptest.NewServer(handler)
	defer srv.Close()

	probes := nonPublicProbePerService(policy)
	// Eight of the eleven mounted services have something to probe.
	// DecisionTestService, EventService and SystemService expose only
	// public procedures, so there is nothing on them that the
	// interceptor should refuse.
	//
	// Some probes land on a procedure no handler implements
	// (AuthService.ChangeEmail, MemberService.DeleteAccount and others
	// from the declaredNotImplemented list). That is not a flaw in the
	// choice: Connect would answer those with `unimplemented`, so a 401
	// proves the interceptor decided *before* Connect's own
	// Unimplemented handler got the chance, which is exactly the
	// ordering being tested.
	//
	// The floor is asserted so a broken derivation cannot quietly
	// reduce this test to probing nothing.
	if len(probes) < 6 {
		t.Fatalf("only %d services have a non-public procedure to probe; "+
			"the derivation from the policy map is broken and this test is "+
			"asserting almost nothing", len(probes))
	}

	services := make([]string, 0, len(probes))
	for svc := range probes {
		services = append(services, svc)
	}
	sort.Strings(services)

	for _, svc := range services {
		procedure := probes[svc]
		t.Run(svc, func(t *testing.T) {
			// Connect's JSON protocol: a POST with an empty object is a
			// valid unary call, which is all that is needed to find out
			// whether anything intercepted it.
			req := httptest.NewRequest(http.MethodPost, "/api"+procedure,
				strings.NewReader("{}"))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			body, _ := io.ReadAll(rec.Body)

			// Connect maps CodeUnauthenticated onto HTTP 401.
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("%s returned %d, expected 401.\n"+
					"Either this service is mounted without the auth "+
					"interceptor, or the interceptor stopped refusing "+
					"anonymous callers.\nbody: %s",
					procedure, rec.Code, body)
			}
			if !strings.Contains(string(body), "not signed in") {
				t.Errorf("%s was refused, but not by the interceptor: the "+
					"message should be the interceptor's.\nbody: %s",
					procedure, body)
			}
		})
	}
}

// A public procedure is not refused, so the 401s above mean "the
// interceptor decided" rather than "everything is refused".
func TestPublicProceduresStayReachableWithTheInterceptorEnforcing(t *testing.T) {
	policy, err := authpolicy.Build()
	if err != nil {
		t.Fatalf("build the policy: %v", err)
	}
	resolve := func(_ context.Context, _ http.Header) (authpolicy.Caller, error) {
		return authpolicy.Caller{}, nil
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	intc, err := authpolicy.NewInterceptor(policy, resolve, authpolicy.ModeEnforce, log)
	if err != nil {
		t.Fatalf("new interceptor: %v", err)
	}

	handler := New(config.Config{Addr: ":0"}, log, Deps{
		AuthPolicy:      policy,
		AuthInterceptor: intc,
	}).routes()

	// GetVersion is public and SystemService needs no dependencies, so
	// it can answer for real.
	req := httptest.NewRequest(http.MethodPost,
		"/api/career.v1.SystemService/GetVersion", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		body, _ := io.ReadAll(rec.Body)
		t.Fatalf("a public procedure returned %d with the interceptor "+
			"enforcing; it should be reachable anonymously.\nbody: %s",
			rec.Code, body)
	}
}

// nonPublicProbePerService picks one MEMBER or ADMIN procedure for each
// service, preferring a stable choice so a failure names the same
// procedure every run.
func nonPublicProbePerService(policy authpolicy.Map) map[string]string {
	byService := map[string][]string{}
	for procedure, lvl := range policy {
		if lvl == v1.AuthLevel_AUTH_LEVEL_PUBLIC {
			continue
		}
		cut := strings.LastIndex(procedure, "/")
		if cut <= 0 {
			continue
		}
		svc := strings.TrimPrefix(procedure[:cut], "/")
		byService[svc] = append(byService[svc], procedure)
	}

	out := make(map[string]string, len(byService))
	for svc, procedures := range byService {
		// Services the server does not mount have no route, so probing
		// them would return 404 and say nothing about interception.
		if unmountedService(svc) {
			continue
		}
		sort.Strings(procedures)
		out[svc] = procedures[0]
	}
	return out
}

// unmountedService mirrors the three services routes() never mounts.
// Kept here rather than imported because the coverage test asserts the
// same three from the other direction, so a change has to break one of
// them loudly.
func unmountedService(svc string) bool {
	switch svc {
	case "career.v1.HomeService", "career.v1.ContentService", "career.v1.DownloadService":
		return true
	}
	return false
}

// The observation heartbeat.
//
// S3's success condition is the absence of something, and after Logout
// was declared PUBLIC on 2026-10-09 there is nothing left for observe
// mode to disagree about. So the expected output for the week is an
// empty disagreement log, which is also what an interceptor installed on
// nothing produces. The periodic summary is what separates those two,
// and a goroutine on a 15-minute timer is not something to ship
// unexercised, so observeInterval is a var and this drives it.
func TestTheObservationHeartbeatReportsWhatWasCompared(t *testing.T) {
	policy, err := authpolicy.Build()
	if err != nil {
		t.Fatalf("build the policy: %v", err)
	}

	// An active member, so a member procedure is allowed and compared.
	resolve := func(_ context.Context, _ http.Header) (authpolicy.Caller, error) {
		return authpolicy.Caller{Authenticated: true, Active: true}, nil
	}
	logBuf := &syncWriter{w: &bytes.Buffer{}}
	log := slog.New(slog.NewTextHandler(logBuf, nil))
	intc, err := authpolicy.NewInterceptor(policy, resolve, authpolicy.ModeObserve, log)
	if err != nil {
		t.Fatalf("new interceptor: %v", err)
	}

	prev := observeInterval
	observeInterval = 10 * time.Millisecond
	defer func() { observeInterval = prev }()

	s := New(config.Config{Addr: ":0", ShutdownTimeout: time.Second}, log, Deps{
		AuthPolicy:      policy,
		AuthInterceptor: intc,
	})
	handler := s.routes()
	s.startPolicyObservation()
	defer s.stopPolicyObservation()

	// One public call, which is allowed and never compared.
	callOnce(t, handler, "/api/career.v1.SystemService/GetVersion")

	// Wait for a periodic summary rather than sleeping a fixed time.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(logBuf.String(), "reason=periodic") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	out := logBuf.String()
	if !strings.Contains(out, "reason=periodic") {
		t.Fatalf("no periodic summary was logged within 3s, so a quiet week "+
			"would be indistinguishable from a broken interceptor:\n%s", out)
	}
	for _, want := range []string{
		"auth policy observation",
		"mode=observe",
		"public_allowed=1",
		"would_close=0",
		"would_open=0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the summary is missing %q:\n%s", want, out)
		}
	}
	// A clean run is not a warning.
	if strings.Contains(out, "level=WARN") {
		t.Errorf("a clean observation was logged at warn:\n%s", out)
	}
}

// Nothing starts in enforce mode: the summary exists to make observe
// mode's silence legible, and enforce mode's evidence is the denials
// themselves.
func TestTheHeartbeatDoesNotRunWhenEnforcing(t *testing.T) {
	policy, err := authpolicy.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	resolve := func(_ context.Context, _ http.Header) (authpolicy.Caller, error) {
		return authpolicy.Caller{}, nil
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	intc, err := authpolicy.NewInterceptor(policy, resolve, authpolicy.ModeEnforce, log)
	if err != nil {
		t.Fatalf("new interceptor: %v", err)
	}

	s := New(config.Config{Addr: ":0"}, log, Deps{
		AuthPolicy:      policy,
		AuthInterceptor: intc,
	})
	s.startPolicyObservation()
	if s.observeStop != nil {
		t.Error("the heartbeat started in enforce mode")
	}
	// Stopping when nothing started must be safe: Shutdown calls it
	// unconditionally.
	s.stopPolicyObservation()
}

func callOnce(t *testing.T, h http.Handler, path string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(httptest.NewRecorder(), req)
}

// syncWriter serialises writes so the test can read the buffer while the
// heartbeat goroutine is still writing to it. Without it this test is a
// data race that only -race reports.
type syncWriter struct {
	mu sync.Mutex
	w  *bytes.Buffer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

// String must take the same lock as Write. Reading the buffer directly
// while the heartbeat goroutine is logging into it is a data race, and
// one that only -race reports, so the first version of this test passed
// locally and would have been caught in CI.
func (s *syncWriter) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.String()
}
