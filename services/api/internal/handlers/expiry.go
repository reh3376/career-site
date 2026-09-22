package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/reh3376/career-site/services/api/internal/email"
	"github.com/reh3376/career-site/services/api/internal/events"
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
	events    *events.Writer // product event stream; nil is silent

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
		j.events.Emit(ctx, events.Event{Name: "access.expired", UserID: u.ID, Props: map[string]any{"user_id": u.ID}})
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
		"Name":            u.Name,
		"ExpiresAt":       formatExpiry(u),
		"OwnerEmail":      j.ownerAddr,
		"ExtensionMailto": extensionMailto(u, j.ownerAddr, "Extension request"),
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
		Kind:     "expiry_warn",
		UserID:   u.ID,
	})
}

func (j *ExpiryJobs) sendEnded(ctx context.Context, u *users.User) error {
	text, html, err := email.AccessEndedTemplate.Render(map[string]any{
		"Name":            u.Name,
		"ExpiresAt":       formatExpiry(u),
		"OwnerEmail":      j.ownerAddr,
		"ExtensionMailto": extensionMailto(u, j.ownerAddr, "Extension request"),
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
		Kind:     "expired",
		UserID:   u.ID,
	})
}

// extensionMailto builds a mailto: URL that opens the user's mail client
// with Roger's address pre-filled plus a subject that identifies who's
// asking. Body is left generic; the user can add context.
func extensionMailto(u *users.User, ownerAddr, subjectPrefix string) string {
	subject := url.QueryEscape(fmt.Sprintf("%s: %s", subjectPrefix, u.Name))
	body := url.QueryEscape(fmt.Sprintf(
		"Hi Roger,\n\nI'd like to request an extension of my career-site access (%s).\n\nThanks,\n%s",
		u.Email, u.Name,
	))
	return fmt.Sprintf("mailto:%s?subject=%s&body=%s", ownerAddr, subject, body)
}

func formatExpiry(u *users.User) string {
	if u.ExpiresAt == nil {
		return "an unspecified time"
	}
	return u.ExpiresAt.UTC().Format("Mon, 02 Jan 2006 15:04 UTC")
}

// AutoDeclineJobs bundles the FR-AUTH-16 auto-decline path: any user still
// pending_approval whose verify email was used more than PendingTTL ago
// gets flipped to `declined` with a `scheduler` audit trail, and receives
// the auto-declined notice.
type AutoDeclineJobs struct {
	log        *slog.Logger
	users      *users.Repo
	decision   *AdminDecision
	pendingTTL time.Duration
	events     *events.Writer // product event stream; nil is silent
}

func NewAutoDeclineJobs(log *slog.Logger, repo *users.Repo, decision *AdminDecision, pendingTTL time.Duration) *AutoDeclineJobs {
	if pendingTTL == 0 {
		pendingTTL = 7 * 24 * time.Hour
	}
	return &AutoDeclineJobs{
		log:        log,
		users:      repo,
		decision:   decision,
		pendingTTL: pendingTTL,
	}
}

func (j *AutoDeclineJobs) Run(ctx context.Context) error {
	list, err := j.users.PendingOlderThan(ctx, j.pendingTTL)
	if err != nil {
		return err
	}
	for _, u := range list {
		if err := j.users.SetStatus(ctx, u.ID, users.StatusDeclined); err != nil {
			j.log.Warn("auto-decline flip failed",
				slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
			continue
		}
		if err := j.users.RecordApprovalDecision(ctx, users.ApprovalDecision{
			UserID:     u.ID,
			Decision:   "auto_decline",
			DecidedVia: "scheduler",
		}); err != nil {
			j.log.Warn("auto-decline record failed",
				slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
		}
		j.events.Emit(ctx, events.Event{Name: "approval.auto_declined", UserID: u.ID, Props: map[string]any{"user_id": u.ID}})
		if err := j.decision.SendUserAutoDeclined(ctx, u); err != nil {
			j.log.Warn("auto-decline email failed",
				slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
		}
	}
	return nil
}
