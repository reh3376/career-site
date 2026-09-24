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
	// CorpusRoot is the base directory the corpus filesystem walker
	// reads from when an admin hits ReindexCorpus. Each source_kind
	// resolves to a subdirectory (e.g. `article` → `<root>/articles`).
	// In prod this is a docker bind-mount of `apps/web/content` to
	// `/corpus:ro`, so the walker sees the same markdown that ships
	// with the web bundle.
	CorpusRoot string
	// CorpusPrivateRoot is the bind-mount of the curated private corpus
	// (synced from docs/personal by `make sync-corpus`). Each top-level
	// directory under it is a source_kind; everything ingested from it
	// is visibility=corpus_only.
	CorpusPrivateRoot string
	// LLMMonthlyCallCap bounds LLM gateway calls per calendar month
	// (Phase 4 guardrail #8). 0 = unlimited; set it in prod once a
	// metered provider is in play.
	LLMMonthlyCallCap int64
	// JDMatchThreshold is the requirement-weighted score at or above
	// which a JD gets a tailored résumé. It is model-dependent (each
	// judge model reads evidence with its own strictness), so it lives
	// in .env.prod next to OLLAMA_LLM_MODEL and is recalibrated with
	// it; docs/llm-tuning-log.md records the value per model.
	JDMatchThreshold float64
	// JDDailyLimit caps how many postings one member may submit in a
	// rolling 24 hours. A submission is roughly an hour of inference on
	// this box, so the ceiling is capacity, not politeness: without it
	// one member with a script takes the site down. 0 disables the cap.
	JDDailyLimit int
	// JDPipelineTimeout bounds one submission's score + generate run.
	// CPU inference of a two-page résumé can take minutes.
	JDPipelineTimeout time.Duration
	// LLMNumCtx is the model context window the sidecar requests; the
	// API budgets its judgment batches and résumé evidence to it so the
	// pipeline fits the box (8192 on the CPX31 with qwen3:8b).
	LLMNumCtx int
	// LLMAllowStub lets the structured JD assessor run against the
	// sidecar's stub provider (schema-valid, meaningless output). Off
	// by default so a stub never gates real submissions.
	LLMAllowStub bool
	// ResumePDFOwnerPassword locks editing on generated résumé PDFs
	// (the user password is empty, so they open freely). Empty skips
	// PDF rendering; never commit the value, the repo is public.
	ResumePDFOwnerPassword string
	DatabaseURL            string
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

	// Product event stream (docs/events/README.md). EventIPSalt keys
	// the client-address hash stored with each event; empty falls back
	// to a fixed dev salt with a warning. EventIdentityRetention is how
	// long identity columns (user, session, anon id, address hash) stay
	// on a row before the nightly job blanks them; the row itself is
	// kept for aggregate counts.
	EventIPSalt            string
	EventIdentityRetention time.Duration

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
		Addr:                   envOr("API_ADDR", ":8080"),
		Env:                    envOr("API_ENV", "development"),
		ReadTimeout:            15 * time.Second,
		WriteTimeout:           30 * time.Second,
		ShutdownTimeout:        10 * time.Second,
		SidecarAddr:            envOr("SIDECAR_ADDR", "localhost:50051"),
		SidecarTimeout:         2 * time.Second,
		CorpusRoot:             envOr("CORPUS_ROOT", "/corpus"),
		CorpusPrivateRoot:      envOr("CORPUS_PRIVATE_ROOT", "/corpus-private"),
		LLMMonthlyCallCap:      int64(envIntOr("LLM_MONTHLY_CALL_CAP", 0)),
		JDMatchThreshold:       envFloatOr("JD_MATCH_THRESHOLD", 0.55),
		JDDailyLimit:           envIntOr("JD_DAILY_LIMIT", 5),
		JDPipelineTimeout:      time.Duration(envIntOr("JD_PIPELINE_TIMEOUT_SECONDS", 900)) * time.Second,
		LLMAllowStub:           os.Getenv("LLM_ALLOW_STUB") == "1",
		LLMNumCtx:              envIntOr("LLM_NUM_CTX", 16384),
		ResumePDFOwnerPassword: os.Getenv("RESUME_PDF_OWNER_PASSWORD"),
		DatabaseURL:            envOr("DATABASE_URL", "postgres://career:career_dev_only@localhost:5432/career?sslmode=disable"),
		DatabaseURLReadonly:    os.Getenv("DATABASE_URL_READONLY"),
		DBReadonlyPassword:     os.Getenv("DB_READONLY_PASSWORD"),
		DBTimeout:              2 * time.Second,
		SkipMigrate:            os.Getenv("API_SKIP_MIGRATE") == "1",

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

		EventIPSalt:            os.Getenv("EVENT_IP_SALT"),
		EventIdentityRetention: time.Duration(envIntOr("EVENT_IDENTITY_RETENTION_DAYS", 400)) * 24 * time.Hour,
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

func envFloatOr(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
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
