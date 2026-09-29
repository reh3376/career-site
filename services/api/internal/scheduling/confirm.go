package scheduling

import (
	"fmt"
	"time"
)

// Confirming a booking, and why the calendar has to be asked twice.
//
// A member sees the slots, thinks about it, fills in a note and
// submits. Minutes pass. The owner's calendar is his real one and does
// not stop moving while somebody decides: a call gets booked, a meeting
// runs long, something is accepted from an invitation.
//
// So the free/busy check that produced the list is stale by the time
// anyone acts on it. Availability is read again for the one interval
// being claimed, immediately before the event is created, and the
// booking is refused if the answer changed. Without that, this
// application writes a double-booking onto the owner's real calendar
// and he finds out when two people arrive.
//
// There are two distinct races and they need different answers:
//
//   - Somebody else took the time in Google. Only Google knows, so it
//     has to be asked, and asked about the specific interval rather
//     than trusting a list fetched earlier.
//   - Another member is booking the same slot through this
//     application. Google does not know about that yet, because the
//     event is not created until the claim succeeds. That one is
//     settled in the database, by taking the row before calling out.
//
// This file covers the first. The second belongs to the repository,
// which must insert the booking under a constraint that makes a second
// claim on the same interval fail rather than succeed quietly.

// ErrSlotTaken is returned when the interval stopped being free between
// the member seeing it and claiming it.
type ErrSlotTaken struct {
	Start time.Time
	Zone  string
}

func (e ErrSlotTaken) Error() string {
	loc, err := time.LoadLocation(e.Zone)
	if err != nil {
		loc = time.UTC
	}
	return fmt.Sprintf("that time was taken while you were booking it (%s)",
		e.Start.In(loc).Format("Mon 2 Jan, 15:04 MST"))
}

// StillFree reports whether a requested meeting can still be booked,
// given what the calendar says right now.
//
// fresh must be free/busy read at confirmation time, not the list the
// member was shown. Passing the earlier list would make this function
// agree with itself and prove nothing.
func StillFree(s Settings, start time.Time, durationMins int, fresh, booked []Interval) error {
	if !s.Allows(durationMins) {
		return fmt.Errorf("%d minutes is not an offered meeting length", durationMins)
	}
	if err := s.Validate(); err != nil {
		return err
	}
	loc, err := time.LoadLocation(s.Zone)
	if err != nil {
		return err
	}

	// The claim has to land inside a configured window, not merely in a
	// gap the calendar happens to leave. A member holding a stale link
	// from before the owner narrowed his hours would otherwise book
	// time he has since withdrawn.
	if !withinWindow(s, start, durationMins, loc) {
		return fmt.Errorf("%s is outside the hours available for booking",
			start.In(loc).Format("Mon 2 Jan, 15:04 MST"))
	}

	gap := time.Duration(s.GapMins) * time.Minute
	guard := Interval{
		Start: start.Add(-gap),
		End:   start.Add(time.Duration(durationMins) * time.Minute).Add(gap),
	}
	if anyOverlap(guard, fresh) || anyOverlap(guard, booked) {
		return ErrSlotTaken{Start: start, Zone: s.Zone}
	}
	return nil
}

// withinWindow reports whether the whole meeting sits inside one of the
// owner's configured ranges for that weekday. The clearance may run
// past the end; the conversation may not.
func withinWindow(s Settings, start time.Time, durationMins int, loc *time.Location) bool {
	local := start.In(loc)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	startMins := int(local.Sub(midnight).Minutes())
	endMins := startMins + durationMins
	for _, w := range s.Windows {
		if w.Weekday != local.Weekday() {
			continue
		}
		if startMins >= w.StartMins && endMins <= w.EndMins {
			return true
		}
	}
	return false
}
