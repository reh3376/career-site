package users

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reh3376/career-site/services/api/internal/tenant"
)

// These run against a real Postgres because the behaviour under test is
// a database constraint. A mock would assert that the code calls what
// the code calls, and would have passed just as happily against a table
// with no constraint on it at all.
//
//	MEETINGS_TEST_DB=postgres://career:career@localhost:55433/career go test ./internal/users/
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("MEETINGS_TEST_DB")
	if dsn == "" {
		t.Skip("set MEETINGS_TEST_DB to run the booking tests against a database")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), "DELETE FROM meeting_bookings"); err != nil {
		t.Fatalf("clean: %v", err)
	}
	return pool
}

func at(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

func booking(t *testing.T, start string, mins int) Booking {
	return Booking{
		Name: "A Member", Email: "m@example.com",
		StartsAt: at(t, start), DurationMin: mins, GapMin: 15,
	}
}

func TestASecondClaimOnTheSameTimeIsRefusedByTheDatabase(t *testing.T) {
	r := &Repo{pool: testPool(t)}
	ctx := tenant.WithID(context.Background(), tenant.Default)

	if _, err := r.ClaimBooking(ctx, booking(t, "2026-10-06T13:00:00Z", 30)); err != nil {
		t.Fatalf("the first claim should succeed: %v", err)
	}
	_, err := r.ClaimBooking(ctx, booking(t, "2026-10-06T13:30:00Z", 30))
	if !errors.Is(err, ErrSlotTaken) {
		t.Fatalf("an overlapping claim should report the slot as taken, got %v", err)
	}
}

// The owner's rule as the database enforces it: the held time is the
// meeting plus fifteen minutes, so the next claim must start that far
// on and not a minute sooner.
func TestEachLengthHoldsItsOwnPlusFifteen(t *testing.T) {
	for _, tc := range []struct {
		mins      int
		tooSoon   string
		farEnough string
	}{
		{15, "2026-10-06T13:15:00Z", "2026-10-06T13:30:00Z"},
		{30, "2026-10-06T13:30:00Z", "2026-10-06T13:45:00Z"},
		{45, "2026-10-06T13:45:00Z", "2026-10-06T14:00:00Z"},
	} {
		r := &Repo{pool: testPool(t)}
		ctx := tenant.WithID(context.Background(), tenant.Default)
		if _, err := r.ClaimBooking(ctx, booking(t, "2026-10-06T13:00:00Z", tc.mins)); err != nil {
			t.Fatalf("%d: first claim: %v", tc.mins, err)
		}
		if _, err := r.ClaimBooking(ctx, booking(t, tc.tooSoon, 15)); !errors.Is(err, ErrSlotTaken) {
			t.Errorf("%d min: %s leaves no clearance and should be refused, got %v", tc.mins, tc.tooSoon, err)
		}
		if _, err := r.ClaimBooking(ctx, booking(t, tc.farEnough, 15)); err != nil {
			t.Errorf("%d min: %s is exactly clear and should be allowed: %v", tc.mins, tc.farEnough, err)
		}
	}
}

func TestCancellingFreesTheTimeAndKeepsTheRow(t *testing.T) {
	r := &Repo{pool: testPool(t)}
	ctx := tenant.WithID(context.Background(), tenant.Default)

	id, err := r.ClaimBooking(ctx, booking(t, "2026-10-06T13:00:00Z", 30))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.CancelBooking(ctx, id); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := r.ClaimBooking(ctx, booking(t, "2026-10-06T13:00:00Z", 30)); err != nil {
		t.Errorf("the time should be free again after a cancellation: %v", err)
	}
	active, err := r.ActiveBookings(ctx, at(t, "2026-10-06T00:00:00Z"), at(t, "2026-10-07T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 {
		t.Errorf("only the new booking should hold time, got %d", len(active))
	}
}

// A claim whose event could not be created is deleted rather than
// cancelled: it never happened, and recording it as a cancellation
// would put a meeting in the member's history nobody agreed to.
func TestReleasingAClaimRemovesItEntirely(t *testing.T) {
	r := &Repo{pool: testPool(t)}
	ctx := tenant.WithID(context.Background(), tenant.Default)

	id, err := r.ClaimBooking(ctx, booking(t, "2026-10-06T13:00:00Z", 30))
	if err != nil {
		t.Fatal(err)
	}
	if err := r.ReleaseBooking(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ClaimBooking(ctx, booking(t, "2026-10-06T13:00:00Z", 30)); err != nil {
		t.Errorf("the released time should be claimable: %v", err)
	}
}

// Release must not touch a booking that did reach the calendar, or a
// late failure elsewhere would delete a meeting that exists.
func TestReleaseWillNotRemoveABookingThatHasAnEvent(t *testing.T) {
	r := &Repo{pool: testPool(t)}
	ctx := tenant.WithID(context.Background(), tenant.Default)

	id, err := r.ClaimBooking(ctx, booking(t, "2026-10-06T13:00:00Z", 30))
	if err != nil {
		t.Fatal(err)
	}
	if err := r.AttachBookingEvent(ctx, id, "google-event-1"); err != nil {
		t.Fatal(err)
	}
	if err := r.ReleaseBooking(ctx, id); err != nil {
		t.Fatal(err)
	}
	active, err := r.ActiveBookings(ctx, at(t, "2026-10-06T00:00:00Z"), at(t, "2026-10-07T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 {
		t.Error("a booking with a calendar event must survive release; the meeting exists")
	}
}
