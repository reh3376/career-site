package config

import "testing"

// The Turnstile secret is the first setting this service treats as
// boot-critical, so these tests pin the behavior rather than the
// wording: production without a usable secret must not produce a
// Config at all.
//
// Why a boot failure and not a warn log: the FSD declares registration
// and anonymous contact bot-protected (FR-AUTH-09, FR-CNT-22). A warn
// log would let the service run with the declaration intact and the
// check inert, which is the precise failure ADR 0031 was written after
// finding ten times over.

func TestProductionRefusesToStartWithoutATurnstileSecret(t *testing.T) {
	cases := []struct {
		name   string
		secret string
	}{
		{"unset", ""},
		{"truncated paste", "0x4AAAAAAA"},
		{"quoted in .env.prod", `"0x4AAAAAAABBBBBBBBCCCCCCCCDDDDDDDD"`},
		{"trailing comment glued on", "0x4AAAAAAABBBBBBBBCCCCCCCC # key"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{Env: "production", TurnstileSecretKey: tc.secret}
			if err := cfg.requireTurnstileInProduction(); err == nil {
				t.Fatalf("production accepted TURNSTILE_SECRET_KEY=%q; it must refuse to boot", tc.secret)
			}
		})
	}
}

func TestProductionStartsWithAPlausibleTurnstileSecret(t *testing.T) {
	cfg := Config{Env: "production", TurnstileSecretKey: "0x4AAAAAAABBBBBBBBCCCCCCCCDDDDDDDD"}
	if err := cfg.requireTurnstileInProduction(); err != nil {
		t.Fatalf("production rejected a well-formed secret: %v", err)
	}
}

// Dev and the docker stack must still come up with nothing configured,
// or `docker compose up` would need a Cloudflare account. turnstile.Noop
// covers those environments.
func TestNonProductionDoesNotRequireATurnstileSecret(t *testing.T) {
	for _, env := range []string{"development", "docker", "test"} {
		cfg := Config{Env: env}
		if err := cfg.requireTurnstileInProduction(); err != nil {
			t.Fatalf("API_ENV=%s refused to start without a Turnstile secret: %v", env, err)
		}
	}
}

// Load is where the gate has to live for it to matter: a check that
// exists but is never called from Load is the same as no check. This
// fails if someone removes the call at the end of Load.
func TestLoadAppliesTheTurnstileGate(t *testing.T) {
	t.Setenv("API_ENV", "production")
	t.Setenv("TURNSTILE_SECRET_KEY", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load returned a production Config with no Turnstile secret; " +
			"the requireTurnstileInProduction call in Load is missing or unreachable")
	}

	t.Setenv("TURNSTILE_SECRET_KEY", "0x4AAAAAAABBBBBBBBCCCCCCCCDDDDDDDD")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load rejected a production Config with a valid secret: %v", err)
	}
	if cfg.TurnstileSecretKey == "" {
		t.Fatal("Load did not read TURNSTILE_SECRET_KEY into the Config")
	}
}
