package users

import (
	"context"
	"strconv"
	"testing"
	"time"
)

// The welcome-back panel against the shape production actually has.
//
// The first version of WelcomeBackFor counted distinct
// activity_events.session_id. That column exists on the table and
// nothing has ever written to it: all 272 production rows carry NULL.
// So the query returned zero visits, the panel rendered nothing, and
// the feature was dead on arrival.
//
// It passed its own check at the time because that check inserted rows
// with session_id populated. The data was shaped to match the
// assumption. These tests insert it the way the application does, which
// is with session_id left alone, so the mistake cannot come back
// quietly.

func TestWelcomeBackCountsVisitsWithoutASessionID(t *testing.T) {
	r, ctx := dtTestRepo(t)
	userID := seedWelcomeBackUser(t, r, ctx)

	now := time.Now().UTC()
	// Three visits: two older runs and a current one. Events inside a
	// run are minutes apart; the runs are days apart.
	seedActivity(t, r, ctx, userID, []time.Time{
		now.Add(-72 * time.Hour), now.Add(-72*time.Hour + 2*time.Minute),
		now.Add(-24 * time.Hour), now.Add(-24*time.Hour + 5*time.Minute),
		now.Add(-2 * time.Minute), now,
	})

	wb, err := r.WelcomeBackFor(ctx, userID)
	if err != nil {
		t.Fatalf("WelcomeBackFor: %v", err)
	}
	if wb.PreviousVisits != 2 {
		t.Errorf("previous visits = %d, want 2: the three runs are days apart "+
			"and every row has a NULL session_id, exactly as production does",
			wb.PreviousVisits)
	}
	if wb.LastVisitAt == nil {
		t.Fatal("no last visit; the panel renders nothing without one")
	}
	// The previous visit ended ~24h ago, not 72h and not now.
	gap := now.Sub(*wb.LastVisitAt)
	if gap < 23*time.Hour || gap > 25*time.Hour {
		t.Errorf("last visit was %v ago, want about 24h: it should be the end "+
			"of the previous run, not the oldest and not the current one", gap)
	}
}

// Two events a few minutes apart are one visit, not two. This is the
// whole reason the gap rule beats counting login events, which would
// read a member signing in twice in one sitting as two visits.
func TestWelcomeBackTreatsOneSittingAsOneVisit(t *testing.T) {
	r, ctx := dtTestRepo(t)
	userID := seedWelcomeBackUser(t, r, ctx)

	now := time.Now().UTC()
	seedActivity(t, r, ctx, userID, []time.Time{
		now.Add(-48 * time.Hour),
		now.Add(-10 * time.Minute), now.Add(-5 * time.Minute), now,
	})

	wb, err := r.WelcomeBackFor(ctx, userID)
	if err != nil {
		t.Fatalf("WelcomeBackFor: %v", err)
	}
	if wb.PreviousVisits != 1 {
		t.Errorf("previous visits = %d, want 1: the three recent events are "+
			"minutes apart and are one sitting", wb.PreviousVisits)
	}
}

// A first-time member gets no panel rather than "your 0th visit".
func TestWelcomeBackIsEmptyOnAFirstVisit(t *testing.T) {
	r, ctx := dtTestRepo(t)
	userID := seedWelcomeBackUser(t, r, ctx)

	seedActivity(t, r, ctx, userID, []time.Time{time.Now().UTC()})

	wb, err := r.WelcomeBackFor(ctx, userID)
	if err != nil {
		t.Fatalf("WelcomeBackFor: %v", err)
	}
	if wb.PreviousVisits != 0 {
		t.Errorf("previous visits = %d, want 0", wb.PreviousVisits)
	}
	if wb.LastVisitAt != nil {
		t.Errorf("last visit = %v, want none on a first visit", wb.LastVisitAt)
	}
}

// The home page is excluded by name. For a long time it was the only
// instrumented page, so including it would have made the panel tell
// every member that the last thing they looked at was the page they
// were standing on.
func TestWelcomeBackSkipsTheHomePageItself(t *testing.T) {
	r, ctx := dtTestRepo(t)
	userID := seedWelcomeBackUser(t, r, ctx)

	now := time.Now().UTC()
	seedView(t, r, ctx, userID, homeContentID, now.Add(-3*time.Minute))
	seedView(t, r, ctx, userID, "better-business-decisions-part-1", now.Add(-2*time.Minute))
	seedView(t, r, ctx, userID, homeContentID, now.Add(-time.Minute))

	wb, err := r.WelcomeBackFor(ctx, userID)
	if err != nil {
		t.Fatalf("WelcomeBackFor: %v", err)
	}
	if len(wb.RecentlyViewed) != 1 || wb.RecentlyViewed[0] != "better-business-decisions-part-1" {
		t.Errorf("recently viewed = %v, want only the article", wb.RecentlyViewed)
	}
}

// --- helpers ---

func seedWelcomeBackUser(t *testing.T, r *Repo, ctx context.Context) int64 {
	t.Helper()
	var id int64
	email := "welcome-back-" + time.Now().Format("20060102150405.000000000") + "@example.test"
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, name, status)
		 VALUES ($1, 'x', 'Welcome Back Test', 'active') RETURNING id`, email).
		Scan(&id); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = r.pool.Exec(ctx, `DELETE FROM activity_events WHERE user_id = $1`, id)
		_, _ = r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	})
	return id
}

// seedActivity inserts login events at the given times, leaving
// session_id NULL exactly as the application does.
func seedActivity(t *testing.T, r *Repo, ctx context.Context, userID int64, at []time.Time) {
	t.Helper()
	for i, when := range at {
		if _, err := r.pool.Exec(ctx,
			`INSERT INTO activity_events (user_id, kind, content_id, client_event_id, occurred_at)
			 VALUES ($1, 'login', '', $2, $3)`,
			userID, "seed-"+time.Now().Format("150405.000000000")+"-"+strconv.Itoa(i), when); err != nil {
			t.Fatalf("seed activity: %v", err)
		}
	}
}

func seedView(t *testing.T, r *Repo, ctx context.Context, userID int64, contentID string, when time.Time) {
	t.Helper()
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO activity_events (user_id, kind, content_id, client_event_id, occurred_at)
		 VALUES ($1, 'view', $2, $3, $4)`,
		userID, contentID,
		"seedv-"+contentID+"-"+time.Now().Format("150405.000000000"), when); err != nil {
		t.Fatalf("seed view: %v", err)
	}
}
