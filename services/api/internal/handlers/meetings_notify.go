package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/reh3376/career-site/services/api/internal/email"
	"github.com/reh3376/career-site/services/api/internal/scheduling"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// The owner hears when someone books.
//
// The calendar invitation already arrives, so this is not a duplicate
// of it: it carries what Google has no idea about. Who the member is
// as an account, how long they have had access, whether they have run
// the reviewer, and the one thing the calendar entry cannot say, that
// the slot is held here but no event was created.
//
// Sent after the booking is safely stored, never before, and failing to
// send never fails the booking. A meeting that exists and was not
// announced is recoverable from the scheduler page; a booking refused
// because an SMTP server was slow is not.

type meetingMail struct {
	MemberName        string
	MemberEmail       string
	Organization      string
	StatedRole        string
	MemberSince       string
	Submissions       string
	When              string
	ZoneLabel         string
	DurationMinutes   int
	Topic             string
	ContactPreference string
	HowLine           string
	SchedulerURL      string
	// NoEvent is the case worth shouting about: the time is claimed in
	// this application but nothing reached the calendar, so the owner
	// will not be reminded and the slot is blocked for everyone else.
	NoEvent bool
}

// meetingConfirmation is what the member receives. Deliberately a
// different shape from the owner's notice: it carries no account
// details, no organisation, no submission count and no admin link,
// because none of that is theirs to see and some of it would be
// unsettling to receive.
type meetingConfirmation struct {
	When            string
	ZoneLabel       string
	DurationMinutes int
	Topic           string
	HowLine         string
	VideoSetupLabel string
	IcsURL          string
	MeetingsURL     string
	ContactURL      string
}

// notifyMemberBooked confirms the booking to the member who made it.
//
// They already get Google's invitation, which is the authoritative
// calendar object. This exists because that invitation says nothing
// about how to cancel, and a member who cannot find the cancel path
// silently keeps a slot nobody else can have.
func (h *Meetings) notifyMemberBooked(
	ctx context.Context,
	member *users.User,
	b scheduling.Booking,
	set scheduling.Settings,
	contact meetingContact,
) {
	if h.mailer == nil || member.Email == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
	defer cancel()

	loc := set.Location()
	base := strings.TrimRight(h.webBaseURL, "/")
	d := meetingConfirmation{
		When:            b.Start.In(loc).Format("Monday, 2 January 2006 at 3:04 PM"),
		ZoneLabel:       zoneLabel(set.Zone),
		DurationMinutes: b.DurMins,
		Topic:           b.Note,
		// Written from the member's side: the owner's copy says "Dana is
		// hosting", theirs should not talk about them in the third
		// person.
		HowLine:     memberHowLine(contact),
		IcsURL:      fmt.Sprintf("%s/api/meetings/%d.ics", base, b.ID),
		MeetingsURL: base + "/meetings",
		ContactURL:  base + "/contact",
	}
	if contact.Type == meetingTypeVideo {
		d.VideoSetupLabel = providerLabel(contact.Provider)
	}

	text, html, err := email.MeetingConfirmedTemplate.Render(d)
	if err != nil {
		h.log.Warn("meetings: could not render the member confirmation", slog.String("error", err.Error()))
		return
	}
	if err := h.mailer.Send(ctx, email.Message{
		From:     h.mailFrom,
		To:       member.Email,
		Subject:  fmt.Sprintf("Your meeting with Roger Henley: %s", b.Start.In(loc).Format("Mon 2 Jan at 3:04 PM")),
		TextBody: text,
		HTMLBody: html,
		Kind:     "meeting_confirmed",
		UserID:   member.ID,
	}); err != nil {
		h.log.Warn("meetings: member confirmation not sent",
			slog.Int64("user", member.ID), slog.String("error", err.Error()))
	}
}

// memberHowLine is the owner's contactLine written the other way round.
func memberHowLine(c meetingContact) string {
	switch c.Type {
	case meetingTypeVideo:
		return fmt.Sprintf("A video call on %s, which you are hosting.", providerLabel(c.Provider))
	case meetingTypePhone:
		if c.Phone == "" {
			return "A phone call. You said your number is in the notes you left."
		}
		return fmt.Sprintf("A phone call. You will be calling from %s.", c.Phone)
	}
	return ""
}

// notifyOwnerBooked sends the owner a booking notice. Best-effort and
// detached from the request: the caller has already committed.
func (h *Meetings) notifyOwnerBooked(
	ctx context.Context,
	member *users.User,
	b scheduling.Booking,
	set scheduling.Settings,
	contact meetingContact,
) {
	if h.mailer == nil || h.ownerEmail == "" {
		return
	}
	// Detached, because the booking is done and this must not be
	// cancelled by the caller's context ending.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
	defer cancel()

	loc := set.Location()
	d := meetingMail{
		MemberName:        nameOr(member.Name, "A member"),
		MemberEmail:       member.Email,
		Organization:      member.Organization,
		StatedRole:        member.StatedRole,
		When:              b.Start.In(loc).Format("Monday, 2 January 2006 at 3:04 PM"),
		ZoneLabel:         zoneLabel(set.Zone),
		DurationMinutes:   b.DurMins,
		Topic:             b.Note,
		ContactPreference: strings.TrimSpace(b.ContactPreference),
		HowLine:           contactLine(contact, member.Name),
		SchedulerURL:      strings.TrimRight(h.webBaseURL, "/") + "/admin/scheduler",
		NoEvent:           b.EventID == "",
	}
	if !member.CreatedAt.IsZero() {
		d.MemberSince = member.CreatedAt.In(loc).Format("2 January 2006")
	}
	// Context the calendar cannot carry: whether this person has
	// actually used the reviewer, which is the difference between a
	// curious visitor and someone who has read the output.
	if subs, err := h.users.ListJdSubmissionsByUser(ctx, member.ID, 50); err == nil && len(subs) > 0 {
		d.Submissions = fmt.Sprintf("%d", len(subs))
	}

	subject := fmt.Sprintf("%s booked %d minutes, %s",
		d.MemberName, d.DurationMinutes, b.Start.In(loc).Format("Mon 2 Jan at 3:04 PM"))
	if d.NoEvent {
		// The subject line is the only part guaranteed to be read on a
		// phone, so the broken case says so there.
		subject = "Booking held but NOT on your calendar: " + subject
	}

	text, html, err := email.MeetingBookedTemplate.Render(d)
	if err != nil {
		h.log.Warn("meetings: could not render the booking notice", slog.String("error", err.Error()))
		return
	}
	if err := h.mailer.Send(ctx, email.Message{
		From:     h.mailFrom,
		To:       h.ownerEmail,
		Subject:  subject,
		TextBody: text,
		HTMLBody: html,
		Kind:     "meeting_booked",
		UserID:   member.ID,
	}); err != nil {
		// Logged, never returned. The meeting exists and is on the
		// scheduler page either way.
		h.log.Warn("meetings: booking notice not sent",
			slog.Int64("user", member.ID), slog.String("error", err.Error()))
	}
}
