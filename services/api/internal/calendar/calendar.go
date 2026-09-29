// Package calendar is the seam between the scheduler and Google.
//
// Everything the scheduler needs from a calendar is behind this
// interface, for the same reason the LLM gateway has one: the feature
// has to be buildable and testable before any credential exists, and
// has to stay testable afterwards. A scheduler that can only be
// exercised against the owner's real calendar is a scheduler nobody
// dares change.
//
// The interface is deliberately narrow, and the narrowness is a privacy
// decision rather than a design preference. The target is the owner's
// main personal calendar. FreeBusy returns intervals and nothing else:
// no titles, no attendees, no locations, no descriptions. This
// application never holds the content of his private life in order to
// answer "is this interval taken", and a leaked credential exposes the
// shape of his week rather than its substance.
package calendar

import (
	"context"
	"errors"
	"time"
)

// Interval is a half-open busy span. It carries no detail about what
// made the calendar busy, because nothing here needs to know.
type Interval struct {
	Start time.Time
	End   time.Time
}

// Event is a meeting this application created. Only bookings made here
// are ever described this fully; anything else on the calendar is seen
// as a bare interval.
type Event struct {
	ID          string
	Start       time.Time
	End         time.Time
	Summary     string
	Description string
	// Attendee is the member who booked. The owner is the calendar's
	// owner and is not listed as an attendee on his own event.
	Attendee string
}

// Provider is what the scheduler needs from a calendar.
type Provider interface {
	// FreeBusy returns busy intervals between from and to. It must not
	// return anything about what those intervals contain.
	FreeBusy(ctx context.Context, from, to time.Time) ([]Interval, error)
	// Create puts a meeting on the calendar and returns its id.
	Create(ctx context.Context, e Event) (string, error)
	// Cancel removes a meeting this application created.
	Cancel(ctx context.Context, id string) error
	// Healthy reports whether the provider can currently be used, so a
	// dead credential reads as "unavailable" rather than as a calendar
	// with nothing on it.
	Healthy(ctx context.Context) error
}

// ErrNotConnected is returned when no calendar has been authorised. It
// exists so the surface can say "scheduling is not set up" rather than
// showing an empty week, which a visitor would read as "he has no
// availability".
var ErrNotConnected = errors.New("no calendar is connected")

// ErrUnavailable is returned when the calendar cannot be reached or the
// credential has expired. Same reasoning: an unreachable calendar must
// never be rendered as a free one.
var ErrUnavailable = errors.New("the calendar is not reachable")
