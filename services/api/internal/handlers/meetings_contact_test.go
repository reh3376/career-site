package handlers

import (
	"strings"
	"testing"

	careerv1 "github.com/reh3376/career-site/services/api/gen/career/v1"
)

func req(t careerv1.MeetingType, p careerv1.VideoProvider, phone string) *careerv1.BookMeetingRequest {
	return &careerv1.BookMeetingRequest{MeetingType: t, VideoProvider: p, PhoneNumber: phone}
}

func TestVideoNeedsAProvider(t *testing.T) {
	_, err := contactFromRequest(req(
		careerv1.MeetingType_MEETING_TYPE_VIDEO,
		careerv1.VideoProvider_VIDEO_PROVIDER_UNSPECIFIED, ""))
	if err == nil {
		t.Error("a video meeting with no service was accepted")
	}
}

func TestEachProviderIsStored(t *testing.T) {
	for p, want := range map[careerv1.VideoProvider]string{
		careerv1.VideoProvider_VIDEO_PROVIDER_GOOGLE_MEET: "google_meet",
		careerv1.VideoProvider_VIDEO_PROVIDER_TEAMS:       "teams",
		careerv1.VideoProvider_VIDEO_PROVIDER_ZOOM:        "zoom",
	} {
		got, err := contactFromRequest(req(careerv1.MeetingType_MEETING_TYPE_VIDEO, p, ""))
		if err != nil {
			t.Errorf("%v: %v", p, err)
			continue
		}
		if got.Provider != want || got.Type != meetingTypeVideo {
			t.Errorf("%v gave %+v", p, got)
		}
	}
}

// A provider on a phone call would leave the owner's calendar entry
// saying two contradictory things.
func TestPhoneRejectsAVideoProvider(t *testing.T) {
	_, err := contactFromRequest(req(
		careerv1.MeetingType_MEETING_TYPE_PHONE,
		careerv1.VideoProvider_VIDEO_PROVIDER_ZOOM, "+1 5135551234"))
	if err == nil {
		t.Error("a phone meeting with a video service was accepted")
	}
}

// North American numbers are always ten digits after +1, so the rule
// there is strict and catches a transposed or dropped digit.
func TestNorthAmericanNumbersAreStrict(t *testing.T) {
	for _, ok := range []string{"+1 5135551234", "1 5135551234"} {
		if _, err := contactFromRequest(req(careerv1.MeetingType_MEETING_TYPE_PHONE,
			careerv1.VideoProvider_VIDEO_PROVIDER_UNSPECIFIED, ok)); err != nil {
			t.Errorf("%q was refused: %v", ok, err)
		}
	}
	for _, bad := range []string{
		"5135551234",      // no country code
		"+1-513-555-1234", // dashes, not a space
		"+1 513555123",    // nine digits
		"+1 51355512345",  // eleven digits
		"+1  5135551234",  // two spaces
		"call me",         // words
	} {
		if _, err := contactFromRequest(req(careerv1.MeetingType_MEETING_TYPE_PHONE,
			careerv1.VideoProvider_VIDEO_PROVIDER_UNSPECIFIED, bad)); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

// Everywhere else there is no length rule (owner, 2026-09-30): the
// only requirement is digits. Any length this code invented would turn
// away real numbers somewhere.
func TestOtherCountriesHaveNoLengthRule(t *testing.T) {
	for _, ok := range []string{
		"+44 2079460958",     // London, ten
		"+353 15551234",      // Dublin, eight
		"+61 291234567",      // Sydney, nine
		"+86 13812345678",    // China mobile, eleven
		"+44 123",            // short, and not ours to refuse
		"+49 30123456789012", // long, likewise
	} {
		if _, err := contactFromRequest(req(careerv1.MeetingType_MEETING_TYPE_PHONE,
			careerv1.VideoProvider_VIDEO_PROVIDER_UNSPECIFIED, ok)); err != nil {
			t.Errorf("%q was refused: %v", ok, err)
		}
	}
	// Digits only: nothing to interpret, and nothing that could arrive
	// looking like a number and not be one.
	for _, bad := range []string{
		"+44 20 7946 0958",  // spaces inside the number
		"+44 207-946-0958",  // dashes
		"+44 (20) 79460958", // parentheses
		"+44 call me",       // words
		"+44 ",              // a code and nothing else
		"+44 0790x123456",   // a letter hiding in it
	} {
		if _, err := contactFromRequest(req(careerv1.MeetingType_MEETING_TYPE_PHONE,
			careerv1.VideoProvider_VIDEO_PROVIDER_UNSPECIFIED, bad)); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

// The looser rule must not become a way round the strict one: a +1
// number of the wrong length has to stay refused.
func TestLooseRuleDoesNotRescueABadPlusOne(t *testing.T) {
	for _, bad := range []string{"+1 513555", "1 51355512345"} {
		if _, err := contactFromRequest(req(careerv1.MeetingType_MEETING_TYPE_PHONE,
			careerv1.VideoProvider_VIDEO_PROVIDER_UNSPECIFIED, bad)); err == nil {
			t.Errorf("%q was accepted by falling through to the loose rule", bad)
		}
	}
}

// The escape hatch. A number that does not fit the shape is not
// something a member should have to argue with, so leaving it blank
// says "it is in the comments" and the booking proceeds.
func TestBlankPhoneIsAllowedAndMeansTheComments(t *testing.T) {
	got, err := contactFromRequest(req(careerv1.MeetingType_MEETING_TYPE_PHONE,
		careerv1.VideoProvider_VIDEO_PROVIDER_UNSPECIFIED, "   "))
	if err != nil {
		t.Fatalf("blank was refused: %v", err)
	}
	if got.Phone != "" || got.Type != meetingTypePhone {
		t.Errorf("got %+v", got)
	}
	if line := contactLine(got, "Dana Reed"); !strings.Contains(line, "notes below") {
		t.Errorf("the calendar line does not say where the number is: %q", line)
	}
}

func TestATypeIsRequired(t *testing.T) {
	_, err := contactFromRequest(req(
		careerv1.MeetingType_MEETING_TYPE_UNSPECIFIED,
		careerv1.VideoProvider_VIDEO_PROVIDER_UNSPECIFIED, ""))
	if err == nil {
		t.Error("a booking with no meeting type was accepted")
	}
}

// The calendar line is the whole point of asking: it is what the owner
// reads when the reminder fires.
func TestCalendarLineSaysHowToReachThem(t *testing.T) {
	video, _ := contactFromRequest(req(careerv1.MeetingType_MEETING_TYPE_VIDEO,
		careerv1.VideoProvider_VIDEO_PROVIDER_TEAMS, ""))
	if line := contactLine(video, "Dana Reed"); !strings.Contains(line, "Microsoft Teams") ||
		!strings.Contains(line, "Dana Reed") {
		t.Errorf("video line is %q", line)
	}

	phone, _ := contactFromRequest(req(careerv1.MeetingType_MEETING_TYPE_PHONE,
		careerv1.VideoProvider_VIDEO_PROVIDER_UNSPECIFIED, "+1 5135551234"))
	if line := contactLine(phone, "Dana Reed"); !strings.Contains(line, "+1 5135551234") {
		t.Errorf("phone line does not carry the number: %q", line)
	}
}

// A booking made before this was asked has no type, and must not
// produce a stray line on the calendar entry.
func TestNoTypeProducesNoLine(t *testing.T) {
	if line := contactLine(meetingContact{}, "Dana Reed"); line != "" {
		t.Errorf("got %q, want empty", line)
	}
}
