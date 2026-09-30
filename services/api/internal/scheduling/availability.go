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
	Zone string // IANA name, never an offset
	// Durations a member may choose, in minutes. The owner offers 15,
	// 30 and 45: the length is the member's decision because only they
	// know whether they need a quick question answered or a real
	// conversation, and offering one length makes both of those
	// awkward.
	Durations []int
	// GapMins is clearance between meetings, applied on both sides of a
	// candidate. It is not carved out of the grid in advance: nothing
	// is held until something is actually booked, and then a 15-minute
	// meeting occupies 30 minutes of the day, a 30 occupies 45 and a 45
	// occupies 60.
	GapMins int
	// StepMins is how finely start times are offered.
	StepMins    int
	MaxPerDay   int // cap per calendar day in the owner's zone
	LeadHours   int // how far ahead the first bookable slot sits
	HorizonDays int // how far ahead the calendar is offered
	Windows     []Window
}

// Allows reports whether a requested meeting length is one the owner
// offers. A member picking a length nobody offered is a bad request,
// not a silent round to the nearest.
func (s Settings) Allows(mins int) bool {
	for _, d := range s.Durations {
		if d == mins {
			return true
		}
	}
	return false
}

// Footprint is how much of the day a meeting of this length consumes:
// the meeting plus the clearance after it.
func (s Settings) Footprint(mins int) time.Duration {
	return time.Duration(mins+s.GapMins) * time.Minute
}

// Location resolves the zone. Validate has already refused an unknown
// one, so a failure here means the setting changed underneath us; UTC
// is the safe answer because it is wrong visibly rather than quietly.
func (s Settings) Location() *time.Location {
	loc, err := time.LoadLocation(s.Zone)
	if err != nil {
		return time.UTC
	}
	return loc
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
	// LoadLocation("") returns UTC and no error, so an empty zone would
	// pass every check here and then quietly offer the owner's mornings
	// in UTC while the page beside them says America/New_York. That is
	// 4 hours out under EDT and 5 under EST: wrong all year, never
	// right, and silent. Distinct from the fixed-offset mistake below,
	// which is right for half the year and so takes longer to notice.
	// Checked first because it is the cheapest to get wrong.
	if s.Zone == "" {
		return fmt.Errorf("a time zone is required, as an IANA name such as America/New_York")
	}
	if s.Zone[0] == '+' || s.Zone[0] == '-' {
		return fmt.Errorf("zone %q is a fixed offset; use an IANA name so it moves with daylight saving", s.Zone)
	}
	if _, err := time.LoadLocation(s.Zone); err != nil {
		return fmt.Errorf("zone %q is not an IANA location: %w", s.Zone, err)
	}
	if len(s.Durations) == 0 {
		return fmt.Errorf("no meeting lengths are offered")
	}
	for _, d := range s.Durations {
		if d <= 0 {
			return fmt.Errorf("meeting length must be positive, got %d", d)
		}
	}
	if s.GapMins < 0 {
		return fmt.Errorf("gap cannot be negative, got %d", s.GapMins)
	}
	if s.StepMins <= 0 {
		return fmt.Errorf("step must be positive, got %d", s.StepMins)
	}
	if s.MaxPerDay <= 0 {
		return fmt.Errorf("per-day cap must be positive, got %d", s.MaxPerDay)
	}
	if s.HorizonDays <= 0 {
		return fmt.Errorf("horizon must be positive, got %d days", s.HorizonDays)
	}
	if s.LeadHours < 0 {
		return fmt.Errorf("lead time cannot be negative, got %d hours", s.LeadHours)
	}
	// No windows is not an empty calendar, it is a calendar that can
	// never fill. The admin form has to refuse it, because saving it
	// looks like success and reads to a member as "never available".
	if len(s.Windows) == 0 {
		return fmt.Errorf("at least one bookable window is required")
	}
	byDay := map[time.Weekday][]Window{}
	for _, w := range s.Windows {
		if w.Weekday < time.Sunday || w.Weekday > time.Saturday {
			return fmt.Errorf("window weekday %d is not a day of the week", int(w.Weekday))
		}
		if w.StartMins < 0 || w.EndMins > 24*60 || w.StartMins >= w.EndMins {
			return fmt.Errorf("window on %s is not a range: %d to %d", w.Weekday, w.StartMins, w.EndMins)
		}
		if shortest := minOf(s.Durations); w.EndMins-w.StartMins < shortest {
			return fmt.Errorf("window on %s is shorter than the shortest meeting offered (%d minutes)", w.Weekday, shortest)
		}
		byDay[w.Weekday] = append(byDay[w.Weekday], w)
	}
	// Overlapping windows on one day would offer the same start time
	// twice, and the duplicate is invisible until a member sees it.
	for day, ws := range byDay {
		sorted := append([]Window(nil), ws...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].StartMins < sorted[j].StartMins })
		for i := 1; i < len(sorted); i++ {
			if sorted[i].StartMins < sorted[i-1].EndMins {
				return fmt.Errorf("two windows on %s overlap (%d to %d and %d to %d); merge them",
					day, sorted[i-1].StartMins, sorted[i-1].EndMins, sorted[i].StartMins, sorted[i].EndMins)
			}
		}
	}
	return nil
}

