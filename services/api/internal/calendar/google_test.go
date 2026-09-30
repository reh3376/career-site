package calendar

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeGoogle stands in for Google's endpoints so these tests never
// touch the network. It records what was asked, because several of the
// properties worth testing are about the request rather than the reply.
type fakeGoogle struct {
	srv *httptest.Server

	tokenCalls  int
	tokenStatus int
	expiresIn   int

	freeBusyBody string
	eventsStatus int

	lastEventBody map[string]any
	lastMethod    string
	lastPath      string
	lastQuery     string
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	t.Helper()
	f := &fakeGoogle{tokenStatus: 200, expiresIn: 3600, eventsStatus: 200}
	mux := http.NewServeMux()

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		f.tokenCalls++
		if f.tokenStatus != 200 {
			w.WriteHeader(f.tokenStatus)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "access-token", "expires_in": f.expiresIn,
		})
	})
	mux.HandleFunc("/freebusy", func(w http.ResponseWriter, r *http.Request) {
		f.lastMethod, f.lastPath = r.Method, r.URL.Path
		_, _ = w.Write([]byte(f.freeBusyBody))
	})
	mux.HandleFunc("/calendars/", func(w http.ResponseWriter, r *http.Request) {
		f.lastMethod, f.lastPath, f.lastQuery = r.Method, r.URL.Path, r.URL.RawQuery
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &f.lastEventBody)
		if f.eventsStatus != 200 {
			w.WriteHeader(f.eventsStatus)
			return
		}
		_, _ = w.Write([]byte(`{"id":"event-123"}`))
	})

	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)

	oldToken, oldFB, oldEv := googleTokenURL, googleFreeBusyURL, googleEventsURL
	googleTokenURL = f.srv.URL + "/token"
	googleFreeBusyURL = f.srv.URL + "/freebusy"
	googleEventsURL = f.srv.URL + "/calendars/%s/events"
	t.Cleanup(func() {
		googleTokenURL, googleFreeBusyURL, googleEventsURL = oldToken, oldFB, oldEv
	})
	return f
}

func newGoogle(f *fakeGoogle) *Google {
	return &Google{
		ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh",
		CalendarID: "primary", HTTP: f.srv.Client(),
	}
}

func TestFreeBusyReturnsIntervals(t *testing.T) {
	f := newFakeGoogle(t)
	f.freeBusyBody = `{"calendars":{"primary":{"busy":[
	  {"start":"2026-10-06T13:00:00Z","end":"2026-10-06T14:00:00Z"}]}}}`

	busy, err := newGoogle(f).FreeBusy(context.Background(),
		time.Now(), time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("free/busy: %v", err)
	}
	if len(busy) != 1 {
		t.Fatalf("got %d intervals, want 1", len(busy))
	}
	if !busy[0].Start.Equal(time.Date(2026, 10, 6, 13, 0, 0, 0, time.UTC)) {
		t.Errorf("start is %v", busy[0].Start)
	}
}

// Google answers per calendar. A missing entry means it did not answer
// about the one asked for, and reading that as "nothing is busy" would
// offer the owner's entire week.
func TestFreeBusyWithNoAnswerForTheCalendarIsAnError(t *testing.T) {
	f := newFakeGoogle(t)
	f.freeBusyBody = `{"calendars":{}}`

	_, err := newGoogle(f).FreeBusy(context.Background(), time.Now(), time.Now().Add(time.Hour))
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("got %v, want ErrUnavailable rather than an empty busy list", err)
	}
}

// Same reasoning: a per-calendar error must not read as a free week.
func TestFreeBusyPerCalendarErrorIsAnError(t *testing.T) {
	f := newFakeGoogle(t)
	f.freeBusyBody = `{"calendars":{"primary":{"errors":[{"reason":"notFound"}],"busy":[]}}}`

	_, err := newGoogle(f).FreeBusy(context.Background(), time.Now(), time.Now().Add(time.Hour))
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("got %v, want ErrUnavailable", err)
	}
}

// A revoked refresh token needs the owner to reconnect, so it must not
// look like a transient outage that will fix itself.
func TestRevokedCredentialReadsAsNotConnected(t *testing.T) {
	f := newFakeGoogle(t)
	f.tokenStatus = http.StatusBadRequest

	err := newGoogle(f).Healthy(context.Background())
	if !errors.Is(err, ErrNotConnected) {
		t.Fatalf("got %v, want ErrNotConnected", err)
	}
}

func TestNoRefreshTokenIsNotConnected(t *testing.T) {
	f := newFakeGoogle(t)
	g := newGoogle(f)
	g.RefreshToken = ""
	if err := g.Healthy(context.Background()); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("got %v, want ErrNotConnected", err)
	}
	if f.tokenCalls != 0 {
		t.Error("asked Google for a token with no refresh token to offer")
	}
}

// The access token is cached, or every page load would mint one.
func TestAccessTokenIsReused(t *testing.T) {
	f := newFakeGoogle(t)
	f.freeBusyBody = `{"calendars":{"primary":{"busy":[]}}}`
	g := newGoogle(f)

	for i := 0; i < 3; i++ {
		if _, err := g.FreeBusy(context.Background(), time.Now(), time.Now().Add(time.Hour)); err != nil {
			t.Fatalf("free/busy: %v", err)
		}
	}
	if f.tokenCalls != 1 {
		t.Errorf("minted %d tokens for 3 calls, want 1", f.tokenCalls)
	}
}

