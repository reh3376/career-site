package calendar

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Stub is an in-memory calendar for development and tests.
//
// It exists so the whole booking flow can be exercised before any
// Google credential does, and so it stays exercisable afterwards. The
// alternative is testing against the owner's real calendar, which means
// either polluting it or not testing.
//
// It is deliberately not a no-op. It records what it was asked to
// create, honours what it was told is busy, and can be made to fail, so
// a test can assert that an unreachable calendar reads as unavailable
// rather than as free.
type Stub struct {
	mu sync.Mutex
	// Busy is what this calendar reports as taken. Tests set it.
	Busy []Interval
	// Created holds every event Create was asked to make, in order.
	Created []Event
	// Fail, when set, is returned by every method, so a test can drive
	// the unreachable path.
	Fail error
	next int
}

// NewStub returns a calendar with nothing on it.
func NewStub() *Stub { return &Stub{} }

func (s *Stub) FreeBusy(_ context.Context, from, to time.Time) ([]Interval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Fail != nil {
		return nil, s.Fail
	}
	// Return only what overlaps the window asked for, as Google does.
	// Returning everything would let a caller accidentally depend on
	// intervals it never requested.
	var out []Interval
	for _, b := range s.Busy {
		if b.Start.Before(to) && from.Before(b.End) {
			out = append(out, b)
		}
	}
	return out, nil
}

func (s *Stub) Create(_ context.Context, e Event) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Fail != nil {
		return "", s.Fail
	}
	s.next++
	e.ID = fmt.Sprintf("stub-%d", s.next)
	s.Created = append(s.Created, e)
	// A created meeting is busy from now on, so a stub used across two
	// calls behaves like a calendar rather than like a list.
	s.Busy = append(s.Busy, Interval{Start: e.Start, End: e.End})
	return e.ID, nil
}

func (s *Stub) Cancel(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Fail != nil {
		return s.Fail
	}
	for i, e := range s.Created {
		if e.ID != id {
			continue
		}
		s.Created = append(s.Created[:i], s.Created[i+1:]...)
		for j, b := range s.Busy {
			if b.Start.Equal(e.Start) && b.End.Equal(e.End) {
				s.Busy = append(s.Busy[:j], s.Busy[j+1:]...)
				break
			}
		}
		return nil
	}
	return fmt.Errorf("no event %q to cancel", id)
}

func (s *Stub) Healthy(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Fail
}

// NotConnected is a provider that refuses everything, which is what the
// application holds before anyone has authorised a calendar. It exists
// so the nil case is a real object with a real error rather than a nil
// pointer nobody remembered to check.
type NotConnected struct{}

func (NotConnected) FreeBusy(context.Context, time.Time, time.Time) ([]Interval, error) {
	return nil, ErrNotConnected
}
func (NotConnected) Create(context.Context, Event) (string, error) { return "", ErrNotConnected }
func (NotConnected) Cancel(context.Context, string) error          { return ErrNotConnected }
func (NotConnected) Healthy(context.Context) error                 { return ErrNotConnected }

var (
	_ Provider = (*Stub)(nil)
	_ Provider = NotConnected{}
)
