package scheduling

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/reh3376/career-site/services/api/internal/calendar"
)

// Booking is a meeting this application has taken.
type Booking struct {
	ID       int64
	UserID   int64
	Name     string
	Email    string
	Note     string
	Start    time.Time
	DurMins  int
	EventID  string
	Created  time.Time
	Canceled *time.Time
	// How the meeting happens. Carried through to the row so the owner
	// sees it on his calendar entry and in the console.
	MeetingType   string
	VideoProvider string
	PhoneNumber   string
	// ContactLine is the rendered "how to reach them" sentence. Built by
	// the handler, which owns the wording, rather than here, which owns
	// the booking.
	ContactLine string
	// ContactPreference is the member's own free text, kept apart from
	// the structured fields so it reaches the owner unedited.
	ContactPreference string
}

// Store is what booking needs from the database. Claim is the important
// one: it must take the interval atomically, so two members submitting
// the same slot at the same moment cannot both succeed. Google cannot
// settle that race, because no event exists until after the claim.
type Store interface {
	// Claim records the booking, or returns ErrAlreadyClaimed if the
	// interval is already taken here.
	Claim(ctx context.Context, b Booking) (int64, error)
	// Active returns bookings that still hold time, for availability.
	Active(ctx context.Context, from, to time.Time) ([]Booking, error)
	// Attach records the calendar event id against a claimed booking.
	Attach(ctx context.Context, id int64, eventID string) error
	// Release removes a claim whose calendar event could not be made.
	Release(ctx context.Context, id int64) error
}

// ErrAlreadyClaimed means another member took the interval here first.
var ErrAlreadyClaimed = errors.New("that time has just been taken")

// describe is what the owner reads on his calendar entry: how to reach
// the member, then what they want to discuss.
//
// The contact line comes first because it is the thing needed at the
// moment the reminder fires. A calendar entry that says only the
// subject leaves him looking for a phone number two minutes before the
// call.
func (b Booking) describe() string {
	var parts []string
	if line := b.ContactLine; line != "" {
		parts = append(parts, line)
	}
	if b.Note != "" {
		parts = append(parts, b.Note)
	}
	return strings.Join(parts, "\n\n")
}

// Service books meetings.
type Service struct {
	Cal   calendar.Provider
	Store Store
	Log   *slog.Logger
	// Now is injectable so tests do not depend on the wall clock.
	Now func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Available returns bookable start times for a requested length.
//
// An unreachable calendar is an error rather than an empty list. A
// calendar that cannot be read is not a calendar with nothing on it,
// and rendering one as the other invites a member to book over
// something real.
func (s *Service) Available(ctx context.Context, set Settings, durationMins int) ([]Slot, error) {
	now := s.now()
	to := now.AddDate(0, 0, set.HorizonDays)

	busy, err := s.Cal.FreeBusy(ctx, now, to)
	if err != nil {
		return nil, fmt.Errorf("read availability: %w", err)
	}
	booked, err := s.Store.Active(ctx, now, to)
	if err != nil {
		return nil, fmt.Errorf("read existing bookings: %w", err)
	}
	return Slots(set, durationMins, toIntervals(busy), bookedIntervals(set, booked), now)
}

// Book claims a slot and puts it on the calendar.
//
// The order matters and is the whole point of this function.
//
//  1. Ask the calendar again, about this interval, right now. The list
//     the member was shown is stale: the owner's real calendar does not
//     stop moving while somebody fills in a note.
//  2. Claim the interval in the database, which settles the race
//     against another member booking the same slot here, because
//     Google cannot see a booking that has not been created yet.
//  3. Only then create the event.
//
// If the event cannot be created the claim is released, so a failure
// leaves no phantom hold that blocks the slot for everybody.
func (s *Service) Book(ctx context.Context, set Settings, b Booking) (Booking, error) {
	if !set.Allows(b.DurMins) {
		return Booking{}, fmt.Errorf("%d minutes is not an offered meeting length", b.DurMins)
	}
	end := b.Start.Add(time.Duration(b.DurMins) * time.Minute)

	// 1. The second look. Asked about this interval plus its clearance,
	// not about the week, so the answer is about the thing being
	// claimed.
	gap := time.Duration(set.GapMins) * time.Minute
	fresh, err := s.Cal.FreeBusy(ctx, b.Start.Add(-gap), end.Add(gap))
	if err != nil {
		// Refuse rather than proceed. Booking onto a calendar that
		// could not be checked is how two people arrive at once.
		return Booking{}, fmt.Errorf("confirm availability: %w", err)
	}
	booked, err := s.Store.Active(ctx, b.Start.Add(-gap), end.Add(gap))
	if err != nil {
		return Booking{}, fmt.Errorf("confirm availability: %w", err)
	}
	if err := StillFree(set, b.Start, b.DurMins, toIntervals(fresh), bookedIntervals(set, booked)); err != nil {
		return Booking{}, err
	}

	// 2. Take it here first.
	id, err := s.Store.Claim(ctx, b)
	if err != nil {
		return Booking{}, err
	}
	b.ID = id

	// 3. Now the calendar.
	eventID, err := s.Cal.Create(ctx, calendar.Event{
		Start:       b.Start,
		End:         end,
		Summary:     fmt.Sprintf("%s (%d min)", b.Name, b.DurMins),
		Description: b.describe(),
		Attendee:    b.Email,
	})
	if err != nil {
		if rErr := s.Store.Release(ctx, id); rErr != nil && s.Log != nil {
			// A claim that outlives its failed event blocks the slot for
			// everyone and shows on no calendar, so say so loudly.
			s.Log.Error("could not release a claim whose event failed",
				slog.Int64("booking", id), slog.String("error", rErr.Error()))
		}
		return Booking{}, fmt.Errorf("create the calendar event: %w", err)
	}
	if err := s.Store.Attach(ctx, id, eventID); err != nil {
		// The meeting exists on the calendar; losing the id only costs
		// the ability to cancel it from here, so this is not fatal to
		// the member's booking.
		if s.Log != nil {
			s.Log.Error("booking made but its event id was not recorded",
				slog.Int64("booking", id), slog.String("event", eventID),
				slog.String("error", err.Error()))
		}
	}
	b.EventID = eventID
	return b, nil
}

func toIntervals(in []calendar.Interval) []Interval {
	out := make([]Interval, 0, len(in))
	for _, i := range in {
		out = append(out, Interval{Start: i.Start, End: i.End})
	}
	return out
}

// bookedIntervals turns bookings into the time they actually hold.
func bookedIntervals(set Settings, in []Booking) []Interval {
	out := make([]Interval, 0, len(in))
	for _, b := range in {
		if b.Canceled != nil {
			continue
		}
		out = append(out, Interval{
			Start: b.Start,
			End:   b.Start.Add(time.Duration(b.DurMins) * time.Minute),
		})
	}
	_ = set
	return out
}
