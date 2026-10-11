package config

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Does every setting the api reads actually reach the container?
//
// **Why this exists.** On 2026-10-11 the auth interceptor's mode was
// added as `AUTH_INTERCEPTOR_MODE`, documented as "S4 is one line in
// .env.prod and a restart", and the line was duly added. Nothing
// happened. The api service enumerates its environment in
// docker-compose.yml, so a variable set in `.env.prod` and not listed
// there never reaches the process: compose saw no change to the service,
// did not recreate the container, and the server went on reading the
// default.
//
// The failure mode is the dangerous kind. The config field existed, the
// code read it, the tests passed, the deploy succeeded, and the setting
// was silently inert. Every gate in the repo was green while the thing
// the gate was about did not work, which is the same shape as the UFW
// rule that filtered nothing and the auth options declared and never
// enforced.
//
// So: anything `config.Load` reads must either be passed by a compose
// file or be listed below as deliberately defaulted.

// defaultedInBothEnvironments are settings the api reads and neither
// compose file passes, on purpose. Every one is a tuning knob whose
// default is the intended value in dev and in production; none of them
// changes who can call what.
//
// Adding to this list is a decision: it says "this setting is expected
// to be its default everywhere, including production". A setting meant
// to be *set* in production belongs in docker-compose.prod.yml instead.
var defaultedInBothEnvironments = map[string]string{
	"API_READ_TIMEOUT_SECONDS":   "server timeout, default is right",
	"API_WRITE_TIMEOUT_SECONDS":  "server timeout, default is right",
	"API_SKIP_MIGRATE":           "an escape hatch for local work; production always migrates",
	"CONSENT_VERSION":            "versioned in code, not per environment",
	"DECISION_TOKEN_TTL_HOURS":   "tuning knob",
	"EXPIRY_INTERVAL_SECONDS":    "tuning knob",
	"JD_DAILY_LIMIT":             "tuning knob",
	"PENDING_APPROVAL_TTL_HOURS": "tuning knob",
	"SESSION_TTL_HOURS":          "tuning knob",
	// Production sends through Resend and dev through mailpit, so
	// neither path authenticates to an SMTP server.
	"SMTP_USER": "unused: resend in production, mailpit in dev",
	"SMTP_PASS": "unused: resend in production, mailpit in dev",
}

// devOnly are passed by docker-compose.yml and deliberately absent from
// production, where their defaults are the safe values.
//
// This list is why the check below looks at the *production* compose
// file rather than accepting either. "Set in dev, missing in prod" is
// the same silent-default bug as "missing from both", and it is easier
// to introduce: the local stack works, so nothing looks wrong.
var devOnly = map[string]string{
	"DT_ALLOW_FIXTURE_BANK": "dev draws the decision test from the placeholder bank; production must not",
	"LLM_ALLOW_STUB":        "dev may stub the model; production answers for real or fails",
	"SMTP_HOST":             "mailpit in dev; production sends through Resend",
	"SMTP_PORT":             "mailpit in dev; production sends through Resend",
}

