package email

import (
	"strings"
	"testing"
)

// The booking notice is not a duplicate of the calendar invitation: it
// carries what Google has no idea about. These check the parts that
// would be silently wrong.
func bookedData() map[string]any {
	return map[string]any{
		"MemberName":        "Dana Reed",
		"MemberEmail":       "dana@example.com",
		"Organization":      "Cornerstone Controls",
		"StatedRole":        "VP Professional Services",
		"MemberSince":       "12 September 2026",
		"Submissions":       "3",
		"When":              "Thursday, 1 October 2026 at 9:00 AM",
		"ZoneLabel":         "Eastern time",
		"DurationMinutes":   30,
		"Topic":             "The automation services director role",
		"ContactPreference": "Call my mobile, not the desk",
		"HowLine":           "Dana Reed is hosting on Google Meet and will send the link.",
		"SchedulerURL":      "https://rogerhenley.dev/admin/scheduler",
		"NoEvent":           false,
	}
}

func TestBookedNoticeCarriesWhatTheCalendarCannot(t *testing.T) {
	text, html, err := MeetingBookedTemplate.Render(bookedData())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{
		"Dana Reed", "dana@example.com",
		"Cornerstone Controls",     // who they are, not on the invite
		"VP Professional Services", // ditto
		"12 September 2026",        // how long they have had access
		"Google Meet",              // how the call happens
		"The automation services director role",
		"Call my mobile, not the desk", // their own words, unedited
		"/admin/scheduler",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text is missing %q", want)
		}
		if !strings.Contains(html, want) {
			t.Errorf("html is missing %q", want)
		}
	}
}

// The case worth shouting about: the slot is claimed here but nothing
// reached the calendar, so no reminder will fire and the time is
// blocked for everyone else.
func TestBookedNoticeFlagsAMissingCalendarEvent(t *testing.T) {
	d := bookedData()
	d["NoEvent"] = true
	text, html, err := MeetingBookedTemplate.Render(d)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(text, "no event reached your calendar") {
		t.Error("text does not warn that the calendar has no event")
	}
	if !strings.Contains(html, "no event reached your calendar") {
		t.Error("html does not warn that the calendar has no event")
	}
}

func TestBookedNoticeOmitsWhatIsAbsent(t *testing.T) {
	d := bookedData()
	for _, k := range []string{"Organization", "StatedRole", "ContactPreference", "Topic", "Submissions", "MemberSince"} {
		d[k] = ""
	}
	text, _, err := MeetingBookedTemplate.Render(d)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// An empty optional must not leave a labelled blank line behind.
	for _, stray := range []string{"Also:", "Member since:", "JD reviews run:"} {
		if strings.Contains(text, stray) {
			t.Errorf("renders %q with nothing after it", stray)
		}
	}
	if !strings.Contains(text, "Dana Reed booked 30 minutes") {
		t.Error("lost the headline when the optionals went")
	}
}
