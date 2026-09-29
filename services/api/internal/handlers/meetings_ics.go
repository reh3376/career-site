package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/reh3376/career-site/services/api/internal/users"
)

// icsTimeLayout is iCalendar's UTC form. Every timestamp written here
// is in UTC with the trailing Z, which sidesteps VTIMEZONE entirely:
// an instant needs no zone definition to be unambiguous, and the
// receiving calendar renders it in whatever zone its owner uses.
const icsTimeLayout = "20060102T150405Z"

// ServeMeetingICS streams one meeting as a calendar file.
//
// This is the "reserve a slot and send an invitation another way" path
// the owner asked for, and it is also just good manners for a booking
// made in the app: the member gets the meeting into their own calendar
// without this application needing any access to it.
//
// Plain HTTP rather than an RPC because it is a file a browser
// downloads. Gated by a signed-in session that owns the booking, since
// the file carries the meeting's subject.
func (h *Meetings) ServeMeetingICS(w http.ResponseWriter, r *http.Request) {
	member, err := h.auth.LookupSessionUserHTTP(r.Context(), r)
	if err != nil || member == nil || member.Status != users.StatusActive {
		http.Error(w, "sign in to download", http.StatusUnauthorized)
		return
	}
	// The route is /api/meetings/{file} where file is "<id>.ics", so the
	// link reads as a filename and a browser saves it sensibly.
	id, err := strconv.ParseInt(strings.TrimSuffix(r.PathValue("file"), ".ics"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}

	rows, err := h.users.BookingsForUser(r.Context(), member.ID)
	if err != nil {
		http.Error(w, "could not read your meetings", http.StatusInternalServerError)
		return
	}
	var found *users.Booking
	for i := range rows {
		if rows[i].ID == id {
			found = &rows[i]
			break
		}
	}
	// Not found rather than forbidden: whether a booking exists is not
	// something a stranger should be able to probe by changing a number.
	if found == nil {
		http.NotFound(w, r)
		return
	}

	set := h.settings.Get(r.Context())
	body := buildICS(*found, set.Zone, h.icsOrganizer())

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="meeting-%d.ics"`, id))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write([]byte(body))
}

// icsOrganizer is the address the invitation comes from.
func (h *Meetings) icsOrganizer() string {
	if h.organizer != "" {
		return h.organizer
	}
	return "roger@rogerhenley.dev"
}

// buildICS renders one booking as a VEVENT.
//
// A cancelled booking is still served, with STATUS:CANCELLED and the
// sequence bumped, because a member who already imported the file needs
// the cancellation to reach their calendar. Serving a 404 instead would
// leave the meeting sitting in their calendar forever.
func buildICS(b users.Booking, zone, organizer string) string {
	start := b.StartsAt.UTC()
	end := start.Add(time.Duration(b.DurationMin) * time.Minute)

	status, seq := "CONFIRMED", 0
	if b.CancelledAt != nil {
		status, seq = "CANCELLED", 1
	}

	summary := "Meeting with Roger Henley"
	desc := b.Note
	if desc == "" {
		desc = "Booked at rogerhenley.dev"
	}
	desc += fmt.Sprintf("\nTimes were chosen in %s.", zone)

	var sb strings.Builder
	write := func(line string) { sb.WriteString(foldICS(line)); sb.WriteString("\r\n") }

	write("BEGIN:VCALENDAR")
	write("VERSION:2.0")
	write("PRODID:-//rogerhenley.dev//meetings//EN")
	write("CALSCALE:GREGORIAN")
	write("METHOD:PUBLISH")
	write("BEGIN:VEVENT")
	// Stable across re-downloads, so importing twice updates the same
	// entry rather than creating a duplicate.
	write(fmt.Sprintf("UID:meeting-%d@rogerhenley.dev", b.ID))
	write("DTSTAMP:" + time.Now().UTC().Format(icsTimeLayout))
	write("DTSTART:" + start.Format(icsTimeLayout))
	write("DTEND:" + end.Format(icsTimeLayout))
	write("SUMMARY:" + escapeICS(summary))
	write("DESCRIPTION:" + escapeICS(desc))
	write("ORGANIZER;CN=Roger Henley:mailto:" + organizer)
	if b.Email != "" {
		write("ATTENDEE;CN=" + escapeICS(b.Name) + ";RSVP=FALSE:mailto:" + b.Email)
	}
	write("STATUS:" + status)
	write(fmt.Sprintf("SEQUENCE:%d", seq))
	write("END:VEVENT")
	write("END:VCALENDAR")
	return sb.String()
}

// escapeICS applies RFC 5545 text escaping. Without it a comma or a
// newline in the member's own words would end the property early and
// produce a file some calendars refuse to import.
func escapeICS(s string) string {
	r := strings.NewReplacer(
		"\\", "\\\\",
		";", "\\;",
		",", "\\,",
		"\r\n", "\\n",
		"\n", "\\n",
		"\r", "\\n",
	)
	return r.Replace(s)
}

// foldICS wraps a content line at 75 octets, continuing with a leading
// space, as RFC 5545 requires. A long topic would otherwise produce a
// line some parsers reject.
//
// Counted in bytes rather than runes, which is what the spec says, but
// the break is placed on a rune boundary so a multi-byte character is
// never split across the fold.
func foldICS(line string) string {
	const limit = 75
	if len(line) <= limit {
		return line
	}
	var sb strings.Builder
	count := 0
	for i, r := range line {
		n := len(string(r))
		// The continuation line begins with a space, which itself counts
		// toward the next line's octets.
		if count+n > limit {
			sb.WriteString("\r\n ")
			count = 1
		}
		sb.WriteString(line[i : i+n])
		count += n
	}
	return sb.String()
}
