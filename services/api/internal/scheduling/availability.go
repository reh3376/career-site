// Package scheduling computes which meeting slots a member may book.
//
// The owner's availability is a list of windows per weekday, not a
// start and an end. His stated hours are Tuesday, Wednesday and
// Thursday, 09:00 to 12:00 and 14:00 to 16:00, and building that as one
// range with a break bolted on later is the usual way this ends up
// wrong.
//
// Everything here works in the owner's zone, America/New_York, and that
// is deliberate rather than incidental. The zone is stored as an IANA
// name and never as an offset: America/New_York is UTC-5 in winter and
// UTC-4 in summer, so a 09:00 window expressed as an offset would drift
// by an hour twice a year and nobody would notice until a meeting was
// missed.
//
// The windows are the outer bound. The calendar subtracts from them and
// never adds: a free Tuesday evening is still not bookable, because
// availability is a decision the owner made and not a gap Google
// happens to report.
package scheduling

import (
	"fmt"
	"sort"
	"time"
)

// Window is one bookable range on one weekday, in the owner's zone.
// Start and End are minutes from midnight, so they survive a DST change
// that moves the wall clock underneath them.
type Window struct {
	Weekday   time.Weekday
	StartMins int
	EndMins   int
}

// Interval is a half-open span [Start, End) in absolute time. Busy
// intervals come from the calendar; booked ones from this application.
type Interval struct {
	Start time.Time
	End   time.Time
}

// Overlaps reports whether two half-open intervals intersect. Touching
// at an endpoint is not an overlap: a meeting ending at 10:00 does not
// collide with one starting at 10:00.
func (i Interval) Overlaps(o Interval) bool {
	return i.Start.Before(o.End) && o.Start.Before(i.End)
}

// Settings is the owner's configuration. Everything here is editable in
// the admin surface; none of it is compiled in.
type Settings struct {
	// Zone is an IANA name, never an offset.
	Zone string
	// SlotMins is the length of a bookable meeting.
	SlotMins int
	// BufferMins is dead time kept after each booking, so three
	// meetings in a morning are not back to back.
	BufferMins int
	// MaxPerDay caps bookings per calendar day in the owner's zone, so
	// three available days do not become five conversations on a
	// Wednesday.
	MaxPerDay int
	// LeadHours is how far ahead of now the first bookable slot sits.
	// Nobody should be able to book a meeting starting in ten minutes.
	LeadHours int
	// HorizonDays is how far ahead the calendar is offered.
	HorizonDays int
	// Windows are the outer bound of availability.
	Windows []Window
}

// Slot is one offered start time.
type Slot struct {
	Start time.Time
	End   time.Time
}

// Validate reports a configuration that cannot produce sensible slots,
// so the admin surface refuses it rather than rendering an empty
// calendar that looks like no availability.
func (s Settings) Validate() error {
	if _, err := time.LoadLocation(s.Zone); err != nil {
		return fmt.Errorf("zone %q is not an IANA location: %w", s.Zone, err)
	}
	if s.SlotMins <= 0 {
		return fmt.Errorf("slot length must be positive, got %d", s.SlotMins)
	}
	if s.BufferMins < 0 {
		return fmt.Errorf("buffer cannot be negative, got %d", s.BufferMins)
	}
	if s.MaxPerDay <= 0 {
		return fmt.Errorf("per-day cap must be positive, got %d", s.MaxPerDay)
	}
	if s.HorizonDays <= 0 {
		return fmt.Errorf("horizon must be positive, got %d days", s.HorizonDays)
	}
	for _, w := range s.Windows {
		if w.StartMins < 0 || w.EndMins > 24*60 || w.StartMins >= w.EndMins {
			return fmt.Errorf("window on %s is not a range: %d to %d",
				w.Weekday, w.StartMins, w.EndMins)
		}
		if w.EndMins-w.StartMins < s.SlotMins {
			return fmt.Errorf("window on %s is shorter than one %d-minute slot",
				w.Weekday, s.SlotMins)
		}
	}
	return nil
}

// Slots returns every start time a member may book, in order.
//
// busy is what the calendar reports, and carries no detail about what
// those intervals are: free/busy only. booked is what this application
// has already taken, which is tracked separately because a held slot is
// not yet on the calendar and would otherwise be offered twice.
func Slots(s Settings, busy, booked []Interval, now time.Time) ([]Slot, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation(s.Zone)
	if err != nil {
		return nil, err
	}

	earliest := now.Add(time.Duration(s.LeadHours) * time.Hour)
	limit := now.AddDate(0, 0, s.HorizonDays)

	// Count what is already booked per day, so the cap is applied to
	// the day a slot falls in rather than to the list as a whole.
	perDay := map[string]int{}
	for _, b := range booked {
		perDay[b.Start.In(loc).Format("2006-01-02")]++
	}

	byWeekday := map[time.Weekday][]Window{}
	for _, w := range s.Windows {
		byWeekday[w.Weekday] = append(byWeekday[w.Weekday], w)
	}

	var out []Slot
	// Walk calendar days in the owner's zone. Adding 24 hours would
	// drift across a DST boundary; AddDate on a midnight-anchored date
	// lands on the next local midnight whatever the offset did.
	day := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 0, 0, 0, 0, loc)
	for ; day.Before(limit); day = day.AddDate(0, 0, 1) {
		windows := byWeekday[day.Weekday()]
		if len(windows) == 0 {
			continue
		}
		key := day.Format("2006-01-02")
		for _, w := range windows {
			// Build each boundary from the local date plus minutes, so a
			// spring-forward day simply has fewer slots rather than
			// silently shifting them.
			start := day.Add(time.Duration(w.StartMins) * time.Minute)
			end := day.Add(time.Duration(w.EndMins) * time.Minute)
			step := time.Duration(s.SlotMins+s.BufferMins) * time.Minute
			for t := start; !t.Add(time.Duration(s.SlotMins) * time.Minute).After(end); t = t.Add(step) {
				if perDay[key] >= s.MaxPerDay {
					break
				}
				slot := Interval{Start: t, End: t.Add(time.Duration(s.SlotMins) * time.Minute)}
				if slot.Start.Before(earliest) {
					continue
				}
				// The buffer applies to conflicts as well as to spacing:
				// a booking ending at 10:00 blocks a 10:00 start when a
				// buffer is configured.
				guard := Interval{
					Start: slot.Start.Add(-time.Duration(s.BufferMins) * time.Minute),
					End:   slot.End.Add(time.Duration(s.BufferMins) * time.Minute),
				}
				if anyOverlap(guard, busy) || anyOverlap(guard, booked) {
					continue
				}
				out = append(out, Slot{Start: slot.Start, End: slot.End})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out, nil
}

func anyOverlap(i Interval, list []Interval) bool {
	for _, o := range list {
		if i.Overlaps(o) {
			return true
		}
	}
	return false
}