// Slots returns every start time a member may book for a meeting of
// the requested length, in order.
//
// The length is the member's choice, so availability has to be computed
// per length rather than once: a 45-minute meeting has fewer places to
// go than a 15-minute one, and computing the grid for the shortest and
// then filtering would offer starts that cannot actually hold the
// meeting asked for.
//
// busy is what the calendar reports, free/busy only and carrying no
// detail about what those intervals are. booked is what this
// application has already taken, tracked separately because a held slot
// is not yet on the calendar and would otherwise be offered twice.
//
// Clearance is applied as a guard on both sides of the candidate rather
// than baked into the stored intervals. Nothing is held until something
// is booked, and then the clearance falls out of the arithmetic: a
// 15-minute meeting leaves the next start 30 minutes away, a 30 leaves
// it 45, a 45 leaves it 60.
func Slots(s Settings, durationMins int, busy, booked []Interval, now time.Time) ([]Slot, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	if !s.Allows(durationMins) {
		return nil, fmt.Errorf("%d minutes is not an offered meeting length; the owner offers %v", durationMins, s.Durations)
	}
	loc, err := time.LoadLocation(s.Zone)
	if err != nil {
		return nil, err
	}

	earliest := now.Add(time.Duration(s.LeadHours) * time.Hour)
	limit := now.AddDate(0, 0, s.HorizonDays)
	meeting := time.Duration(durationMins) * time.Minute
	gap := time.Duration(s.GapMins) * time.Minute

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
		// The cap limits meetings actually taken, not slots displayed.
		// Once the day is full it offers nothing; until then it offers
		// every free slot, because hiding them would only make the
		// calendar look emptier than it is.
		if perDay[day.Format("2006-01-02")] >= s.MaxPerDay {
			continue
		}
		for _, w := range windows {
			start := day.Add(time.Duration(w.StartMins) * time.Minute)
			end := day.Add(time.Duration(w.EndMins) * time.Minute)
			step := time.Duration(s.StepMins) * time.Minute
			// The meeting must finish inside the window. Its clearance
			// may run past the end: the window bounds the conversation,
			// not the gap after it.
			for t := start; !t.Add(meeting).After(end); t = t.Add(step) {
				if t.Before(earliest) {
					continue
				}
				guard := Interval{Start: t.Add(-gap), End: t.Add(meeting).Add(gap)}
				if anyOverlap(guard, busy) || anyOverlap(guard, booked) {
					continue
				}
				out = append(out, Slot{Start: t, End: t.Add(meeting)})
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

func minOf(xs []int) int {
	m := xs[0]
	for _, x := range xs[1:] {
		if x < m {
			m = x
		}
	}
	return m
}
