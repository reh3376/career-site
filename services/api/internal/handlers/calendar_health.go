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

// Telling the owner that calendar booking has stopped working.
//
// When the Google credential dies, the scheduler stops returning slots,
// and the failure does not look like a failure: an empty calendar reads
// as "no availability", so a recruiter sees a working page offering
// nothing. "Book a meeting" is one of the three things the landing page
// asks people to do, which is why this is worth a job at all.
//
// # Why this stopped counting days
//
// The first version computed an expiry date: Google caps a refresh token
// at seven days while the OAuth consent screen is in Testing, so it
// warned two days before that deadline. That was right for exactly as
// long as the project stayed in Testing.
//
// The seven-day cap is a property of the Testing publishing status, not
// of being unverified, so publishing removes it. A job that keeps
// counting to seven days afterwards does not become harmless, it becomes
// a liar: it would mail a reconnect warning five days after every
// connection, for a deadline that no longer exists. A warning that is
// sometimes wrong is one the reader learns to skip, and then it is worth
// less than no warning at all.
//
// So this asks the credential instead of predicting it. Every run calls
// Healthy, which performs a real Google call through the same wrapper
// that records connection health. The answer is evidence rather than
// arithmetic, and it is correct whether the project is in Testing,
// published, or revoked by hand.
//
// Two useful consequences fall out of probing rather than counting.
//
// The probe is also a keep-alive. Google expires a refresh token that
// has gone unused for six months; a credential exercised every six hours
// can never reach that, so the one remaining time-based expiry defends
// itself and needs no clock here either.
//
// And the warning now describes something that has happened rather than
// something predicted, so it can carry Google's own error text. "Booking
// is down, here is what Google said" is actionable in a way that "a
// deadline is approaching" never was.
//
// The cost, stated plainly: this reports after the break rather than
// before it, up to one scheduler interval late. That is the honest trade
// once the deadline it used to predict may not exist.
//
// # Not re-sending on restart
//
// ExpiryJobs learned this the hard way: a per-process map of who had
// been notified looked sufficient, restarts turned out to be dozens a
// day, and two members received 30 and 26 identical emails. A marker in
// a process records what that process did, not what the person received.
//
// So the marker is a row in app_settings holding the connection's
// updated_at. Reconnecting changes updated_at and re-arms the warning
// with no reset step. A recovery clears the marker, so the next failure
// is reported rather than swallowed as a duplicate of the last one.

// calendarDownKey holds the connection timestamp a failure notice was
// last sent for. It replaces calendar_expiry_warned_for, which the
// day-counting version used; an old row under that key is inert.
const calendarDownKey = "calendar_health_warned_for"

// CalendarHealthJob reports a calendar credential that has stopped
// working.
type CalendarHealthJob struct {
	log       *slog.Logger
	users     *users.Repo
	email     email.Provider
	from      string
	ownerAddr string
	webBase   string

	// probe performs a real call against the credential. nil means the
	// calendar is not configured at all, which is not a failure to warn
	// about: there is nothing connected to break.
	probe func(context.Context) error
}

func NewCalendarHealthJob(
	log *slog.Logger, repo *users.Repo, mailer email.Provider,
	from, ownerAddr, webBase string, probe func(context.Context) error,
) *CalendarHealthJob {
	return &CalendarHealthJob{
		log: log, users: repo, email: mailer,
		from: from, ownerAddr: ownerAddr, webBase: webBase,
		probe: probe,
	}
}

