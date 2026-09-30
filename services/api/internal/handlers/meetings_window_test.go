package handlers

import (
	"testing"
	"time"

	"github.com/reh3376/career-site/services/api/internal/scheduling"
)

// The horizon is a date, not an instant.
//
// Measuring it as now + N*24h clipped the furthest day by however late
// in the day someone looked: at 23:00 Eastern a 21-day horizon reached
// only to 23:00 on day 20, so the last bookable days vanished and came
// back the next morning. Availability that shrinks through the evening
// is indistinguishable from the owner filling up.
func TestHorizonReachesTheEndOfTheLastDayWheneverYouLook(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("zone: %v", err)
	}
	set := scheduling.Settings{Zone: "America/New_York", LeadHours: 24, HorizonDays: 21}
	h := &Meetings{}

	// Same calendar day, two very different times of day.
	morning := time.Date(2026, 9, 29, 8, 0, 0, 0, loc)
	night := time.Date(2026, 9, 29, 23, 0, 0, 0, loc)

	_, toMorning := h.window(set, morning, time.Time{}, time.Time{}, false, false)
	_, toNight := h.window(set, night, time.Time{}, time.Time{}, false, false)

	wantDay := time.Date(2026, 10, 20, 0, 0, 0, 0, loc)
	for name, got := range map[string]time.Time{"morning": toMorning, "night": toNight} {
		g := got.In(loc)
		if g.Year() != wantDay.Year() || g.Month() != wantDay.Month() || g.Day() != wantDay.Day() {
			t.Errorf("%s: horizon lands on %s, want %s", name, g.Format("2006-01-02"), wantDay.Format("2006-01-02"))
		}
		// It must cover that day's working hours, not stop part way in.
		if g.Hour() < 23 {
			t.Errorf("%s: horizon stops at %s, which clips the last day", name, g.Format("15:04"))
		}
	}
	// And the two must agree, which is the property that was broken.
	if !toMorning.In(loc).Truncate(time.Hour).Equal(toNight.In(loc).Truncate(time.Hour)) {
		t.Errorf("horizon moved with the clock: morning %s, night %s",
			toMorning.In(loc).Format(time.RFC3339), toNight.In(loc).Format(time.RFC3339))
	}
}

// The lead time is still an instant, which is correct: "not in the next
// 24 hours" really does mean hours.
func TestLeadTimeStaysAnInstant(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	set := scheduling.Settings{Zone: "America/New_York", LeadHours: 24, HorizonDays: 21}
	h := &Meetings{}
	now := time.Date(2026, 9, 29, 23, 0, 0, 0, loc)

	from, _ := h.window(set, now, time.Time{}, time.Time{}, false, false)
	if want := now.Add(24 * time.Hour); !from.Equal(want) {
		t.Errorf("earliest is %s, want %s", from.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}