// A token that passes the expiry check and then expires mid-request
// produces a 401 on an operation that may already have had an effect,
// so it is refreshed early.
func TestTokenIsRefreshedBeforeItExpires(t *testing.T) {
	f := newFakeGoogle(t)
	f.freeBusyBody = `{"calendars":{"primary":{"busy":[]}}}`
	f.expiresIn = 90 // inside the 60-second margin after 31 seconds

	now := time.Now()
	g := newGoogle(f)
	g.Now = func() time.Time { return now }

	if _, err := g.FreeBusy(context.Background(), now, now.Add(time.Hour)); err != nil {
		t.Fatalf("first: %v", err)
	}
	now = now.Add(31 * time.Second) // 59 seconds of life left
	if _, err := g.FreeBusy(context.Background(), now, now.Add(time.Hour)); err != nil {
		t.Fatalf("second: %v", err)
	}
	if f.tokenCalls != 2 {
		t.Errorf("minted %d tokens, want 2: the margin did not trigger a refresh", f.tokenCalls)
	}
}

func TestCreateSendsTheInvitationAndReturnsTheID(t *testing.T) {
	f := newFakeGoogle(t)
	g := newGoogle(f)

	id, err := g.Create(context.Background(), Event{
		Start:    time.Date(2026, 10, 6, 13, 0, 0, 0, time.UTC),
		End:      time.Date(2026, 10, 6, 13, 30, 0, 0, time.UTC),
		Summary:  "Meeting with Roger Henley",
		Attendee: "dana@example.com",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id != "event-123" {
		t.Errorf("id is %q", id)
	}
	// Without sendUpdates the member never hears about the meeting they
	// just booked.
	if !strings.Contains(f.lastQuery, "sendUpdates=all") {
		t.Errorf("query was %q, want sendUpdates=all", f.lastQuery)
	}
	att, _ := f.lastEventBody["attendees"].([]any)
	if len(att) != 1 {
		t.Fatalf("attendees were %v", f.lastEventBody["attendees"])
	}
}

// An event already gone is the state the caller wanted, so cancelling
// twice must not fail the second time.
func TestCancelIsIdempotent(t *testing.T) {
	f := newFakeGoogle(t)
	f.eventsStatus = http.StatusNotFound

	if err := newGoogle(f).Cancel(context.Background(), "event-123"); err != nil {
		t.Errorf("cancelling an event that is already gone failed: %v", err)
	}
}

func TestCancelWithNoIDDoesNothing(t *testing.T) {
	f := newFakeGoogle(t)
	if err := newGoogle(f).Cancel(context.Background(), ""); err != nil {
		t.Errorf("got %v", err)
	}
	if f.lastMethod != "" {
		t.Error("called Google with no event id to cancel")
	}
}

// Every call reports its outcome so the caller can record connection
// health without this type needing a database.
func TestOutcomesAreReported(t *testing.T) {
	f := newFakeGoogle(t)
	f.freeBusyBody = `{"calendars":{"primary":{"busy":[]}}}`

	var got []error
	g := newGoogle(f)
	g.OnResult = func(err error) { got = append(got, err) }

	if _, err := g.FreeBusy(context.Background(), time.Now(), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("free/busy: %v", err)
	}
	if len(got) != 1 || got[0] != nil {
		t.Errorf("reported %v, want one nil", got)
	}
}

// The consent URL has to ask for offline access with a forced consent,
// or Google returns a refresh token only on the very first
// authorisation and a reconnect silently yields none.
func TestAuthURLAsksForARefreshToken(t *testing.T) {
	u := GoogleAuthURL("client-id", "https://example.com/cb", "state-123")
	for _, want := range []string{
		"access_type=offline", "prompt=consent", "state=state-123",
		"response_type=code", "client_id=client-id",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("missing %q in %s", want, u)
		}
	}
	// Reading every event is a permission this application deliberately
	// does not hold.
	if strings.Contains(u, "calendar.readonly") {
		t.Error("asks for calendar.readonly, which would grant reading every event")
	}
}

// An authorisation that returns no refresh token works for an hour and
// then stops, which is worse than failing now.
func TestExchangeWithoutARefreshTokenIsRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"a","scope":"s"}`))
	}))
	defer srv.Close()
	old := googleTokenURL
	googleTokenURL = srv.URL
	defer func() { googleTokenURL = old }()

	_, err := ExchangeGoogleCode(context.Background(), srv.Client(), "id", "secret", "code", "https://example.com/cb")
	if !errors.Is(err, ErrNoRefreshToken) {
		t.Fatalf("got %v, want ErrNoRefreshToken", err)
	}
}

func TestExchangeReturnsTheRefreshTokenAndAccount(t *testing.T) {
	// An unsigned id_token payload carrying an email claim, which is
	// all this reads and only ever to label the connection.
	payload := `{"email":"roger@example.com"}`
	idToken := "header." + strings.TrimRight(
		base64URL(payload), "=") + ".sig"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "a", "refresh_token": "r",
			"scope": GoogleScopes, "id_token": idToken,
		})
	}))
	defer srv.Close()
	old := googleTokenURL
	googleTokenURL = srv.URL
	defer func() { googleTokenURL = old }()

	got, err := ExchangeGoogleCode(context.Background(), srv.Client(), "id", "secret", "code", "https://example.com/cb")
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if got.RefreshToken != "r" {
		t.Errorf("refresh token is %q", got.RefreshToken)
	}
	if got.Email != "roger@example.com" {
		t.Errorf("email is %q", got.Email)
	}
}

// base64URL encodes a JWT payload segment for the test above.
func base64URL(s string) string {
	return base64.URLEncoding.EncodeToString([]byte(s))
}
