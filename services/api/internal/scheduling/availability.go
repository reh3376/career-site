// Package scheduling computes which meeting slots a member may book.
//
// The owner's availability is a list of windows per weekday, not a
// start and an end. His stated hours are Tuesday, Wednesday and
// Thursday, 09:00 to 12:00 and 14:00 to 16:00, and building that as one
// range with a lunch break bolted on later is the usual way this ends
// up wrong.
//
// Everything works in the owner's zone, America/New_York, stored as an
// IANA name and never as an offset. America/New_York is UTC-5 in winter
// and UTC-4 in summer, so a 09:00 window held as an offset would drift
// by an hour twice a year and nobody would notice until a meeting was
// missed.
//
// The windows are the outer bound. The calendar subtracts from them and
// never adds: a free Tuesday evening is still not bookable, because
// availability is a decision the owner made rather than a gap Google
// happens to report.
package scheduling

import (
	"fmt"
	"sort"
	"time"
)

// Window is one bookable range on one weekday. Start and End are
// minutes from local midnight, so they survive a DST change that moves
// the wall clock underneath them.
type Window struct {
	Weekday   time.Weekday
	StartMins int
	EndMins   int
}

// Interval is a half-open span [Start, End). Busy intervals come from
// the calendar; booked ones from this application.
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

// Settings is the owner's configuration, all of it editable in the
// admin surface and none of it compiled in.
type Settings struct {
	Zone        string // IANA name, never an offset
	SlotMins    int    // length of a bookable meeting
	BufferMins  int    // dead time kept around each booking
	MaxPerDay   int    // cap per calendar day in the owner's zone
	LeadHours   int    // how far ahead the first bookable slot sits
	HorizonDays int    // how far ahead the calendar is offered
	Windows     []Window
}

// Slot is one offered start time.
type Slot struct {
	Start time.Time
	End   time.Time
}

// Validate rejects a configuration that cannot produce sensible slots,
// so the admin surface refuses it rather than rendering an empty
// calendar that reads as "no availability".
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
			return fmt.Errorf("window on %s is not a range: %d to %d", w.Weekday, w.StartMins, w.EndMins)
		}
		if w.EndMins-w.StartMins < s.SlotMins {
			return fmt.Errorf("window on %s is shorter than one %d-minute slot", w.Weekday, s.SlotMins)
		}
	}
	return nil
}

// Slots returns every start time a member may book, in order.
//
// busy is what the calendar reports and carries no detail about what
// those intervals are: free/busy only, never event contents. booked is
// what this application has already taken, tracked separately because a
// held slot is not yet on the calendar and would otherwise be offered
// twice.
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

	// The cap counts bookings per local day, so it applies to the day a
	// slot falls in rather than to the list as a whole.
	perDay := map[string]int{}
	for _, b := range booked {
		perDay[b.Start.In(loc).Format("2006-01-02")]++
	}

	byWeekday := map[time.Weekday][]Window{}
	for _, w := range s.Windows {
		byWeekday[w.Weekday] = append(byWeekday[w.Weekday], w)
	}

	var out []Slot
	// Walk local calendar days. Adding 24 hours would drift across a DST
	// boundary; AddDate on a midnight-anchored date lands on the next
	// local midnight whatever the offset did.
	n := now.In(loc)
	day := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
	for ; day.Before(limit); day = day.AddDate(0, 0, 1) {
		windows := byWeekday[day.Weekday()]
		if len(windows) == 0 {
			continue
		}
		key := day.Format("2006-01-02")
		taken := perDay[key]
		for _, w := range windows {
			start := day.Add(time.Duration(w.StartMins) * time.Minute)
			end := day.Add(time.Duration(w.EndMins) * time.Minute)
			slotLen := time.Duration(s.SlotMins) * time.Minute
			step := time.Duration(s.SlotMins+s.BufferMins) * time.Minute
			for t := start; !t.Add(slotLen).After(end); t = t.Add(step) {
				if taken >= s.MaxPerDay {
					break
				}
				slot := Interval{Start: t, End: t.Add(slotLen)}
				if slot.Start.Before(earliest) {
					continue
				}
				// The buffer guards conflicts as well as spacing: with a
				// buffer configured, a booking ending at 10:00 blocks a
				// 10:00 start.
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
