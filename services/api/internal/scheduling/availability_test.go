package scheduling

import (
	"testing"
	"time"
)

// The owner's actual configuration, 2026-09-29.
func ownerSettings() Settings {
	return Settings{
		Zone:        "America/New_York",
		SlotMins:    30,
		BufferMins:  0,
		MaxPerDay:   4,
		LeadHours:   12,
		HorizonDays: 14,
		Windows: []Window{
			{time.Tuesday, 9 * 60, 12 * 60}, {time.Tuesday, 14 * 60, 16 * 60},
			{time.Wednesday, 9 * 60, 12 * 60}, {time.Wednesday, 14 * 60, 16 * 60},
			{time.Thursday, 9 * 60, 12 * 60}, {time.Thursday, 14 * 60, 16 * 60},
		},
	}
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	ts, err := time.ParseInLocation("2006-01-02 15:04", s, loc)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

func TestTwoWindowsADayLeaveTheLunchGapEmpty(t *testing.T) {
	s := ownerSettings()
	s.LeadHours = 0
	// A Tuesday.
	now := mustTime(t, "2026-10-06 00:00")
	s.HorizonDays = 1
	got, err := Slots(s, nil, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	// 09:00-12:00 is six 30-minute slots and 14:00-16:00 is four. The
	// per-day cap limits meetings taken, not slots shown, so with
	// nothing booked all ten are offerable.
	if len(got) != 10 {
		t.Fatalf("want 10 slots across the two windows, got %d", len(got))
	}
	for _, sl := range got {
		h := sl.Start.Hour()
		if h == 12 || h == 13 {
			t.Errorf("a slot was offered inside the lunch gap: %s", sl.Start.Format("15:04"))
		}
	}
}

func TestNothingIsOfferedOnADayWithNoWindow(t *testing.T) {
	s := ownerSettings()
	s.LeadHours = 0
	s.HorizonDays = 1
	// A Monday.
	got, err := Slots(s, nil, nil, mustTime(t, "2026-10-05 00:00"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("Monday is not a configured day; got %d slots", len(got))
	}
}

func TestAFreeEveningIsStillNotBookable(t *testing.T) {
	// The calendar subtracts from the windows and never adds to them.
	s := ownerSettings()
	s.LeadHours = 0
	s.HorizonDays = 1
	got, err := Slots(s, nil, nil, mustTime(t, "2026-10-06 00:00"))
	if err != nil {
		t.Fatal(err)
	}
	for _, sl := range got {
		if sl.Start.Hour() >= 16 || sl.Start.Hour() < 9 {
			t.Errorf("offered a slot outside the configured windows: %s", sl.Start.Format("15:04"))
		}
	}
}

func TestBusyTimeRemovesOnlyTheSlotsItCovers(t *testing.T) {
	s := ownerSettings()
	s.LeadHours = 0
	s.HorizonDays = 1
	s.MaxPerDay = 20
	busy := []Interval{{
		Start: mustTime(t, "2026-10-06 09:00"),
		End:   mustTime(t, "2026-10-06 10:00"),
	}}
	got, err := Slots(s, busy, nil, mustTime(t, "2026-10-06 00:00"))
	if err != nil {
		t.Fatal(err)
	}
	for _, sl := range got {
		hm := sl.Start.Format("15:04")
		if hm == "09:00" || hm == "09:30" {
			t.Errorf("offered %s, which the calendar says is busy", hm)
		}
	}
	// 10:00 touches the end of the busy block and must survive.
	var found bool
	for _, sl := range got {
		if sl.Start.Format("15:04") == "10:00" {
			found = true
		}
	}
	if !found {
		t.Error("10:00 touches the end of a busy block and is not a conflict; it should be offered")
	}
}

func TestBufferBlocksASlotThatMerelyTouches(t *testing.T) {
	s := ownerSettings()
	s.LeadHours = 0
	s.HorizonDays = 1
	s.MaxPerDay = 20
	s.BufferMins = 15
	busy := []Interval{{
		Start: mustTime(t, "2026-10-06 09:00"),
		End:   mustTime(t, "2026-10-06 10:00"),
	}}
	got, err := Slots(s, busy, nil, mustTime(t, "2026-10-06 00:00"))
	if err != nil {
		t.Fatal(err)
	}
	for _, sl := range got {
		if sl.Start.Format("15:04") == "10:00" {
			t.Error("with a 15-minute buffer, 10:00 is too close to a meeting ending at 10:00")
		}
	}
}

// The cap is per calendar day, so filling one day must not close
// another. It also limits meetings taken rather than slots shown, which
// is why the full day still offers its slots while the booked day
// offers none.
func TestCapClosesOneDayWithoutClosingTheNext(t *testing.T) {
	s := ownerSettings()
	s.LeadHours = 0
	s.HorizonDays = 8 // spans two Tuesdays
	s.MaxPerDay = 2
	booked := []Interval{
		{Start: mustTime(t, "2026-10-06 09:00"), End: mustTime(t, "2026-10-06 09:30")},
		{Start: mustTime(t, "2026-10-06 10:00"), End: mustTime(t, "2026-10-06 10:30")},
	}
	got, err := Slots(s, nil, booked, mustTime(t, "2026-10-06 00:00"))
	if err != nil {
		t.Fatal(err)
	}
	perDay := map[string]int{}
	for _, sl := range got {
		perDay[sl.Start.Format("2006-01-02")]++
	}
	if perDay["2026-10-06"] != 0 {
		t.Errorf("6 Oct has two bookings and a cap of two; it should offer nothing, got %d", perDay["2026-10-06"])
	}
	if perDay["2026-10-13"] == 0 {
		t.Error("13 Oct has no bookings and should still be open")
	}
}

func TestExistingBookingsCountTowardTheCap(t *testing.T) {
	s := ownerSettings()
	s.LeadHours = 0
	s.HorizonDays = 1
	s.MaxPerDay = 2
	booked := []Interval{
		{Start: mustTime(t, "2026-10-06 09:00"), End: mustTime(t, "2026-10-06 09:30")},
		{Start: mustTime(t, "2026-10-06 09:30"), End: mustTime(t, "2026-10-06 10:00")},
	}
	got, err := Slots(s, nil, booked, mustTime(t, "2026-10-06 00:00"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("the day's cap is already used by two bookings; got %d further slots", len(got))
	}
}

func TestLeadTimeKeepsTheNextHourUnbookable(t *testing.T) {
	s := ownerSettings()
	s.LeadHours = 12
	s.HorizonDays = 2
	// Tuesday 08:00 local: the 09:00 slot is an hour away and must not
	// be offered.
	got, err := Slots(s, nil, nil, mustTime(t, "2026-10-06 08:00"))
	if err != nil {
		t.Fatal(err)
	}
	for _, sl := range got {
		if sl.Start.Format("2006-01-02") == "2026-10-06" {
			t.Errorf("offered %s, inside the 12-hour lead time", sl.Start.Format("2006-01-02 15:04"))
		}
	}
}

// The reason the zone is an IANA name and the reason days are walked
// with AddDate. On 2026-11-01 the US moves off daylight time, so the
// Tuesday after it is UTC-5 where the Tuesday before was UTC-4. A 09:00
// window must still mean 09:00 to somebody in New York.
func TestWindowsHoldTheirLocalTimeAcrossADstChange(t *testing.T) {
	s := ownerSettings()
	s.LeadHours = 0
	s.HorizonDays = 16 // 2026-10-27 through past 2026-11-03
	s.MaxPerDay = 20
	got, err := Slots(s, nil, nil, mustTime(t, "2026-10-27 00:00"))
	if err != nil {
		t.Fatal(err)
	}
	var before, after bool
	for _, sl := range got {
		d := sl.Start.Format("2006-01-02")
		if sl.Start.Format("15:04") != "09:00" {
			continue
		}
		if d == "2026-10-27" {
			before = true
			if _, off := sl.Start.Zone(); off != -4*3600 {
				t.Errorf("27 Oct 09:00 should be UTC-4, offset was %d", off/3600)
			}
		}
		if d == "2026-11-03" {
			after = true
			if _, off := sl.Start.Zone(); off != -5*3600 {
				t.Errorf("3 Nov 09:00 should be UTC-5, offset was %d", off/3600)
			}
		}
	}
	if !before || !after {
		t.Fatalf("wanted a 09:00 slot on both Tuesdays either side of the DST change; before=%v after=%v", before, after)
	}
}

func TestBadSettingsAreRefusedRatherThanShowingAnEmptyCalendar(t *testing.T) {
	for _, tc := range []struct {
		name  string
		mutit func(*Settings)
	}{
		{"unknown zone", func(s *Settings) { s.Zone = "Mars/Olympus" }},
		{"zero slot length", func(s *Settings) { s.SlotMins = 0 }},
		{"zero cap", func(s *Settings) { s.MaxPerDay = 0 }},
		{"backwards window", func(s *Settings) { s.Windows[0].EndMins = s.Windows[0].StartMins - 60 }},
		{"window shorter than a slot", func(s *Settings) { s.Windows[0].EndMins = s.Windows[0].StartMins + 10 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := ownerSettings()
			tc.mutit(&s)
			if _, err := Slots(s, nil, nil, time.Now()); err == nil {
				t.Error("expected a refusal; an empty calendar reads as 'no availability' and hides the mistake")
			}
		})
	}
}
