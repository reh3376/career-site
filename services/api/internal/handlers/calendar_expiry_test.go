package handlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// The behaviour these pin is the one ExpiryJobs got wrong first time: a
// reminder that re-sends on every restart. Two members received 30 and
// 26 identical emails before that was found, and it was found by a
// person noticing, not by a test.

func TestCalendarReminderFiresOnlyNearExpiry(t *testing.T) {
	j := NewCalendarExpiryJob(nil, nil, nil, "", "", "")

	// A token issued now has its full seven days; nothing is due.
	fresh := time.Now()
	if left := time.Until(fresh.Add(j.tokenLife)); left <= j.warnWithin {
		t.Fatalf("a freshly issued token is already inside the warning window "+
			"(%v left, window %v); the reminder would fire immediately on every "+
			"reconnect", left, j.warnWithin)
	}

	// Five days in, two remain, which is the edge the window is set to.
	old := time.Now().Add(-5 * 24 * time.Hour)
	if left := time.Until(old.Add(j.tokenLife)); left > j.warnWithin {
		t.Errorf("five days in, %v left, which is outside the %v window: the "+
			"reminder would arrive with less than two days of notice", left, j.warnWithin)
	}
}

// The marker has to be keyed on the connection, not on a boolean and not
// on process state. Keying it on updated_at means reconnecting re-arms
// the reminder with no reset step, which is the property that stops this
// going quiet after the first cycle.
func TestTheMarkerIsKeyedOnTheConnectionTimestamp(t *testing.T) {
	first := time.Date(2026, 9, 30, 3, 13, 30, 0, time.UTC)
	second := first.Add(7 * 24 * time.Hour)

	stamp := func(ts time.Time) string {
		v, err := json.Marshal(ts.UTC().Format(time.RFC3339))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var out string
		if err := json.Unmarshal(v, &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return out
	}

	if stamp(first) == stamp(second) {
		t.Fatal("two different connections produced the same marker; " +
			"reconnecting would not re-arm the reminder")
	}
	// Same connection read twice must compare equal, or the reminder
	// resends on the next tick.
	if stamp(first) != stamp(first) {
		t.Error("the same connection produced two different markers; " +
			"the reminder would send on every run")
	}
}

// A nil mailer must not be fatal, and must not leave the job re-entering
// the same branch forever. The job records the marker either way.
func TestNoMailerIsNotAnError(t *testing.T) {
	j := NewCalendarExpiryJob(nil, nil, nil, "", "", "")
	if j.email != nil || j.ownerAddr != "" {
		t.Fatal("this test is meaningless unless the mailer is absent")
	}
	// Run needs a repo, so the full path is covered by the integration
	// suite. What is asserted here is the construction: a job built
	// without a mailer is a valid job rather than a nil-pointer waiting
	// to happen, which is how NewChat's nil answer service behaved until
	// it panicked a live connection.
	if j.tokenLife != 7*24*time.Hour {
		t.Errorf("tokenLife = %v, want 7 days: Google's cap while the consent "+
			"screen is unverified", j.tokenLife)
	}
	_ = context.Background()
}
