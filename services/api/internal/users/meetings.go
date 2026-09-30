package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/reh3376/career-site/services/api/internal/tenant"
)

// Meeting bookings.
//
// The overlap rule lives in the database (migration 00041), not here.
// Two members can submit the same slot in the same second, and checking
// before inserting cannot settle that: both checks pass, both inserts
// run, and the owner discovers it when two people arrive. The exclusion
// constraint makes the second insert fail, and this file's job is to
// recognise that failure and report it as a taken slot rather than as
// an internal error.

// ErrSlotTaken means the database refused the booking because the time
// is already held. It is a normal outcome of two people wanting the
// same slot, not a fault.
var ErrSlotTaken = errors.New("that time is already booked")

// Booking is one meeting held on the owner's calendar.
type Booking struct {
	ID          int64
	UserID      *int64
	Name        string
	Email       string
	Note        string
	StartsAt    time.Time
	DurationMin int
	GapMin      int
	EventID     string
	CreatedAt   time.Time
	CancelledAt *time.Time
}

// ClaimBooking takes the interval, or reports that somebody else has.
//
// Nothing is written to the calendar here. The claim comes first
// precisely so that a lost race costs nothing: no event was created, so
// there is nothing to undo.
func (r *Repo) ClaimBooking(ctx context.Context, b Booking) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
    INSERT INTO meeting_bookings
      (tenant_id, user_id, name, email, note, starts_at, duration_min, gap_min)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    RETURNING id`,
		tenant.FromContext(ctx).Int64(), b.UserID, b.Name, b.Email, b.Note,
		b.StartsAt, b.DurationMin, b.GapMin).Scan(&id)
	if err != nil {
		// 23P01 is exclusion_violation, which here means exactly one
		// thing: the time is held. Reporting it as a database error
		// would show a member a stack trace for something ordinary.
		if isExclusionViolation(err, "meeting_bookings_no_overlap") {
			return 0, ErrSlotTaken
		}
		return 0, fmt.Errorf("claim booking: %w", err)
	}
	return id, nil
}

// AttachBookingEvent records the calendar event id against a claim.
func (r *Repo) AttachBookingEvent(ctx context.Context, id int64, eventID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE meeting_bookings SET event_id = $2 WHERE id = $1`, id, eventID)
	if err != nil {
		return fmt.Errorf("attach booking event: %w", err)
	}
	return nil
}

// ReleaseBooking deletes a claim whose calendar event could not be
// created.
//
// Deleted rather than cancelled, because it never happened: a
// cancellation is a meeting that existed and was called off, and
// recording a failed write as one would put a meeting in the member's
// history that nobody ever agreed to.
func (r *Repo) ReleaseBooking(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM meeting_bookings WHERE id = $1 AND event_id = ''`, id)
	if err != nil {
		return fmt.Errorf("release booking: %w", err)
	}
	return nil
}

// CancelBooking marks a meeting cancelled, which frees its time while
// keeping the fact that it happened.
func (r *Repo) CancelBooking(ctx context.Context, id int64) (Booking, error) {
	var b Booking
	err := r.pool.QueryRow(ctx, `
    UPDATE meeting_bookings SET cancelled_at = now()
     WHERE id = $1 AND cancelled_at IS NULL
    RETURNING id, user_id, name, email, note, starts_at, duration_min, gap_min,
              event_id, created_at, cancelled_at`, id).Scan(
		&b.ID, &b.UserID, &b.Name, &b.Email, &b.Note, &b.StartsAt,
		&b.DurationMin, &b.GapMin, &b.EventID, &b.CreatedAt, &b.CancelledAt)
	if err != nil {
		return Booking{}, ErrNotFound
	}
	return b, nil
}

// ActiveBookings returns bookings holding time in a window.
func (r *Repo) ActiveBookings(ctx context.Context, from, to time.Time) ([]Booking, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT id, user_id, name, email, note, starts_at, duration_min, gap_min,
           event_id, created_at, cancelled_at
      FROM meeting_bookings
     WHERE tenant_id = $1 AND cancelled_at IS NULL AND held && tstzrange($2, $3, '[)')
     ORDER BY starts_at`, tenant.FromContext(ctx).Int64(), from, to)
	if err != nil {
		return nil, fmt.Errorf("active bookings: %w", err)
	}
	defer rows.Close()
	return scanBookings(rows)
}

// BookingsForUser returns a member's own meetings, newest first, so the
// surface can show what they have booked without them asking.
func (r *Repo) BookingsForUser(ctx context.Context, userID int64) ([]Booking, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT id, user_id, name, email, note, starts_at, duration_min, gap_min,
           event_id, created_at, cancelled_at
      FROM meeting_bookings
     WHERE user_id = $1
     ORDER BY starts_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("bookings for user: %w", err)
	}
	defer rows.Close()
	return scanBookings(rows)
}

func scanBookings(rows rowScanner) ([]Booking, error) {
	var out []Booking
	for rows.Next() {
		var b Booking
		if err := rows.Scan(&b.ID, &b.UserID, &b.Name, &b.Email, &b.Note,
			&b.StartsAt, &b.DurationMin, &b.GapMin, &b.EventID,
			&b.CreatedAt, &b.CancelledAt); err != nil {
			return nil, fmt.Errorf("scan booking: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// isExclusionViolation recognises Postgres 23P01, the error an EXCLUDE
// constraint raises. Same shape as isUniqueViolation above, and
// separate because the two mean different things: a unique violation is
// a duplicate, an exclusion violation is an overlap.
func isExclusionViolation(err error, constraint string) bool {
	type sqlErr interface{ SQLState() string }
	if e, ok := err.(sqlErr); ok && e.SQLState() == "23P01" {
		return true
	}
	return constraint != "" && errorContains(err, constraint)
}

// AllBookings returns every booking for the owner's console, soonest
// first.
//
// Unlike ActiveBookings this is not scoped to a window and optionally
// includes what is finished or cancelled, because the question here is
// "what has been taken" rather than "what holds time". Cancelled rows
// are kept rather than deleted, so a meeting that vanished from
// somebody's calendar can still be explained.
func (r *Repo) AllBookings(ctx context.Context, includePast bool) ([]Booking, error) {
	q := `
    SELECT id, user_id, name, email, note, starts_at, duration_min, gap_min,
           event_id, created_at, cancelled_at
      FROM meeting_bookings
     WHERE tenant_id = $1`
	if !includePast {
		q += ` AND cancelled_at IS NULL AND starts_at >= now()`
	}
	q += ` ORDER BY starts_at`

	rows, err := r.pool.Query(ctx, q, tenant.FromContext(ctx).Int64())
	if err != nil {
		return nil, fmt.Errorf("all bookings: %w", err)
	}
	defer rows.Close()
	return scanBookings(rows)
}

// BookingByID returns one booking, or ok false.
func (r *Repo) BookingByID(ctx context.Context, id int64) (Booking, bool, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT id, user_id, name, email, note, starts_at, duration_min, gap_min,
           event_id, created_at, cancelled_at
      FROM meeting_bookings
     WHERE tenant_id = $1 AND id = $2`, tenant.FromContext(ctx).Int64(), id)
	if err != nil {
		return Booking{}, false, fmt.Errorf("booking by id: %w", err)
	}
	defer rows.Close()
	out, err := scanBookings(rows)
	if err != nil {
		return Booking{}, false, err
	}
	if len(out) == 0 {
		return Booking{}, false, nil
	}
	return out[0], true, nil
}
