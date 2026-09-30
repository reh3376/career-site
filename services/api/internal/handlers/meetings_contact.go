package handlers

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	careerv1 "github.com/reh3376/career-site/services/api/gen/career/v1"
)

// How a meeting happens, validated here so a booking cannot be stored
// half-described.
//
// The owner's decision is that the member brings their own video room:
// they name the provider and set it up in their own calendar. This
// application holds no Meet, Teams or Zoom credentials and creates no
// rooms, which is three fewer OAuth grants and three fewer things to
// break. The provider is recorded so the invitation says the right
// thing and the owner knows what to expect.

const (
	meetingTypeVideo = "video"
	meetingTypePhone = "phone"
)

// Phone numbers, validated by how much can honestly be asserted.
//
// North American numbers are always ten digits after the +1, so the
// rule there is exact. That is the whole reason to have one: it catches
// a transposed or dropped digit, and a mistyped number that reaches the
// owner looking correct is worse than no number at all.
//
// Everywhere else there is no length rule at all (owner, 2026-09-30).
// National lengths genuinely vary, from seven digits to twelve and
// beyond, and any number this code invented would turn away real
// numbers. The only requirement is that it is digits: no spaces inside
// it, no dashes, no parentheses, nothing to interpret. The proto caps
// the field at 32 characters, which is a field limit rather than a
// claim about telephony.
//
// Both shapes are escapable. A number that fits neither is not an error
// a member should have to argue with, so leaving it blank says the
// number is in the comments and the booking proceeds.
var (
	phoneNANP  = regexp.MustCompile(`^\+?1 [0-9]{10}$`)
	phoneOther = regexp.MustCompile(`^\+?[0-9]{1,3} [0-9]+$`)
)

var errPhoneShape = errors.New(
	"a North American number is ten digits after +1; elsewhere give digits only, with no spaces or punctuation, or leave it blank and say it in the comments")

// validPhone reports whether a submitted number is one we will pass on
// to the owner.
func validPhone(s string) bool {
	if phoneNANP.MatchString(s) {
		return true
	}
	// The NANP rule owns +1 entirely, so a +1 number that failed it must
	// not fall through to the digits-only check and be accepted at the
	// wrong length.
	if strings.HasPrefix(s, "+1 ") || strings.HasPrefix(s, "1 ") {
		return false
	}
	return phoneOther.MatchString(s)
}

type meetingContact struct {
	Type     string
	Provider string
	Phone    string
}

// contactFromRequest validates the how-we-meet half of a booking.
func contactFromRequest(req *careerv1.BookMeetingRequest) (meetingContact, error) {
	var out meetingContact
	switch req.GetMeetingType() {
	case careerv1.MeetingType_MEETING_TYPE_VIDEO:
		out.Type = meetingTypeVideo
		switch req.GetVideoProvider() {
		case careerv1.VideoProvider_VIDEO_PROVIDER_GOOGLE_MEET:
			out.Provider = "google_meet"
		case careerv1.VideoProvider_VIDEO_PROVIDER_TEAMS:
			out.Provider = "teams"
		case careerv1.VideoProvider_VIDEO_PROVIDER_ZOOM:
			out.Provider = "zoom"
		default:
			return out, errors.New("choose a video service")
		}

	case careerv1.MeetingType_MEETING_TYPE_PHONE:
		out.Type = meetingTypePhone
		// A provider on a phone call is a contradiction, and storing it
		// would leave the owner's calendar entry saying two things.
		if req.GetVideoProvider() != careerv1.VideoProvider_VIDEO_PROVIDER_UNSPECIFIED {
			return out, errors.New("a phone meeting has no video service")
		}
		phone := strings.TrimSpace(req.GetPhoneNumber())
		// Empty is the documented escape hatch, not an oversight.
		if phone != "" && !validPhone(phone) {
			return out, errPhoneShape
		}
		out.Phone = phone

	default:
		return out, errors.New("choose whether this is a video call or a phone call")
	}
	return out, nil
}

// providerLabel is the human name, for the calendar entry the owner
// reads rather than for storage.
func providerLabel(p string) string {
	switch p {
	case "google_meet":
		return "Google Meet"
	case "teams":
		return "Microsoft Teams"
	case "zoom":
		return "Zoom"
	}
	return p
}

// contactLine is what the owner sees on his calendar entry. It is the
// whole point of asking: a time with no way to reach the other person
// is a second exchange of emails to arrange the thing the booking was
// supposed to have arranged.
func contactLine(c meetingContact, name string) string {
	switch c.Type {
	case meetingTypeVideo:
		return fmt.Sprintf("%s is hosting on %s and will send the link.",
			nameOr(name, "The member"), providerLabel(c.Provider))
	case meetingTypePhone:
		if c.Phone == "" {
			return fmt.Sprintf("Phone call. %s said the number is in the notes below.",
				nameOr(name, "The member"))
		}
		return fmt.Sprintf("Phone call. %s will call from %s.", nameOr(name, "The member"), c.Phone)
	}
	return ""
}

func nameOr(name, fallback string) string {
	if strings.TrimSpace(name) == "" {
		return fallback
	}
	return name
}

// typeToProto and providerToProto render stored values back to the
// contract, so a member sees what they chose.
func typeToProto(s string) careerv1.MeetingType {
	switch s {
	case meetingTypeVideo:
		return careerv1.MeetingType_MEETING_TYPE_VIDEO
	case meetingTypePhone:
		return careerv1.MeetingType_MEETING_TYPE_PHONE
	}
	return careerv1.MeetingType_MEETING_TYPE_UNSPECIFIED
}

func providerToProto(s string) careerv1.VideoProvider {
	switch s {
	case "google_meet":
		return careerv1.VideoProvider_VIDEO_PROVIDER_GOOGLE_MEET
	case "teams":
		return careerv1.VideoProvider_VIDEO_PROVIDER_TEAMS
	case "zoom":
		return careerv1.VideoProvider_VIDEO_PROVIDER_ZOOM
	}
	return careerv1.VideoProvider_VIDEO_PROVIDER_UNSPECIFIED
}
