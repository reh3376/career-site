package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/reh3376/career-site/services/api/internal/email"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Reminding the owner to reconnect Google Calendar before booking breaks.
//
// While the OAuth consent screen is in Testing, Google expires the
// refresh token seven days after it is issued. When it goes, the
// scheduler stops returning slots, and the failure does not look like a
// failure: an empty calendar reads as "no availability", so a recruiter
// sees a working page offering nothing. "Book a meeting" is one of the
// three things the landing page asks people to do.
//
// This is deliberately not a blind weekly reminder. A fixed weekly
// nudge drifts out of phase with the token, so it arrives three days
// early one week and a day late the next, and a reminder that is
// sometimes too late is one the reader learns to ignore. This reads the
// connection's own age and fires once, near the end, saying how long is
// actually left.
//
// # Not re-sending on restart
//
// ExpiryJobs learned this the hard way: a per-process map of who had
// been notified looked sufficient, restarts turned out to be dozens a
// day, and two members received 30 and 26 identical emails. A marker in
// a process records what that process did, not what the person
// received.
//
// So the marker is a row in app_settings, and it stores the connection's
// updated_at rather than a boolean. Reconnecting changes updated_at,
// which re-arms the reminder with no reset step and nothing to forget.

// calendarWarnedKey holds the connection timestamp a reminder was last
// sent for.
const calendarWarnedKey = "calendar_expiry_warned_for"

// CalendarExpiryJob warns before the Google refresh token dies.
type CalendarExpiryJob struct {
	log       *slog.Logger
	users     *users.Repo
	email     email.Provider
	from      string
	ownerAddr string
	webBase   string

	// tokenLife is how long Google leaves a refresh token valid while
	// the consent screen is unverified. Seven days, observed twice: the
	// connection made 2026-09-30 died on schedule.
	tokenLife time.Duration
	// warnWithin is how close to expiry the reminder fires. Two days is
	// enough to act on without being so early that the message is stale
	// by the time it matters.
	warnWithin time.Duration
}

func NewCalendarExpiryJob(
	log *slog.Logger, repo *users.Repo, mailer email.Provider,
	from, ownerAddr, webBase string,
) *CalendarExpiryJob {
	return &CalendarExpiryJob{
		log: log, users: repo, email: mailer,
		from: from, ownerAddr: ownerAddr, webBase: webBase,
		tokenLife:  7 * 24 * time.Hour,
		warnWithin: 2 * 24 * time.Hour,
	}
}

// Run sends one reminder per connection, near the end of its life.
//
// Safe on any cadence and safe to run repeatedly: the app_settings
// marker is what stops a second send, not the interval.
func (j *CalendarExpiryJob) Run(ctx context.Context) error {
	conn, ok, err := j.users.GetCalendarConnection(ctx)
	if err != nil {
		return fmt.Errorf("calendar expiry: read connection: %w", err)
	}
	if !ok || !conn.Connected() {
		// Nothing to warn about. A disconnected scheduler is a
		// different problem and the admin console already shows it.
		return nil
	}

	expires := conn.UpdatedAt.Add(j.tokenLife)
	left := time.Until(expires)
	if left > j.warnWithin {
		return nil
	}

	// Keyed on the connection's own timestamp, so a reconnect re-arms
	// this automatically and a restart does not resend.
	stamp := conn.UpdatedAt.UTC().Format(time.RFC3339)
	if raw, found, err := j.users.GetSetting(ctx, calendarWarnedKey); err == nil && found {
		var already string
		if json.Unmarshal(raw, &already) == nil && already == stamp {
			return nil
		}
	}

	if j.email == nil || j.ownerAddr == "" {
		// Nothing to send with. Still record the attempt, or every tick
		// re-enters this branch and logs again.
		j.log.Warn("calendar credential expiring and no mailer configured",
			slog.Time("expires", expires))
	} else {
		subject := "Reconnect Google Calendar: booking stops " + expires.Format("Mon 2 Jan")
		body := fmt.Sprintf(
			"The Google Calendar credential expires %s, in about %d hours.\n\n"+
				"When it goes, /meetings returns no slots. That does not look broken: an\n"+
				"empty calendar reads as no availability, so a visitor sees a working page\n"+
				"offering nothing.\n\n"+
				"Reconnect: %s/admin/scheduler\n\n"+
				"This recurs because the OAuth consent screen is unverified, which caps the\n"+
				"refresh token at seven days. Publishing the consent screen through Google\n"+
				"verification ends it; this reminder only makes it survivable.\n",
			expires.Format("Monday 2 January at 15:04 MST"), int(left.Hours()), j.webBase)

		if err := j.email.Send(ctx, email.Message{
			To: j.ownerAddr, From: j.from, Subject: subject, TextBody: body,
		}); err != nil {
			// Not recorded as sent, so the next tick tries again.
			return fmt.Errorf("calendar expiry: send: %w", err)
		}
		j.log.Info("calendar reconnect reminder sent",
			slog.Time("expires", expires), slog.Duration("left", left))
	}

	v, _ := json.Marshal(stamp)
	if err := j.users.SetSetting(ctx, calendarWarnedKey, v, 0); err != nil {
		return fmt.Errorf("calendar expiry: record notice: %w", err)
	}
	return nil
}
