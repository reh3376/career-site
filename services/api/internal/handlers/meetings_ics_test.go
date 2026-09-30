package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/reh3376/career-site/services/api/internal/users"
)

func testBooking() users.Booking {
	return users.Booking{
		ID:          42,
		Name:        "Dana Reed",
		Email:       "dana@example.com",
		Note:        "Discuss the plant historian work",
		StartsAt:    time.Date(2026, 10, 6, 13, 0, 0, 0, time.UTC),
		DurationMin: 30,
	}
}

func lines(s string) []string { return strings.Split(strings.TrimRight(s, "\r\n"), "\r\n") }

func find(t *testing.T, ics, prefix string) string {
	t.Helper()
	for _, l := range lines(ics) {
		if strings.HasPrefix(l, prefix) {
			return l
		}
	}
	t.Fatalf("no line starting %q in:\n%s", prefix, ics)
	return ""
}

// Every line must end CRLF. A bare newline is the classic way an
// otherwise correct file gets rejected on import.
func TestICSLinesEndWithCRLF(t *testing.T) {
	ics := buildICS(testBooking(), "America/New_York", "roger@example.com")
	if strings.Contains(strings.ReplaceAll(ics, "\r\n", ""), "\n") {
		t.Error("found a bare newline; every content line must end CRLF")
	}
	if !strings.HasPrefix(ics, "BEGIN:VCALENDAR\r\n") {
		t.Error("does not open with BEGIN:VCALENDAR")
	}
	if !strings.HasSuffix(ics, "END:VCALENDAR\r\n") {
		t.Error("does not close with END:VCALENDAR")
	}
}

// Times go out as UTC instants with the trailing Z, which is what lets
// the file carry no VTIMEZONE block and still be unambiguous.
func TestICSTimesAreUTCInstants(t *testing.T) {
	ics := buildICS(testBooking(), "America/New_York", "roger@example.com")
	if got := find(t, ics, "DTSTART:"); got != "DTSTART:20261006T130000Z" {
		t.Errorf("DTSTART is %q", got)
	}
	// 30 minutes later, and the end is the meeting's end, not the end
	// of its clearance: the gap is held in the database, not published
	// to the member's own calendar.
	if got := find(t, ics, "DTEND:"); got != "DTEND:20261006T133000Z" {
		t.Errorf("DTEND is %q", got)
	}
}

// A comma or a semicolon in the member's own words would otherwise end
// the property early and produce a file some calendars refuse.
func TestICSEscapesTheMembersText(t *testing.T) {
	b := testBooking()
	b.Note = "Roles; comp, and the reviewer\nplus a backslash \\ here"
	ics := buildICS(b, "America/New_York", "roger@example.com")

	desc := find(t, ics, "DESCRIPTION:")
	for _, bad := range []string{"Roles;", "comp,"} {
		if strings.Contains(desc, bad) {
			t.Errorf("unescaped %q in %q", bad, desc)
		}
	}
	for _, want := range []string{`Roles\;`, `comp\,`, `\n`, `\\`} {
		if !strings.Contains(desc, want) {
			t.Errorf("missing escape %q in %q", want, desc)
		}
	}
	// The escaped newline must not have become a real line break.
	if len(lines(ics)) != len(lines(buildICS(testBooking(), "America/New_York", "roger@example.com"))) {
		t.Error("an escaped newline split the DESCRIPTION across lines")
	}
}

// RFC 5545 caps a content line at 75 octets; a long topic has to fold
// onto continuation lines beginning with a space.
func TestICSFoldsLongLines(t *testing.T) {
	b := testBooking()
	b.Note = strings.Repeat("a plant historian migration ", 12)
	ics := buildICS(b, "America/New_York", "roger@example.com")

	for _, l := range lines(ics) {
		if len(l) > 75 {
			t.Errorf("line over 75 octets (%d): %q", len(l), l)
		}
	}
	folded := 0
	for _, l := range lines(ics) {
		if strings.HasPrefix(l, " ") {
			folded++
		}
	}
	if folded == 0 {
		t.Error("nothing folded, so the long description was not wrapped at all")
	}
}

// Folding counts octets but must not split a multi-byte character, or
// the file contains a broken rune.
func TestICSFoldingKeepsRunesWhole(t *testing.T) {
	b := testBooking()
	b.Note = strings.Repeat("café résumé ", 12)
	ics := buildICS(b, "America/New_York", "roger@example.com")

	unfolded := strings.ReplaceAll(ics, "\r\n ", "")
	if strings.Contains(unfolded, "�") {
		t.Error("folding split a multi-byte character")
	}
	if !strings.Contains(unfolded, "café résumé") {
		t.Error("unfolding did not restore the original text")
	}
}

// A cancelled booking is still served, because a member who already
// imported the file needs the cancellation to reach their calendar.
// A 404 would leave the meeting in their calendar forever.
func TestICSCarriesCancellation(t *testing.T) {
	b := testBooking()
	at := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	b.CancelledAt = &at

	ics := buildICS(b, "America/New_York", "roger@example.com")
	if got := find(t, ics, "STATUS:"); got != "STATUS:CANCELLED" {
		t.Errorf("status is %q", got)
	}
	// The sequence has to advance or the calendar keeps the old copy.
	if got := find(t, ics, "SEQUENCE:"); got == "SEQUENCE:0" {
		t.Error("sequence did not advance, so the cancellation would be ignored")
	}
}

// Importing the same file twice must update one entry, not make two.
func TestICSUIDIsStable(t *testing.T) {
	a := buildICS(testBooking(), "America/New_York", "roger@example.com")
	b := buildICS(testBooking(), "America/New_York", "roger@example.com")
	if find(t, a, "UID:") != find(t, b, "UID:") {
		t.Error("UID changed between renders of the same booking")
	}
	if !strings.Contains(find(t, a, "UID:"), "42") {
		t.Error("UID does not identify the booking")
	}
}

// An empty topic still has to produce a usable file.
func TestICSWithoutATopicStillRenders(t *testing.T) {
	b := testBooking()
	b.Note = ""
	ics := buildICS(b, "America/New_York", "roger@example.com")
	if d := find(t, ics, "DESCRIPTION:"); strings.TrimSpace(strings.TrimPrefix(d, "DESCRIPTION:")) == "" {
		t.Error("empty description; the file should still say where it came from")
	}
}
