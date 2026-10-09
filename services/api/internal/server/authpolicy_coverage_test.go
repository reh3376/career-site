package server

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/authpolicy"
	"github.com/reh3376/career-site/services/api/internal/config"
	"github.com/reh3376/career-site/services/api/internal/handlers"
)

// routesWithPolicy mounts the routes with a given policy map and returns
// everything the server logged while doing it.
//
// The log is the whole output of S2, so it is what gets asserted. A test
// that only checked the map was built would not have caught the coverage
// logic being wrong, and the coverage logic is the part that reads the
// map the way the S3 interceptor will.
func routesWithPolicy(t *testing.T, policy authpolicy.Map) string {
	t.Helper()
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	New(config.Config{Addr: ":0"}, log, Deps{AuthPolicy: policy}).routes()
	return buf.String()
}

// With the real policy and no handlers wired, SystemService is the only
// service New mounts, so it is the only one covered and everything else
// is correctly reported as unreachable.
func TestPolicyCoverageReportsMountedAndUnmounted(t *testing.T) {
	policy, err := authpolicy.Build()
	if err != nil {
		t.Fatalf("build the policy: %v", err)
	}

	out := routesWithPolicy(t, policy)

	if !strings.Contains(out, "auth policy coverage") {
		t.Fatalf("the coverage line was never logged:\n%s", out)
	}
	// The summary has to carry real figures. "141 methods" rather than
	// a count of what happened to be mounted, because the map is the
	// whole contract and the mounted subset is a property of this
	// process.
	for _, want := range []string{"141 methods", "admin=81", "mounted_services=1"} {
		if !strings.Contains(out, want) {
			t.Errorf("coverage log is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "procedures_served=0") {
		t.Errorf("SystemService is mounted but no procedures were counted "+
			"against it, so the service name is being derived from the "+
			"procedure key wrongly:\n%s", out)
	}

	// The unmounted services from S1, named rather than counted.
	if !strings.Contains(out, "not mounted") {
		t.Fatalf("13 declared services are unmounted here and none were "+
			"reported:\n%s", out)
	}
	for _, want := range []string{
		"career.v1.HomeService",
		"career.v1.ContentService",
		"career.v1.DownloadService",
		// Mounted in production but not by this test, which is the
		// point: the check reports what this process serves, not what
		// the proto declares.
		"career.v1.AdminService",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("%s is unmounted and was not named:\n%s", want, out)
		}
	}

	// Nothing mounted is uncovered, so the error branch must stay quiet.
	if strings.Contains(out, "no declared auth policy") {
		t.Errorf("a mounted service was reported as having no policy, but "+
			"the real map covers every service:\n%s", out)
	}
}

// The production shape: every service main can mount, mounted.
//
// This is the case the boot log is actually read in, and it is the one
// that pins the arithmetic: 141 declared, 133 served across 11 services,
// 8 on the three services nothing mounts. The handlers are zero values
// because routes() only checks them for nil, and nothing here calls one.
func TestPolicyCoverageWithEveryServiceMounted(t *testing.T) {
	policy, err := authpolicy.Build()
	if err != nil {
		t.Fatalf("build the policy: %v", err)
	}

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	New(config.Config{Addr: ":0"}, log, Deps{
		AuthPolicy:   policy,
		Auth:         &handlers.Auth{},
		Member:       &handlers.Member{},
		Contact:      &handlers.Contact{},
		Admin:        &handlers.Admin{},
		Activity:     &handlers.Activity{},
		Jd:           &handlers.Jd{},
		Meetings:     &handlers.Meetings{},
		Chat:         &handlers.Chat{},
		Events:       &handlers.Events{},
		DecisionTest: &handlers.DecisionTest{},
	}).routes()
	out := buf.String()

	for _, want := range []string{
		"mounted_services=11",
		// 141 declared minus the 8 on services nothing mounts. Asserted
		// exactly: this is the number of procedures the S4 interceptor
		// will be deciding on, and it should not drift unnoticed.
		"procedures_served=133",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("coverage log is missing %q:\n%s", want, out)
		}
	}

	// Exactly three services unmounted, and exactly these three. A
	// fourth appearing here means a service was dropped from routes();
	// one disappearing means it was mounted, and it arrived with no
	// enforcement of its own.
	if !strings.Contains(out,
		"career.v1.ContentService,career.v1.DownloadService,career.v1.HomeService") {
		t.Errorf("the unmounted services are not exactly Content, Download "+
			"and Home:\n%s", out)
	}
	if strings.Contains(out, "no declared auth policy") {
		t.Errorf("a mounted service was reported as having no policy:\n%s", out)
	}
}

// The dangerous case: a service is served and the policy says nothing
// about it. Once S4 enforces, every procedure on it is denied, because
// an unknown procedure has to fail closed. It must be loud here.
func TestPolicyCoverageFlagsAMountedServiceWithNoPolicy(t *testing.T) {
	policy, err := authpolicy.Build()
	if err != nil {
		t.Fatalf("build the policy: %v", err)
	}

	// Drop SystemService, which is the one service this test mounts.
	stripped := authpolicy.Map{}
	for procedure, lvl := range policy {
		if strings.HasPrefix(procedure, "/career.v1.SystemService/") {
			continue
		}
		stripped[procedure] = lvl
	}
	if len(stripped) == len(policy) {
		t.Fatalf("nothing was stripped, so this test is not exercising the " +
			"uncovered path; the procedure key format has changed")
	}

	out := routesWithPolicy(t, stripped)

	if !strings.Contains(out, "no declared auth policy") {
		t.Fatalf("SystemService is mounted with no policy entries and "+
			"nothing was logged about it:\n%s", out)
	}
	if !strings.Contains(out, "career.v1.SystemService") {
		t.Errorf("the uncovered service was not named:\n%s", out)
	}
	if !strings.Contains(out, "level=ERROR") {
		t.Errorf("a mounted service with no policy was not logged at error; "+
			"it would be lost in a normal boot:\n%s", out)
	}
}

// A Server built without a policy says so rather than reporting a clean
// run over an empty map. Reachable only from tests, since main exits
// first, but "no policy" and "a policy covering nothing" must not look
// the same in a log.
func TestPolicyCoverageSaysSoWhenThereIsNoPolicy(t *testing.T) {
	out := routesWithPolicy(t, nil)

	if !strings.Contains(out, "no auth policy loaded") {
		t.Errorf("a nil policy was not reported:\n%s", out)
	}
	if strings.Contains(out, "auth policy coverage") {
		t.Errorf("coverage was reported for a nil policy:\n%s", out)
	}
}

// The level names in the log come from the generated enum, so a rename
// in the proto changes the log without changing this package. Pinned
// here because the boot line is what somebody greps during an incident.
func TestLevelNamesInTheLogAreTheProtoNames(t *testing.T) {
	for _, c := range []struct {
		level v1.AuthLevel
		want  string
	}{
		{v1.AuthLevel_AUTH_LEVEL_ADMIN, "AUTH_LEVEL_ADMIN"},
		{v1.AuthLevel_AUTH_LEVEL_MEMBER, "AUTH_LEVEL_MEMBER"},
		{v1.AuthLevel_AUTH_LEVEL_PUBLIC, "AUTH_LEVEL_PUBLIC"},
	} {
		if got := c.level.String(); got != c.want {
			t.Errorf("AuthLevel %d renders as %q, expected %q", c.level, got, c.want)
		}
	}
}