func TestEverySettingTheApiReadsReachesTheContainer(t *testing.T) {
	root := repoRoot(t)

	read := envVarsReadByConfig(t, filepath.Join(root, "services", "api", "internal", "config", "config.go"))
	// A regex that stops matching would empty this set and the
	// comparison would pass over nothing. The real count is in the
	// forties.
	if len(read) < 30 {
		t.Fatalf("found only %d environment variables read by config.go, "+
			"expected 40 or more: the scan is broken and this test is "+
			"asserting nothing", len(read))
	}

	dev := apiEnvKeys(t, filepath.Join(root, "docker-compose.yml"))
	prod := apiEnvKeys(t, filepath.Join(root, "docker-compose.prod.yml"))
	for name, keys := range map[string]map[string]bool{
		"docker-compose.yml":      dev,
		"docker-compose.prod.yml": prod,
	} {
		if len(keys) < 15 {
			t.Fatalf("found only %d environment keys for the api service in "+
				"%s, expected 25 or more: the YAML scan is broken", len(keys), name)
		}
	}

	// Checked against production specifically. Accepting either file
	// would let "set in dev, missing in prod" through, which is the same
	// silent default and easier to introduce, because the local stack
	// works and nothing looks wrong.
	var unreachable []string
	for _, v := range sortedSet(read) {
		if prod[v] {
			continue
		}
		if _, ok := defaultedInBothEnvironments[v]; ok {
			continue
		}
		if _, ok := devOnly[v]; ok {
			continue
		}
		unreachable = append(unreachable, v)
	}

	if len(unreachable) > 0 {
		t.Errorf("config.Load reads %d setting(s) that docker-compose.prod.yml "+
			"does not pass to the api container, and that are on neither "+
			"exemption list:\n  %s\n\n"+
			"Each one is silently stuck at its default in production, whatever "+
			".env.prod says. Either add it to docker-compose.prod.yml, or add "+
			"it to defaultedInBothEnvironments or devOnly with the reason.",
			len(unreachable), strings.Join(unreachable, "\n  "))
	}

	// The exemption list must not rot. An entry that is now passed by a
	// compose file is a contradiction: it says "expected to be its
	// default everywhere" while being set.
	for v, why := range defaultedInBothEnvironments {
		if !read[v] {
			t.Errorf("%s is on defaultedInBothEnvironments (%q) but config.go "+
				"no longer reads it; remove the entry", v, why)
		}
		if dev[v] || prod[v] {
			t.Errorf("%s is on defaultedInBothEnvironments (%q) but a compose "+
				"file now passes it; remove the entry", v, why)
		}
	}
	for v, why := range devOnly {
		if !read[v] {
			t.Errorf("%s is on devOnly (%q) but config.go no longer reads it; "+
				"remove the entry", v, why)
		}
		if prod[v] {
			t.Errorf("%s is on devOnly (%q) but production now passes it; "+
				"remove the entry", v, why)
		}
		if !dev[v] {
			t.Errorf("%s is on devOnly (%q) but docker-compose.yml does not "+
				"pass it either, so it is defaulted everywhere and belongs on "+
				"defaultedInBothEnvironments", v, why)
		}
	}
}

// The setting that prompted this test was AUTH_INTERCEPTOR_MODE, and it
// no longer exists: S5 removed the interceptor's observe mode, because
// with the handler gates gone a mode that observes without acting would
// have left 48 admin methods open. The lesson it taught is what this
// file keeps.

var envReadRe = regexp.MustCompile(`(?:os\.Getenv|envOr|envIntOr|envFloatOr)\("([A-Z_0-9]+)"`)

func envVarsReadByConfig(t *testing.T, path string) map[string]bool {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	out := map[string]bool{}
	for _, m := range envReadRe.FindAllStringSubmatch(string(body), -1) {
		out[m[1]] = true
	}
	return out
}

var (
	serviceRe = regexp.MustCompile(`^  ([A-Za-z0-9_-]+):\s*$`)
	envBlock  = regexp.MustCompile(`^    environment:\s*$`)
	siblingRe = regexp.MustCompile(`^    [A-Za-z0-9_-]+:`)
	envKeyRe  = regexp.MustCompile(`^      ([A-Z_0-9]+):`)
)

// apiEnvKeys reads the environment keys of the `api` service.
//
// Line-based rather than a YAML parse, so it needs no dependency and
// cannot be thrown off by the `${VAR:-default}` interpolation or the
// `<<: *common-env` merge key. The size floors in the test are what
// guard against it silently matching nothing.
func apiEnvKeys(t *testing.T, path string) map[string]bool {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	out := map[string]bool{}
	inAPI, inEnv := false, false
	for _, ln := range strings.Split(string(body), "\n") {
		if m := serviceRe.FindStringSubmatch(ln); m != nil {
			inAPI = m[1] == "api"
			inEnv = false
			continue
		}
		if !inAPI {
			continue
		}
		if envBlock.MatchString(ln) {
			inEnv = true
			continue
		}
		if inEnv {
			if siblingRe.MatchString(ln) {
				inEnv = false
				continue
			}
			if m := envKeyRe.FindStringSubmatch(ln); m != nil {
				out[m[1]] = true
			}
		}
	}
	return out
}

// repoRoot walks up until it finds docker-compose.yml, so the test does
// not depend on how deep the package sits.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for range 8 {
		if _, err := os.Stat(filepath.Join(dir, "docker-compose.yml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not find docker-compose.yml above the working directory")
	return ""
}

func sortedSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
