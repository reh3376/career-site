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
	"testing"

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
	resolve := func(ctx context.Context, _ http.Header) (authpolicy.Caller, context.Context, error) {
		return authpolicy.Caller{}, ctx, nil
	}
	var logBuf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logBuf, nil))
	intc, err := authpolicy.NewInterceptor(policy, resolve, log)
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
			// Required since 2026-10-11. Without it these services
			// answer 400 for the protocol rather than 401 for the
			// session, and this test would be measuring the wrong
			// refusal.
			req.Header.Set("Connect-Protocol-Version", "1")
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
	resolve := func(ctx context.Context, _ http.Header) (authpolicy.Caller, context.Context, error) {
		return authpolicy.Caller{}, ctx, nil
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	intc, err := authpolicy.NewInterceptor(policy, resolve, log)
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
	req.Header.Set("Connect-Protocol-Version", "1")
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
	for procedure, pol := range policy {
		if pol.Level == v1.AuthLevel_AUTH_LEVEL_PUBLIC {
			continue
		}
		// A procedure that waives the active-account check still
		// requires a session, so an anonymous probe is still refused and
		// it is still a valid target here.
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

// The handlers that now have no check of their own at all.
//
// S5 removed the gates. For the 48 that only ever used theirs to refuse,
// nothing replaced it: ListMembers goes straight to the database, and
// ExportDecisionTestData straight to the CSV builder. The interceptor is
// the only thing between an anonymous caller and that data, which makes
// it load-bearing in a way it was not before S5.
//
// The generic probe above happens to pick AdminService/AddMemberNote,
// which is alphabetically first and has no handler, so a 401 there only
// proves the interceptor beat Connect's Unimplemented answer. These are
// real, implemented handlers that would return real data if reached.
//
// ExportDecisionTestData is here deliberately. It is the handler that
// shipped without a gate on 2026-10-05 and served 21,708 bytes of
// participant data to anonymous callers for five hours. It has no gate
// again now, by design, and this is what stands in for the one it lost.
func TestImplementedAdminHandlersWithNoCheckAreStillRefused(t *testing.T) {
	policy, err := authpolicy.Build()
	if err != nil {
		t.Fatalf("build the policy: %v", err)
	}
	resolve := func(ctx context.Context, _ http.Header) (authpolicy.Caller, context.Context, error) {
		return authpolicy.Caller{}, ctx, nil
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	intc, err := authpolicy.NewInterceptor(policy, resolve, log)
	if err != nil {
		t.Fatalf("new interceptor: %v", err)
	}

	handler := New(config.Config{Addr: ":0"}, log, Deps{
		AuthPolicy:      policy,
		AuthInterceptor: intc,
		Admin:           &handlers.Admin{},
	}).routes()

	for _, procedure := range []string{
		"/career.v1.AdminService/ListMembers",
		"/career.v1.AdminService/ListCorpusDocuments",
		"/career.v1.AdminService/ListJdSubmissions",
		"/career.v1.AdminService/ExportDecisionTestData",
		"/career.v1.AdminService/ListDecisionTestRuns",
		"/career.v1.AdminService/GetMetrics",
	} {
		t.Run(procedure, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api"+procedure,
				strings.NewReader("{}"))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Connect-Protocol-Version", "1")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			body, _ := io.ReadAll(rec.Body)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("%s returned %d for an anonymous caller, expected 401.\n"+
					"This handler has no access check of its own since S5, so "+
					"anything but a refusal here means real data is being "+
					"served to anyone.\nbody: %s", procedure, rec.Code, body)
			}
			if !strings.Contains(string(body), "not signed in") {
				t.Errorf("%s was refused but not by the interceptor.\nbody: %s",
					procedure, body)
			}
		})
	}
}