// Run probes the credential and reports a failure once.
//
// Safe on any cadence and safe to run repeatedly: the app_settings
// marker is what stops a second send, not the interval.
func (j *CalendarHealthJob) Run(ctx context.Context) error {
	conn, ok, err := j.users.GetCalendarConnection(ctx)
	if err != nil {
		return fmt.Errorf("calendar health: read connection: %w", err)
	}
	if !ok || !conn.Connected() {
		// Nothing connected. A scheduler that was never set up is a
		// different problem and the admin console already shows it.
		return nil
	}
	if j.probe == nil {
		return nil
	}

	stamp := conn.UpdatedAt.UTC().Format(time.RFC3339)
	probeErr := j.probe(ctx)

	if probeErr == nil {
		// Working. Clear the marker so a later failure is reported
		// rather than mistaken for the one already sent.
		//
		// Cleared by writing the empty string rather than deleting the
		// row, because an RFC3339 stamp can never equal "", so the
		// comparison below treats it as "nothing sent yet". That avoids
		// a DeleteSetting on the repo for exactly one caller. The read
		// guard keeps a healthy calendar from writing every six hours
		// forever.
		if raw, found, err := j.users.GetSetting(ctx, calendarDownKey); err == nil && found {
			var prev string
			if json.Unmarshal(raw, &prev) == nil && prev != "" {
				if err := j.users.SetSetting(ctx, calendarDownKey, json.RawMessage(`""`), 0); err != nil {
					j.log.Warn("calendar health: could not clear the notice marker",
						slog.String("error", err.Error()))
				}
			}
		}
		return nil
	}

	// Keyed on the connection's own timestamp, so a reconnect re-arms
	// this automatically and a restart does not resend.
	if raw, found, err := j.users.GetSetting(ctx, calendarDownKey); err == nil && found {
		var already string
		if json.Unmarshal(raw, &already) == nil && already == stamp {
			return nil
		}
	}

	if j.email == nil || j.ownerAddr == "" {
		// Nothing to send with. Still record it, or every tick re-enters
		// this branch and logs again.
		j.log.Warn("calendar credential is failing and no mailer is configured",
			slog.String("error", probeErr.Error()))
	} else {
		age := time.Since(conn.UpdatedAt)
		subject := "Google Calendar booking has stopped working"
		body := fmt.Sprintf(
			"The calendar credential is being refused, so /meetings is returning no\n"+
				"slots. That does not look broken from outside: an empty calendar reads\n"+
				"as no availability, so a visitor sees a working page offering nothing.\n\n"+
				"What Google said:\n\n    %s\n\n"+
				"Connected %s ago, on %s.\n\n"+
				"Reconnect: %s/admin/scheduler\n\n%s",
			probeErr.Error(),
			roughAge(age), conn.UpdatedAt.Format("Monday 2 January at 15:04 MST"),
			j.webBase, expiryHint(age))

		if err := j.email.Send(ctx, email.Message{
			To: j.ownerAddr, From: j.from, Subject: subject, TextBody: body,
		}); err != nil {
			// Not recorded as sent, so the next tick tries again.
			return fmt.Errorf("calendar health: send: %w", err)
		}
		j.log.Info("calendar failure reported",
			slog.String("error", probeErr.Error()), slog.Duration("connection_age", age))
	}

	v, _ := json.Marshal(stamp)
	if err := j.users.SetSetting(ctx, calendarDownKey, v, 0); err != nil {
		return fmt.Errorf("calendar health: record notice: %w", err)
	}
	return nil
}

// expiryHint names the likeliest cause when the timing fits Google's
// Testing-mode cap. It is a hint drawn from the connection's age, not a
// diagnosis: the job no longer assumes a publishing status, and this
// line is the one place the seven days are still mentioned.
func expiryHint(age time.Duration) string {
	if age < 6*24*time.Hour || age > 9*24*time.Hour {
		return "If this keeps happening, check the connection at the link above.\n"
	}
	return "This failed at about seven days old, which is what Google's Testing\n" +
		"publishing status caps a refresh token at. If the project is still in\n" +
		"Testing, publishing it ends this permanently: Audience, then Publish app.\n" +
		"Publishing is not the same as submitting for verification.\n"
}

// roughAge prints a duration the way a person would say it.
func roughAge(d time.Duration) string {
	switch days := int(d.Hours() / 24); {
	case days >= 2:
		return fmt.Sprintf("%d days", days)
	case days == 1:
		return "a day"
	default:
		return fmt.Sprintf("%d hours", int(d.Hours()))
	}
}
