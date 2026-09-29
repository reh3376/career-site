package scheduling

import (
	"testing"
	"time"
)

// The owner's actual configuration, 2026-09-29.
func ownerSettings() Settings {
	return Settings{
		Zone:        "America/New_York",
		Durations:   []int{15, 30, 45},
		GapMins:     15,
		StepMins:    15,
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
	got, err := Slots(s, 30, nil, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	// Starts are offered every 15 minutes and a 30-minute meeting has to
	// finish inside its window: 09:00 to 11:30 is eleven starts, 14:00
	// to 15:30 is seven. The per-day cap limits meetings taken, not
	// slots shown, so with nothing booked all eighteen are offerable.
	if len(got) != 18 {
		t.Fatalf("want 18 starts across the two windows, got %d", len(got))
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
	got, err := Slots(s, 30, nil, nil, mustTime(t, "2026-10-05 00:00"))
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
	got, err := Slots(s, 30, nil, nil, mustTime(t, "2026-10-06 00:00"))
	if err != nil {
		t.Fatal(err)
	}
	for _, sl := range got {
		if sl.Start.Hour() >= 16 || sl.Start.Hour() < 9 {
			t.Errorf("offered a slot outside the configured windows: %s", sl.Start.Format("15:04"))
		}
	}
}

// Busy time removes what it covers and no more than it has to. With
// clearance the boundary moves by fifteen minutes, but a slot well
// clear of the block must survive.
func TestBusyTimeRemovesOnlyWhatItHasTo(t *testing.T) {
	s := ownerSettings()
	s.LeadHours = 0
	s.HorizonDays = 1
	s.MaxPerDay = 20
	busy := []Interval{{
		Start: mustTime(t, "2026-10-06 09:00"),
		End:   mustTime(t, "2026-10-06 10:00"),
	}}
	got, err := Slots(s, 30, busy, nil, mustTime(t, "2026-10-06 00:00"))
	if err != nil {
		t.Fatal(err)
	}
	blocked := map[string]bool{"09:00": true, "09:15": true, "09:30": true, "10:00": true}
	var sawLater bool
	for _, sl := range got {
		hm := sl.Start.Format("15:04")
		if blocked[hm] {
			t.Errorf("offered %s, which the calendar or its clearance covers", hm)
		}
		if hm == "10:30" {
			sawLater = true
		}
	}
	if !sawLater {
		t.Error("10:30 is well clear of a block ending at 10:00 and should still be offered")
	}
}

// The clearance is not optional and not carved out in advance. A
// meeting ending at 10:00 means the next one cannot start until 10:15.
func TestClearanceKeepsFifteenMinutesBetweenMeetings(t *testing.T) {
	s := ownerSettings()
	s.LeadHours = 0
	s.HorizonDays = 1
	s.MaxPerDay = 20
	busy := []Interval{{
		Start: mustTime(t, "2026-10-06 09:00"),
		End:   mustTime(t, "2026-10-06 10:00"),
	}}
	got, err := Slots(s, 30, busy, nil, mustTime(t, "2026-10-06 00:00"))
	if err != nil {
		t.Fatal(err)
	}
	for _, sl := range got {
		if hm := sl.Start.Format("15:04"); hm == "10:00" {
			t.Error("10:00 is only a moment after a meeting ending at 10:00; 15 minutes of clearance is required")
		}
	}
	var at1015 bool
	for _, sl := range got {
		if sl.Start.Format("15:04") == "10:15" {
			at1015 = true
		}
	}
	if !at1015 {
		t.Error("10:15 is exactly 15 minutes after the meeting ended and should be offered")
	}
}

// What the owner asked for, stated as arithmetic: a booked meeting
// consumes its own length plus the clearance, so the next available
// start is 30, 45 or 60 minutes after it began.
func TestABookedMeetingConsumesItsLengthPlusFifteen(t *testing.T) {
	for _, tc := range []struct {
		mins      int
		nextStart string
	}{
		{15, "09:30"},
		{30, "09:45"},
		{45, "10:00"},
	} {
		s := ownerSettings()
		s.LeadHours = 0
		s.HorizonDays = 1
		s.MaxPerDay = 20
		booked := []Interval{{
			Start: mustTime(t, "2026-10-06 09:00"),
			End:   mustTime(t, "2026-10-06 09:00").Add(time.Duration(tc.mins) * time.Minute),
		}}
		got, err := Slots(s, 15, nil, booked, mustTime(t, "2026-10-06 00:00"))
		if err != nil {
			t.Fatal(err)
		}
		var first string
		for _, sl := range got {
			if sl.Start.Format("2006-01-02") == "2026-10-06" && sl.Start.Format("15:04") >= "09:00" {
				first = sl.Start.Format("15:04")
				break
			}
		}
		if first != tc.nextStart {
			t.Errorf("a %d-minute meeting at 09:00 should free %s; first offered was %s",
				tc.mins, tc.nextStart, first)
		}
	}
}

// A longer meeting has fewer places to go, so availability has to be
// computed for the length actually requested rather than once.
func TestALongerMeetingHasFewerPlacesToGo(t *testing.T) {
	s := ownerSettings()
	s.LeadHours = 0
	s.HorizonDays = 1
	s.MaxPerDay = 20
	// A gap between two busy blocks big enough for 15 but not 45.
	busy := []Interval{
		{Start: mustTime(t, "2026-10-06 09:00"), End: mustTime(t, "2026-10-06 09:30")},
		{Start: mustTime(t, "2026-10-06 10:30"), End: mustTime(t, "2026-10-06 12:00")},
	}
	short, err := Slots(s, 15, busy, nil, mustTime(t, "2026-10-06 00:00"))
	if err != nil {
		t.Fatal(err)
	}
	long, err := Slots(s, 45, busy, nil, mustTime(t, "2026-10-06 00:00"))
	if err != nil {
		t.Fatal(err)
	}
	if len(short) <= len(long) {
		t.Errorf("a 15-minute meeting should fit in more places than a 45: got %d and %d", len(short), len(long))
	}
}

func TestALengthNobodyOffersIsRefused(t *testing.T) {
	s := ownerSettings()
	if _, err := Slots(s, 20, nil, nil, time.Now()); err == nil {
		t.Error("20 minutes is not offered; it should be refused rather than rounded to something nearby")
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
	got, err := Slots(s, 30, nil, booked, mustTime(t, "2026-10-06 00:00"))
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
	got, err := Slots(s, 30, nil, booked, mustTime(t, "2026-10-06 00:00"))
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
	got, err := Slots(s, 30, nil, nil, mustTime(t, "2026-10-06 08:00"))
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
	got, err := Slots(s, 30, nil, nil, mustTime(t, "2026-10-27 00:00"))
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
		{"no lengths offered", func(s *Settings) { s.Durations = nil }},
		{"zero cap", func(s *Settings) { s.MaxPerDay = 0 }},
		{"backwards window", func(s *Settings) { s.Windows[0].EndMins = s.Windows[0].StartMins - 60 }},
		{"window shorter than the shortest meeting", func(s *Settings) { s.Windows[0].EndMins = s.Windows[0].StartMins + 10 }},
		// LoadLocation("") returns UTC with no error, so an empty zone
		// used to pass every check and then offer the owner's mornings
		// in UTC while the page beside them said America/New_York.
		{"empty zone", func(s *Settings) { s.Zone = "" }},
		// A fixed offset is right for half the year. America/New_York
		// is UTC-5 in winter and UTC-4 in summer.
		{"fixed offset instead of an IANA name", func(s *Settings) { s.Zone = "-05:00" }},
		// Not an empty calendar, a calendar that can never fill.
		{"no windows at all", func(s *Settings) { s.Windows = nil }},
		{"negative lead time", func(s *Settings) { s.LeadHours = -1 }},
		{"weekday out of range", func(s *Settings) { s.Windows[0].Weekday = 9 }},
		// Would offer the same start time twice, invisibly.
		{"two windows on one day overlap", func(s *Settings) {
			s.Windows = append(s.Windows, Window{
				Weekday:   s.Windows[0].Weekday,
				StartMins: s.Windows[0].StartMins + 30,
				EndMins:   s.Windows[0].EndMins + 30,
			})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := ownerSettings()
			tc.mutit(&s)
			if _, err := Slots(s, 30, nil, nil, time.Now()); err == nil {
				t.Error("expected a refusal; an empty calendar reads as 'no availability' and hides the mistake")
			}
		})
	}
}
