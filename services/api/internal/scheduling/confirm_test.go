package scheduling

import (
	"errors"
	"testing"
	"time"
)

func TestAFreeSlotConfirms(t *testing.T) {
	s := ownerSettings()
	start := mustTime(t, "2026-10-06 09:00")
	if err := StillFree(s, start, 30, nil, nil); err != nil {
		t.Errorf("nothing conflicts; the booking should confirm: %v", err)
	}
}

// The race this exists for: the member was shown the slot, and the
// owner's real calendar moved while they were deciding.
func TestASlotTakenWhileBookingIsRefused(t *testing.T) {
	s := ownerSettings()
	start := mustTime(t, "2026-10-06 09:00")
	fresh := []Interval{{
		Start: mustTime(t, "2026-10-06 09:15"),
		End:   mustTime(t, "2026-10-06 09:45"),
	}}
	err := StillFree(s, start, 30, fresh, nil)
	if err == nil {
		t.Fatal("the calendar now covers this time; the booking must be refused")
	}
	var taken ErrSlotTaken
	if !errors.As(err, &taken) {
		t.Errorf("want ErrSlotTaken so the surface can offer fresh times, got %T", err)
	}
}

// Clearance applies at confirmation exactly as it does when listing,
// or a booking could be accepted that the list would never have shown.
func TestClearanceIsEnforcedAtConfirmationToo(t *testing.T) {
	s := ownerSettings()
	start := mustTime(t, "2026-10-06 10:00")
	fresh := []Interval{{
		Start: mustTime(t, "2026-10-06 09:00"),
		End:   mustTime(t, "2026-10-06 10:00"),
	}}
	if err := StillFree(s, start, 30, fresh, nil); err == nil {
		t.Error("10:00 is a moment after a meeting ending at 10:00; 15 minutes of clearance is required here as well")
	}
}

// A member holding a link from before the owner narrowed his hours must
// not be able to book time he has since withdrawn.
func TestAClaimOutsideTheWindowsIsRefused(t *testing.T) {
	s := ownerSettings()
	for _, tc := range []struct {
		name  string
		start string
		mins  int
	}{
		{"a free evening", "2026-10-06 18:00", 30},
		{"the lunch gap", "2026-10-06 12:30", 30},
		{"a day with no window", "2026-10-05 10:00", 30},
		{"running past the window end", "2026-10-06 11:45", 45},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := StillFree(s, mustTime(t, tc.start), tc.mins, nil, nil); err == nil {
				t.Error("outside the configured hours; must be refused whatever the calendar says")
			}
		})
	}
}

// A meeting finishing exactly on the window boundary is inside it.
func TestAMeetingEndingExactlyAtTheBoundaryIsAllowed(t *testing.T) {
	s := ownerSettings()
	if err := StillFree(s, mustTime(t, "2026-10-06 11:30"), 30, nil, nil); err != nil {
		t.Errorf("11:30 to 12:00 finishes exactly at the window end and should be allowed: %v", err)
	}
}

func TestALengthNobodyOffersCannotBeConfirmed(t *testing.T) {
	s := ownerSettings()
	if err := StillFree(s, mustTime(t, "2026-10-06 09:00"), 20, nil, nil); err == nil {
		t.Error("20 minutes is not offered and must not be confirmable by posting it directly")
	}
}

// The message a member sees has to name the time in the owner's zone,
// because a bare time is how somebody argues they booked a different
// hour.
func TestTheRefusalNamesTheTimeAndZone(t *testing.T) {
	e := ErrSlotTaken{Start: mustTime(t, "2026-10-06 09:00"), Zone: "America/New_York"}
	msg := e.Error()
	for _, want := range []string{"09:00", "EDT", "Tue"} {
		if !contains(msg, want) {
			t.Errorf("message %q should contain %q", msg, want)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

var _ = time.Now
