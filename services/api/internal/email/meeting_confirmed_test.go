package email

import (
	"strings"
	"testing"
)

// The member's confirmation is deliberately a different shape from the
// owner's notice. These check the difference holds, because the easy
// mistake is to reuse one template and leak the other side's view.
func confirmedData() map[string]any {
	return map[string]any{
		"When":            "Thursday, 1 October 2026 at 9:00 AM",
		"ZoneLabel":       "Eastern time",
		"DurationMinutes": 30,
		"Topic":           "The automation services director role",
		"HowLine":         "A video call on Google Meet, which you are hosting.",
		"VideoSetupLabel": "Google Meet",
		"IcsURL":          "https://rogerhenley.dev/api/meetings/42.ics",
		"MeetingsURL":     "https://rogerhenley.dev/meetings",
		"ContactURL":      "https://rogerhenley.dev/contact",
	}
}

func TestConfirmationCarriesTheDetailsAndACancelPath(t *testing.T) {
	text, html, err := MeetingConfirmedTemplate.Render(confirmedData())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{
		"Thursday, 1 October 2026 at 9:00 AM",
		"30", "Eastern time",
		"Google Meet",
		"The automation services director role",
		"/api/meetings/42.ics",
		"/meetings", // the cancel path, which Google's invitation has no idea about
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text is missing %q", want)
		}
		if !strings.Contains(html, want) {
			t.Errorf("html is missing %q", want)
		}
	}
}

// The member must never see the owner's view of them.
func TestConfirmationLeaksNothingFromTheOwnersNotice(t *testing.T) {
	text, html, err := MeetingConfirmedTemplate.Render(confirmedData())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, leak := range []string{"/admin", "Member since", "JD reviews run", "Organization"} {
		if strings.Contains(text, leak) {
			t.Errorf("text leaks %q", leak)
		}
		if strings.Contains(html, leak) {
			t.Errorf("html leaks %q", leak)
		}
	}
}

// A phone booking should not be told to create a video room.
func TestConfirmationOmitsVideoSetupForAPhoneCall(t *testing.T) {
	d := confirmedData()
	d["VideoSetupLabel"] = ""
	d["HowLine"] = "A phone call. You will be calling from +1 5135551234."
	text, _, err := MeetingConfirmedTemplate.Render(d)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(text, "Create the room") {
		t.Error("tells a phone booker to create a video room")
	}
	if !strings.Contains(text, "+1 5135551234") {
		t.Error("lost the number they gave")
	}
}
