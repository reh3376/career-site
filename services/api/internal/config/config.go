// Package config reads runtime configuration from the environment.
// One config struct, one loader; values are read once at start.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr            string
	Env             string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	SidecarAddr     string
	SidecarTimeout  time.Duration
	DatabaseURL     string
	// DatabaseURLReadonly is an optional DSN used by the /admin/db
	// surface. When set, the SQL console runs through this pool
	// instead of the write-capable app pool, so the SELECT-only guard
	// in adminquery is defence in depth rather than the only line.
	// Empty → the console falls back to the main pool with a warn
	// log at boot.
	DatabaseURLReadonly string
	// DBReadonlyPassword, when set, is applied at boot via
	// `ALTER ROLE career_admin_readonly LOGIN PASSWORD '...'` so the
	// role created by the migration becomes usable. Keeps the
	// bootstrap out of migration files (secrets never land in git).
	DBReadonlyPassword string
	DBTimeout          time.Duration
	SkipMigrate        bool

	// Auth / email
	WebBaseURL        string
	OwnerContactEmail string
	MailFrom          string
	ConsentVersion    string
	EmailProvider     string // "smtp" (default in dev) or "resend"
	SMTPHost          string
	SMTPPort          int
	SMTPUser          string
	SMTPPass          string
	ResendAPIKey      string
	PwnedCheckEnabled bool

	// Scheduler
	ExpirySchedulerInterval time.Duration
	// PendingApprovalTTL bounds how long a verified user may sit in
	// pending_approval before auto-decline (FR-AUTH-16). Default 7 days.
	PendingApprovalTTL time.Duration

	// One-click Accept/Decline signing key (FR-AUTH-15).
	// Empty in dev auto-generates a random one at boot (with a warn log)
	// so `docker compose up` works without extra config; production sets a
	// stable value.
	DecisionTokenSecret []byte
	DecisionTokenTTL    time.Duration

	// Sessions (FR-AUTH-08)
	SessionTTL   time.Duration
	CookieSecure bool

	// Admin bootstrap. When both are set the API ensures a user with
	// AdminEmail exists in role=admin, status=active on every boot,
	// re-hashing AdminPassword when it differs. Empty in dev; set in prod.
	AdminEmail    string
	AdminPassword string
}

func Load() (Config, error) {
	cfg := Config{
		Addr:                envOr("API_ADDR", ":8080"),
		Env:                 envOr("API_ENV", "development"),
		ReadTimeout:         15 * time.Second,
		WriteTimeout:        30 * time.Second,
		ShutdownTimeout:     10 * time.Second,
		SidecarAddr:         envOr("SIDECAR_ADDR", "localhost:50051"),
		SidecarTimeout:      2 * time.Second,
		DatabaseURL:         envOr("DATABASE_URL", "postgres://career:career_dev_only@localhost:5432/career?sslmode=disable"),
		DatabaseURLReadonly: os.Getenv("DATABASE_URL_READONLY"),
		DBReadonlyPassword:  os.Getenv("DB_READONLY_PASSWORD"),
		DBTimeout:           2 * time.Second,
		SkipMigrate:         os.Getenv("API_SKIP_MIGRATE") == "1",

		WebBaseURL:        envOr("WEB_BASE_URL", "http://localhost"),
		OwnerContactEmail: envOr("OWNER_CONTACT_EMAIL", "rogerhenley345@gmail.com"),
		MailFrom:          envOr("MAIL_FROM", "career-site <noreply@career-site.local>"),
		ConsentVersion:    os.Getenv("CONSENT_VERSION"),
		EmailProvider:     envOr("EMAIL_PROVIDER", "smtp"),
		SMTPHost:          envOr("SMTP_HOST", "mailpit"),
		SMTPPort:          envIntOr("SMTP_PORT", 1025),
		SMTPUser:          os.Getenv("SMTP_USER"),
		SMTPPass:          os.Getenv("SMTP_PASS"),
		ResendAPIKey:      os.Getenv("RESEND_API_KEY"),
		PwnedCheckEnabled: os.Getenv("PWNED_CHECK_ENABLED") == "1",

		ExpirySchedulerInterval: time.Duration(envIntOr("EXPIRY_INTERVAL_SECONDS", 3600)) * time.Second,
		PendingApprovalTTL:      time.Duration(envIntOr("PENDING_APPROVAL_TTL_HOURS", 24*7)) * time.Hour,

		DecisionTokenTTL: time.Duration(envIntOr("DECISION_TOKEN_TTL_HOURS", 24*7)) * time.Hour,

		SessionTTL:   time.Duration(envIntOr("SESSION_TTL_HOURS", 24*30)) * time.Hour,
		CookieSecure: os.Getenv("COOKIE_SECURE") == "1",

		AdminEmail:    strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_USERNAME"))),
		AdminPassword: os.Getenv("CAREER_SITE_ADMIN_PW"),
	}
	if secretHex := os.Getenv("DECISION_TOKEN_SECRET"); secretHex != "" {
		s, err := hexDecode(secretHex)
		if err != nil {
			return Config{}, fmt.Errorf("DECISION_TOKEN_SECRET: %w", err)
		}
		cfg.DecisionTokenSecret = s
	}
	if v := os.Getenv("API_READ_TIMEOUT_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("API_READ_TIMEOUT_SECONDS: %w", err)
		}
		cfg.ReadTimeout = time.Duration(n) * time.Second
	}
	if v := os.Getenv("API_WRITE_TIMEOUT_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("API_WRITE_TIMEOUT_SECONDS: %w", err)
		}
		cfg.WriteTimeout = time.Duration(n) * time.Second
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func hexDecode(s string) ([]byte, error) {
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s)/2; i++ {
		var hi, lo byte
		if err := hexNibble(s[2*i], &hi); err != nil {
			return nil, err
		}
		if err := hexNibble(s[2*i+1], &lo); err != nil {
			return nil, err
		}
		b[i] = hi<<4 | lo
	}
	return b, nil
}

func hexNibble(c byte, out *byte) error {
	switch {
	case c >= '0' && c <= '9':
		*out = c - '0'
	case c >= 'a' && c <= 'f':
		*out = c - 'a' + 10
	case c >= 'A' && c <= 'F':
		*out = c - 'A' + 10
	default:
		return fmt.Errorf("invalid hex character %q", c)
	}
	return nil
}
