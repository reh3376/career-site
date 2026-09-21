// Package email owns transactional email delivery: provider interface,
// implementations (Resend for prod, SMTP for dev / Mailpit), and template
// rendering. Every message carries a plain-text alternative per FR-NOTF-01.
package email

import (
	"context"
	"errors"
	"fmt"
)

type Message struct {
	From     string // "Name <addr@domain>" or just "addr@domain"
	To       string
	Subject  string
	TextBody string
	HTMLBody string

	// Audit metadata. Providers ignore these; the Audited decorator
	// records them so the admin console can show per-member delivery
	// history. Kind is a short slug ("user_approved"); UserID is the
	// member the mail is about (0 for owner-bound mail); TriggeredBy
	// is "system" when empty, or "admin:<id>" for console resends.
	Kind        string
	UserID      int64
	TriggeredBy string
}

type Provider interface {
	Send(ctx context.Context, msg Message) error
	Name() string
}

// ErrNotConfigured indicates the provider was constructed but is missing
// credentials or config; callers use this to fall back to a dev provider.
var ErrNotConfigured = errors.New("email provider not configured")

// Config chooses the provider. `smtp` binds to Mailpit or any local relay;
// `resend` uses the Resend HTTPS API.
type Config struct {
	Provider  string // "smtp" (default in dev) or "resend"
	From      string // required
	SMTPHost  string
	SMTPPort  int
	SMTPUser  string
	SMTPPass  string
	ResendKey string
}

func NewFromConfig(cfg Config) (Provider, error) {
	if cfg.From == "" {
		return nil, fmt.Errorf("%w: EMAIL_FROM is required", ErrNotConfigured)
	}
	switch cfg.Provider {
	case "", "smtp":
		return NewSMTP(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.From)
	case "resend":
		return NewResend(cfg.ResendKey, cfg.From)
	default:
		return nil, fmt.Errorf("unknown EMAIL_PROVIDER %q", cfg.Provider)
	}
}
