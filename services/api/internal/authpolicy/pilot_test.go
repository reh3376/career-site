package authpolicy

import (
	"strings"
	"testing"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
)

// FSD FR-PLT-12: no pilot answer, flag or comment reaches a public page.
// The data half of that promise is the absence of any public read path,
// and the cheapest place to hold it is here, where every declared level
// is already in one map: a PilotService method at AUTH_LEVEL_PUBLIC is a
// leak by declaration, whatever its handler does. The admin methods are
// pinned too, because an export that only needed a member session would
// hand one participant the others' answers.
func TestPilotServiceHasNoPublicMethod(t *testing.T) {
	m, err := Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	const prefix = "/career.v1.PilotService/"
	admin := map[string]bool{
		prefix + "ListPilotSessions": true,
		prefix + "GetPilotSession":   true,
		prefix + "ExportPilot":       true,
	}
	seen := 0
	for proc, p := range m {
		if !strings.HasPrefix(proc, prefix) {
			continue
		}
		seen++
		if p.Level == v1.AuthLevel_AUTH_LEVEL_PUBLIC {
			t.Errorf("%s is declared public; the pilot has no public surface (FR-PLT-12)", proc)
		}
		if admin[proc] && p.Level != v1.AuthLevel_AUTH_LEVEL_ADMIN {
			t.Errorf("%s is %s; the pilot's read-all and export methods must be admin", proc, p.Level)
		}
		if p.AllowUnverified {
			t.Errorf("%s allows an unverified account; a participant is a verified, whitelisted member", proc)
		}
	}
	if seen != 12 {
		t.Errorf("PilotService declares %d methods, expected 12 (nine member, three admin)", seen)
	}
}
