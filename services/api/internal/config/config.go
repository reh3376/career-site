// Package config reads runtime configuration from the environment.
// One config struct, one loader; values are read once at start.
package config

import (
	"fmt"
	"os"
	"strconv"
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
	DBTimeout       time.Duration
	SkipMigrate     bool

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
}

func Load() (Config, error) {
	cfg := Config{
		Addr:            envOr("API_ADDR", ":8080"),
		Env:             envOr("API_ENV", "development"),
		ReadTimeout:     15 * time.Second,
		WriteTimeout:    30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		SidecarAddr:     envOr("SIDECAR_ADDR", "localhost:50051"),
		SidecarTimeout:  2 * time.Second,
		DatabaseURL:     envOr("DATABASE_URL", "postgres://career:career_dev_only@localhost:5432/career?sslmode=disable"),
		DBTimeout:       2 * time.Second,
		SkipMigrate:     os.Getenv("API_SKIP_MIGRATE") == "1",

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
