package scheduling

import (
	"context"
	"errors"
	"time"

	"github.com/reh3376/career-site/services/api/internal/users"
)

// RepoStore adapts the bookings repository to the Store this package
// books against.
//
// The two Booking shapes are deliberately not the same type. The
// repository's carries what the row holds, including the clearance
// actually applied when the row was written, so a booking made under an
// older gap setting keeps the footprint it was made with rather than
// being reinterpreted when the setting changes. This package's carries
// only what booking logic needs. Translating between them in one place
// keeps that difference from leaking into either side.
type RepoStore struct {
	Repo *users.Repo
	// Gap is the clearance to record on a new claim, read from the
	// settings in force at the moment of booking.
	Gap int
}

var _ Store = (*RepoStore)(nil)

// Claim records the booking, translating the database's exclusion
// violation into the taken-slot error this package understands.
func (s *RepoStore) Claim(ctx context.Context, b Booking) (int64, error) {
	var user *int64
	if b.UserID != 0 {
		id := b.UserID
		user = &id
	}
	id, err := s.Repo.ClaimBooking(ctx, users.Booking{
		UserID:      user,
		Name:        b.Name,
		Email:       b.Email,
		Note:        b.Note,
		StartsAt:    b.Start,
		DurationMin: b.DurMins,
		GapMin:      s.Gap,

		MeetingType:   b.MeetingType,
		VideoProvider: b.VideoProvider,
		PhoneNumber:   b.PhoneNumber,
	})
	if errors.Is(err, users.ErrSlotTaken) {
		return 0, ErrAlreadyClaimed
	}
	return id, err
}

// Active returns bookings that still hold time in the range.
func (s *RepoStore) Active(ctx context.Context, from, to time.Time) ([]Booking, error) {
	rows, err := s.Repo.ActiveBookings(ctx, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]Booking, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromRow(r))
	}
	return out, nil
}

// Attach records the calendar event id against a claimed booking.
func (s *RepoStore) Attach(ctx context.Context, id int64, eventID string) error {
	return s.Repo.AttachBookingEvent(ctx, id, eventID)
}

// Release removes a claim whose calendar event could not be made.
func (s *RepoStore) Release(ctx context.Context, id int64) error {
	return s.Repo.ReleaseBooking(ctx, id)
}

// fromRow converts a stored booking to the booking shape this package
// works in.
func fromRow(r users.Booking) Booking {
	b := Booking{
		ID:       r.ID,
		Name:     r.Name,
		Email:    r.Email,
		Note:     r.Note,
		Start:    r.StartsAt,
		DurMins:  r.DurationMin,
		EventID:  r.EventID,
		Created:  r.CreatedAt,
		Canceled: r.CancelledAt,

		MeetingType:   r.MeetingType,
		VideoProvider: r.VideoProvider,
		PhoneNumber:   r.PhoneNumber,
	}
	if r.UserID != nil {
		b.UserID = *r.UserID
	}
	return b
}

// FromRow is fromRow for callers outside this package, so a handler can
// render a stored booking without duplicating the field mapping.
func FromRow(r users.Booking) Booking { return fromRow(r) }
