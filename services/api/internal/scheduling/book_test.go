package scheduling

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/reh3376/career-site/services/api/internal/calendar"
)

type memStore struct {
	mu       sync.Mutex
	rows     []Booking
	nextID   int64
	claimErr error
	released []int64
	attached map[int64]string
}

func newStore() *memStore { return &memStore{attached: map[int64]string{}} }

func (m *memStore) Claim(_ context.Context, b Booking) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.claimErr != nil {
		return 0, m.claimErr
	}
	end := b.Start.Add(time.Duration(b.DurMins) * time.Minute)
	for _, r := range m.rows {
		if r.Canceled != nil {
			continue
		}
		rEnd := r.Start.Add(time.Duration(r.DurMins) * time.Minute)
		if b.Start.Before(rEnd) && r.Start.Before(end) {
			return 0, ErrAlreadyClaimed
		}
	}
	m.nextID++
	b.ID = m.nextID
	m.rows = append(m.rows, b)
	return b.ID, nil
}

func (m *memStore) Active(_ context.Context, from, to time.Time) ([]Booking, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Booking
	for _, r := range m.rows {
		if r.Canceled == nil {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *memStore) Attach(_ context.Context, id int64, eventID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attached[id] = eventID
	return nil
}

func (m *memStore) Release(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.released = append(m.released, id)
	for i, r := range m.rows {
		if r.ID == id {
			m.rows = append(m.rows[:i], m.rows[i+1:]...)
			return nil
		}
	}
	return nil
}

func svc(cal calendar.Provider, st Store, now time.Time) *Service {
	return &Service{
		Cal: cal, Store: st,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: func() time.Time { return now },
	}
}

func TestBookingTakesTheSlotAndCreatesTheEvent(t *testing.T) {
	cal := calendar.NewStub()
	st := newStore()
	s := svc(cal, st, mustTime(t, "2026-10-05 00:00"))
	got, err := s.Book(context.Background(), ownerSettings(), Booking{
		Name: "A Recruiter", Email: "r@example.com", DurMins: 30,
		Start: mustTime(t, "2026-10-06 09:00"),
	})
	if err != nil {
		t.Fatalf("a free slot should book: %v", err)
	}
	if got.EventID == "" {
		t.Error("the booking should carry the calendar event id")
	}
	if len(cal.Created) != 1 {
		t.Fatalf("want one event created, got %d", len(cal.Created))
	}
	if cal.Created[0].Attendee != "r@example.com" {
		t.Errorf("the member should be the attendee, got %q", cal.Created[0].Attendee)
	}
}

// The race the owner asked about: the calendar moved between the member
// seeing the slot and submitting it.
func TestASlotTakenInGoogleAfterTheListWasShownIsRefused(t *testing.T) {
	cal := calendar.NewStub()
	st := newStore()
	s := svc(cal, st, mustTime(t, "2026-10-05 00:00"))

	// Something lands on the real calendar after the member started.
	cal.Busy = []calendar.Interval{{
		Start: mustTime(t, "2026-10-06 09:15"),
		End:   mustTime(t, "2026-10-06 09:45"),
	}}

	_, err := s.Book(context.Background(), ownerSettings(), Booking{
		Name: "A Recruiter", Email: "r@example.com", DurMins: 30,
		Start: mustTime(t, "2026-10-06 09:00"),
	})
	var taken ErrSlotTaken
	if !errors.As(err, &taken) {
		t.Fatalf("want ErrSlotTaken so the surface can offer fresh times, got %v", err)
	}
	if len(cal.Created) != 0 {
		t.Error("nothing should have been written to the calendar")
	}
	if len(st.rows) != 0 {
		t.Error("nothing should have been claimed")
	}
}

// A calendar that cannot be read is not a calendar with nothing on it.
func TestAnUnreachableCalendarRefusesRatherThanBooks(t *testing.T) {
	cal := calendar.NewStub()
	cal.Fail = calendar.ErrUnavailable
	s := svc(cal, newStore(), mustTime(t, "2026-10-05 00:00"))
	_, err := s.Book(context.Background(), ownerSettings(), Booking{
		Name: "A Recruiter", Email: "r@example.com", DurMins: 30,
		Start: mustTime(t, "2026-10-06 09:00"),
	})
	if err == nil {
		t.Fatal("an unreadable calendar must refuse the booking, not accept it")
	}
}

func TestAnUnreachableCalendarDoesNotShowAnEmptyWeek(t *testing.T) {
	cal := calendar.NewStub()
	cal.Fail = calendar.ErrUnavailable
	s := svc(cal, newStore(), mustTime(t, "2026-10-05 00:00"))
	if _, err := s.Available(context.Background(), ownerSettings(), 30); err == nil {
		t.Error("availability must error; an empty list reads as 'no times available'")
	}
}

// Google cannot settle this one, because no event exists until the
// claim succeeds.
func TestTwoMembersClaimingTheSameSlotCannotBothWin(t *testing.T) {
	cal := calendar.NewStub()
	st := newStore()
	s := svc(cal, st, mustTime(t, "2026-10-05 00:00"))
	set := ownerSettings()
	start := mustTime(t, "2026-10-06 09:00")

	first, err := s.Book(context.Background(), set, Booking{
		Name: "First", Email: "a@example.com", DurMins: 30, Start: start,
	})
	if err != nil {
		t.Fatalf("the first booking should succeed: %v", err)
	}
	_, err = s.Book(context.Background(), set, Booking{
		Name: "Second", Email: "b@example.com", DurMins: 30, Start: start,
	})
	if err == nil {
		t.Fatal("the second claim on the same interval must fail")
	}
	if len(cal.Created) != 1 {
		t.Errorf("exactly one event should exist, got %d", len(cal.Created))
	}
	if first.EventID == "" {
		t.Error("the winner should hold the event id")
	}
}

// A claim that outlives its failed event blocks the slot for everybody
// and appears on no calendar.
func TestAFailedEventReleasesTheClaim(t *testing.T) {
	cal := &failOnCreate{Stub: calendar.NewStub()}
	st := newStore()
	s := svc(cal, st, mustTime(t, "2026-10-05 00:00"))
	_, err := s.Book(context.Background(), ownerSettings(), Booking{
		Name: "A Recruiter", Email: "r@example.com", DurMins: 30,
		Start: mustTime(t, "2026-10-06 09:00"),
	})
	if err == nil {
		t.Fatal("the event could not be created; the booking should fail")
	}
	if len(st.released) != 1 {
		t.Errorf("the claim should have been released, releases=%v", st.released)
	}
	if len(st.rows) != 0 {
		t.Error("no claim should remain holding the slot")
	}
}

type failOnCreate struct{ *calendar.Stub }

func (f *failOnCreate) Create(context.Context, calendar.Event) (string, error) {
	return "", errors.New("calendar rejected the event")
}

func TestBookingRespectsTheClearanceAgainstAnExistingBooking(t *testing.T) {
	cal := calendar.NewStub()
	st := newStore()
	s := svc(cal, st, mustTime(t, "2026-10-05 00:00"))
	set := ownerSettings()

	if _, err := s.Book(context.Background(), set, Booking{
		Name: "First", Email: "a@example.com", DurMins: 30,
		Start: mustTime(t, "2026-10-06 09:00"),
	}); err != nil {
		t.Fatal(err)
	}
	// 09:30 is the moment the first ends; 15 minutes of clearance is
	// required, so this must be refused.
	if _, err := s.Book(context.Background(), set, Booking{
		Name: "Second", Email: "b@example.com", DurMins: 30,
		Start: mustTime(t, "2026-10-06 09:30"),
	}); err == nil {
		t.Error("09:30 leaves no clearance after a meeting ending at 09:30")
	}
	// 09:45 does.
	if _, err := s.Book(context.Background(), set, Booking{
		Name: "Third", Email: "c@example.com", DurMins: 30,
		Start: mustTime(t, "2026-10-06 09:45"),
	}); err != nil {
		t.Errorf("09:45 is exactly 15 minutes clear and should book: %v", err)
	}
}
