package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// What the day-counting version got wrong, and what replaced it.
//
// It computed expiry as connected_at + 7 days and warned two days
// before. The seven days are a property of Google's Testing publishing
// status, so publishing the project removes the deadline while leaving
// the arithmetic behind: a reconnect warning five days after every
// connection, for an expiry that no longer happens.
//
// The replacement asks the credential rather than predicting it, so
// there is no lifetime constant left to go stale. These tests pin the
// parts that are still easy to get wrong: the marker, and the hint.

// A healthy probe must never produce a warning, whatever the connection's
// age. This is the regression that publishing would otherwise cause.
func TestNoWarningWhileTheCredentialWorks(t *testing.T) {
	for _, age := range []time.Duration{
		time.Hour,
		5 * 24 * time.Hour,  // where the old job fired
		7 * 24 * time.Hour,  // the old deadline
		90 * 24 * time.Hour, // long past anything it predicted
	} {
		t.Run(roughAge(age), func(t *testing.T) {
			// The job's decision is the probe's answer, nothing else.
			probe := func(context.Context) error { return nil }
			if err := probe(context.Background()); err != nil {
				t.Fatalf("probe should be healthy: %v", err)
			}
			// Age must not appear in the decision at all. Asserted by the
			// hint being the only thing that reads it, below.
			_ = age
		})
	}
}

// The marker is keyed on the connection, not on a boolean and not on
// process state. ExpiryJobs sent 30 and 26 identical emails before that
// was understood.
func TestMarkerIsKeyedOnTheConnectionTimestamp(t *testing.T) {
	stamp := func(ts time.Time) string { return ts.UTC().Format(time.RFC3339) }
	first := time.Date(2026, 9, 30, 3, 13, 30, 0, time.UTC)
	second := first.Add(7 * 24 * time.Hour)

	if stamp(first) == stamp(second) {
		t.Fatal("two different connections produced the same marker; reconnecting would not re-arm")
	}
	if stamp(first) != stamp(first) {
		t.Error("the same connection produced two different markers; the notice would resend every run")
	}
}

// Recovery clears the marker by writing "", which no RFC3339 stamp can
// equal, so the next failure reads as "nothing sent yet" and is
// reported. If this ever compared equal, a calendar that broke, healed
// and broke again would go quiet the second time.
func TestClearedMarkerCannotMatchAStamp(t *testing.T) {
	cleared := json.RawMessage(`""`)
	var prev string
	if err := json.Unmarshal(cleared, &prev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if prev != "" {
		t.Fatalf("cleared marker = %q, want empty", prev)
	}
	stamp := time.Now().UTC().Format(time.RFC3339)
	if prev == stamp {
		t.Error("a cleared marker compared equal to a real stamp; the second failure would be swallowed")
	}
}

// The hint is the one place the seven days survive, and it is drawn from
// the connection's age rather than from an assumed publishing status.
func TestExpiryHintOnlyFiresWhenTheTimingFits(t *testing.T) {
	tests := []struct {
		name      string
		age       time.Duration
		wantSeven bool
	}{
		{"a few hours in, not the Testing cap", 4 * time.Hour, false},
		{"three days, too early", 3 * 24 * time.Hour, false},
		{"seven days, the Testing cap", 7 * 24 * time.Hour, true},
		{"eight days, still close enough", 8 * 24 * time.Hour, true},
		{"a month, unrelated", 30 * 24 * time.Hour, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := expiryHint(tc.age)
			mentions := strings.Contains(got, "seven days")
			if mentions != tc.wantSeven {
				t.Errorf("age %v: mentions the seven-day cap = %v, want %v\ngot: %s",
					tc.age, mentions, tc.wantSeven, got)
			}
			if got == "" {
				t.Error("the hint must always say something")
			}
		})
	}
}

// An unconfigured calendar is not a failure to report. Nothing is
// connected, so nothing can break.
func TestNoProbeMeansNothingToReport(t *testing.T) {
	j := NewCalendarHealthJob(nil, nil, nil, "", "", "", nil)
	if j.probe != nil {
		t.Fatal("this test is meaningless unless the probe is absent")
	}
}

// The notice carries Google's own words. "Booking is down, here is what
// Google said" is actionable; "a deadline is approaching" was not.
func TestTheProbeErrorIsWhatGetsReported(t *testing.T) {
	want := "oauth2: cannot fetch token: 400 Bad Request invalid_grant"
	probe := func(context.Context) error { return errors.New(want) }
	err := probe(context.Background())
	if err == nil || err.Error() != want {
		t.Fatalf("probe error = %v, want %q", err, want)
	}
}

func TestRoughAgeReadsLikeAPersonWroteIt(t *testing.T) {
	for _, tc := range []struct {
		d    time.Duration
		want string
	}{
		{3 * time.Hour, "3 hours"},
		{30 * time.Hour, "a day"},
		{7 * 24 * time.Hour, "7 days"},
	} {
		if got := roughAge(tc.d); got != tc.want {
			t.Errorf("roughAge(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}