// `Connect-Protocol-Version: 1` is required, except where sendBeacon
// makes that impossible.
//
// The header is a second layer of CSRF defence behind SameSite=Lax: a
// browser will not attach a custom header to a cross-site form post, so
// a forged request cannot satisfy it. auth.proto claimed this was
// already true and it was not, because
// connect.WithRequireConnectProtocolHeader() was never set.
//
// The exemption is the interesting half. `navigator.sendBeacon` cannot
// set request headers at all, and the web app beacons three procedures
// on pagehide so a closing tab does not lose data. Requiring a header
// they cannot send would discard those writes silently, which is worse
// than the risk it would remove: all three are PUBLIC, so there is no
// session privilege for a forged request to borrow.
func TestConnectProtocolHeaderIsRequiredExceptOnBeaconServices(t *testing.T) {
	policy, err := authpolicy.Build()
	if err != nil {
		t.Fatalf("build the policy: %v", err)
	}
	resolve := func(ctx context.Context, _ http.Header) (authpolicy.Caller, context.Context, error) {
		return authpolicy.Caller{}, ctx, nil
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	intc, err := authpolicy.NewInterceptor(policy, resolve, log)
	if err != nil {
		t.Fatalf("new interceptor: %v", err)
	}
	handler := New(config.Config{Addr: ":0"}, log, Deps{
		AuthPolicy:      policy,
		AuthInterceptor: intc,
		DecisionTest:    &handlers.DecisionTest{},
		Events:          &handlers.Events{},
	}).routes()

	post := func(procedure string, withHeader bool) int {
		req := httptest.NewRequest(http.MethodPost, "/api"+procedure,
			strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		if withHeader {
			req.Header.Set("Connect-Protocol-Version", "1")
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	// A public procedure on a strict service: reachable with the header,
	// refused without it. GetVersion needs no dependencies, so the only
	// thing that can turn it away is the requirement itself.
	const strict = "/career.v1.SystemService/GetVersion"
	if got := post(strict, true); got != http.StatusOK {
		t.Errorf("%s with the header returned %d, want 200", strict, got)
	}
	if got := post(strict, false); got == http.StatusOK {
		t.Errorf("%s without the header returned 200; the requirement is "+
			"not in effect, so the CSRF claim in auth.proto is false again",
			strict)
	}

	// The beacon-reachable services must answer without it, because
	// sendBeacon cannot send it. These reach zero-value handlers, so
	// anything other than a protocol rejection is a pass here: what is
	// being tested is that the request was *accepted* for dispatch.
	for _, procedure := range []string{
		"/career.v1.EventService/Record",
		"/career.v1.DecisionTestService/SubmitAnswer",
		"/career.v1.DecisionTestService/SubmitRecall",
	} {
		code := post(procedure, false)
		if code == http.StatusBadRequest {
			t.Errorf("%s was rejected with 400 for a missing "+
				"Connect-Protocol-Version. sendBeacon cannot set that "+
				"header, so this procedure's writes would be silently lost "+
				"when a tab closes mid-run.", procedure)
		}
	}
}

// The exemption is only acceptable because nothing privileged sits
// behind it. A MEMBER or ADMIN procedure on an exempt service would
// weaken CSRF defence on exactly the calls that matter, so the levels
// are asserted rather than assumed.
func TestOnlyBeaconReachableServicesAreExempt(t *testing.T) {
	policy, err := authpolicy.Build()
	if err != nil {
		t.Fatalf("build the policy: %v", err)
	}

	exempt := map[string]bool{
		"career.v1.EventService":        true,
		"career.v1.DecisionTestService": true,
	}

	seen := map[string]int{}
	for procedure, pol := range policy {
		cut := strings.LastIndex(procedure, "/")
		if cut <= 0 {
			continue
		}
		svc := strings.TrimPrefix(procedure[:cut], "/")
		if !exempt[svc] {
			continue
		}
		seen[svc]++
		if pol.Level != v1.AuthLevel_AUTH_LEVEL_PUBLIC {
			t.Errorf("%s declares %s on a service exempt from the "+
				"Connect-Protocol-Version requirement. The exemption exists "+
				"because sendBeacon cannot set headers, and it is only safe "+
				"while every procedure behind it is PUBLIC: a privileged one "+
				"would lose a layer of CSRF defence. Either gate it "+
				"elsewhere or stop exempting this service.",
				procedure, pol.Level)
		}
	}

	for svc := range exempt {
		if seen[svc] == 0 {
			t.Errorf("no procedures found for exempt service %s; the name is "+
				"wrong and this test is checking nothing", svc)
		}
	}
}
