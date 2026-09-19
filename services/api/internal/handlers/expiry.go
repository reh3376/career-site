package handlers

import (
	"context"
	"log/slog"
	"time"

	"github.com/reh3376/career-site/services/api/internal/email"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// ExpiryJobs holds the two recurring jobs FR-AUTH-18 requires: a 3-day
// warning and a hard cut at expiry. Both are idempotent and safe to run on
// any cadence — the queries filter to just-in-window rows.
type ExpiryJobs struct {
	log       *slog.Logger
	users     *users.Repo
	email     email.Provider
	from      string
	ownerAddr string

	warnWindow time.Duration // how far ahead of expiry the reminder fires (default 3d)
	// notifiedWarn tracks user IDs we've already emailed a warning for in
	// this process lifetime. In production this would be persisted on
	// `users` (e.g. `expiry_warning_sent_at`) so a restart doesn't
	// re-notify. Phase 1 keeps it in-memory — the worst case is a duplicate
	// warning email after a process restart, which is graceful.
	notifiedWarn map[int64]struct{}
}

func NewExpiryJobs(log *slog.Logger, repo *users.Repo, mailer email.Provider, from, ownerAddr string) *ExpiryJobs {
	return &ExpiryJobs{
		log:          log,
		users:        repo,
		email:        mailer,
		from:         from,
		ownerAddr:    ownerAddr,
		warnWindow:   3 * 24 * time.Hour,
		notifiedWarn: make(map[int64]struct{}),
	}
}

// WarnJob sends the 3-day-out reminder to any active user whose expires_at
// falls within warnWindow.
func (j *ExpiryJobs) WarnJob(ctx context.Context) error {
	list, err := j.users.ExpiringSoon(ctx, j.warnWindow)
	if err != nil {
		return err
	}
	for _, u := range list {
		if _, done := j.notifiedWarn[u.ID]; done {
			continue
		}
		if err := j.sendEnding(ctx, u); err != nil {
			j.log.Warn("send access-ending email failed",
				slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
			continue
		}
		j.notifiedWarn[u.ID] = struct{}{}
	}
	return nil
}

// ExpireJob flips every over-due active user to `expired`, revokes their
// sessions, and sends the ended-notice email. The DB update is guarded so
// a concurrent admin action (e.g. an admin manually re-approving with a
// fresh TTL) does not race.
func (j *ExpiryJobs) ExpireJob(ctx context.Context) error {
	list, err := j.users.Expired(ctx)
	if err != nil {
		return err
	}
	for _, u := range list {
		expired, err := j.users.ExpireOne(ctx, u.ID)
		if err != nil {
			j.log.Warn("expire user failed",
				slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
			continue
		}
		if !expired {
			continue // raced with another writer; already handled
		}
		if _, err := j.users.RevokeSessions(ctx, u.ID); err != nil {
			j.log.Warn("revoke sessions failed",
				slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
		}
		if err := j.sendEnded(ctx, u); err != nil {
			j.log.Warn("send access-ended email failed",
				slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
		}
		// Clear the warn set so a re-approved user's next expiry cycle
		// notifies again.
		delete(j.notifiedWarn, u.ID)
	}
	return nil
}

func (j *ExpiryJobs) sendEnding(ctx context.Context, u *users.User) error {
	text, html, err := email.AccessEndingSoonTemplate.Render(map[string]any{
		"Name":       u.Name,
		"ExpiresAt":  formatExpiry(u),
		"OwnerEmail": j.ownerAddr,
	})
	if err != nil {
		return err
	}
	return j.email.Send(ctx, email.Message{
		To:       u.Email,
		From:     j.from,
		Subject:  "Your career-site access ends in about 3 days",
		TextBody: text,
		HTMLBody: html,
	})
}

func (j *ExpiryJobs) sendEnded(ctx context.Context, u *users.User) error {
	text, html, err := email.AccessEndedTemplate.Render(map[string]any{
		"Name":       u.Name,
		"ExpiresAt":  formatExpiry(u),
		"OwnerEmail": j.ownerAddr,
	})
	if err != nil {
		return err
	}
	return j.email.Send(ctx, email.Message{
		To:       u.Email,
		From:     j.from,
		Subject:  "Your career-site access has ended",
		TextBody: text,
		HTMLBody: html,
	})
}

func formatExpiry(u *users.User) string {
	if u.ExpiresAt == nil {
		return "an unspecified time"
	}
	return u.ExpiresAt.UTC().Format("Mon, 02 Jan 2006 15:04 UTC")
}
