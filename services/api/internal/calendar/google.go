package calendar

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Google is a Provider backed by the Google Calendar API.
//
// Two endpoints and nothing else: freebusy.query to find out when the
// owner is busy, and events insert/delete for the meetings this
// application creates. It never lists events, so the titles and
// attendees of the owner's private appointments never enter this
// process. That is a decision recorded in FR-CNT-26 and it is why the
// scope asked for is calendar.events rather than full calendar access.
//
// Tokens: the refresh token is held by the caller (sealed, in the
// database) and handed over through TokenSource. The access token lives
// here, in memory, for about an hour, and is never persisted: it is
// cheap to mint and a copy on disk is a credential to protect for no
// benefit.
type Google struct {
	// ClientID and ClientSecret identify this application to Google.
	ClientID     string
	ClientSecret string
	// RefreshToken is the owner's, already decrypted by the caller.
	RefreshToken string
	// CalendarID is the calendar to write to; "primary" is the
	// account's own.
	CalendarID string
	// HTTP is injectable so tests do not reach the network.
	HTTP *http.Client
	// Now is injectable so token expiry can be tested.
	Now func() time.Time
	// OnResult, when set, is called with the outcome of every Google
	// call, so the caller can record connection health without this
	// type needing a database.
	OnResult func(err error)

	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
}

// Google's endpoints. Variables rather than constants so a test can
// point them at a local server.
var (
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleFreeBusyURL = "https://www.googleapis.com/calendar/v3/freeBusy"
	googleEventsURL   = "https://www.googleapis.com/calendar/v3/calendars/%s/events"
)

// GoogleScopes is what the consent screen asks for.
//
// calendar.events is write access to events and, with freebusy, enough
// to do everything here. calendar.readonly would also work for
// free/busy but would grant the ability to read every event, which this
// application deliberately does not want: a permission not held cannot
// be misused by a later change.
const GoogleScopes = "https://www.googleapis.com/auth/calendar.events " +
	"https://www.googleapis.com/auth/calendar.freebusy"

var _ Provider = (*Google)(nil)

func (g *Google) httpClient() *http.Client {
	if g.HTTP != nil {
		return g.HTTP
	}
	return &http.Client{Timeout: 20 * time.Second}
}

func (g *Google) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now()
}

func (g *Google) calendarID() string {
	if g.CalendarID == "" {
		return "primary"
	}
	return g.CalendarID
}

// report hands the outcome to the caller, if it asked.
func (g *Google) report(err error) error {
	if g.OnResult != nil {
		g.OnResult(err)
	}
	return err
}

// token returns a valid access token, minting one if the cached one is
// missing or close to expiry.
//
// The 60-second margin is not decoration: a token that passes the
// expiry check and then expires mid-request produces a 401 on an
// operation that may already have had an effect, which for a booking is
// the worst possible moment.
func (g *Google) token(ctx context.Context) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.accessToken != "" && g.now().Add(60*time.Second).Before(g.expiresAt) {
		return g.accessToken, nil
	}
	if g.RefreshToken == "" {
		return "", ErrNotConnected
	}

	form := url.Values{
		"client_id":     {g.ClientID},
		"client_secret": {g.ClientSecret},
		"refresh_token": {g.RefreshToken},
		"grant_type":    {"refresh_token"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build the token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := g.httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: reaching Google for a token: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		// A revoked or expired refresh token is not a transient
		// failure, and must not be retried as if it were: it needs the
		// owner to reconnect. Google says so with 400 invalid_grant.
		if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized {
			return "", fmt.Errorf("%w: Google rejected the stored credential (%s)",
				ErrNotConnected, strings.TrimSpace(string(body)))
		}
		return "", fmt.Errorf("%w: token endpoint returned %d", ErrUnavailable, resp.StatusCode)
	}

	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.AccessToken == "" {
		return "", fmt.Errorf("%w: the token response could not be read", ErrUnavailable)
	}
	g.accessToken = out.AccessToken
	g.expiresAt = g.now().Add(time.Duration(out.ExpiresIn) * time.Second)
	return g.accessToken, nil
}

// do performs an authenticated request and returns the body.
func (g *Google) do(ctx context.Context, method, endpoint string, payload any) ([]byte, error) {
	tok, err := g.token(ctx)
	if err != nil {
		return nil, err
	}
	var body io.Reader
	if payload != nil {
		buf, mErr := json.Marshal(payload)
		if mErr != nil {
			return nil, fmt.Errorf("encode the request: %w", mErr)
		}
		body = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("build the request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := g.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: reaching Google: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))

	switch {
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("%w: Google refused the request (%d)", ErrNotConnected, resp.StatusCode)
	case resp.StatusCode >= 400:
		return nil, fmt.Errorf("%w: Google returned %d: %s",
			ErrUnavailable, resp.StatusCode, strings.TrimSpace(string(out)))
	}
	return out, nil
}

// FreeBusy returns the owner's busy intervals. Intervals only: the
// endpoint is chosen precisely because it cannot return anything else.
func (g *Google) FreeBusy(ctx context.Context, from, to time.Time) ([]Interval, error) {
	payload := map[string]any{
		"timeMin": from.UTC().Format(time.RFC3339),
		"timeMax": to.UTC().Format(time.RFC3339),
		"items":   []map[string]string{{"id": g.calendarID()}},
	}
	body, err := g.do(ctx, http.MethodPost, googleFreeBusyURL, payload)
	if err != nil {
		return nil, g.report(err)
	}

	var out struct {
		Calendars map[string]struct {
			Busy []struct {
				Start string `json:"start"`
				End   string `json:"end"`
			} `json:"busy"`
			Errors []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
		} `json:"calendars"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, g.report(fmt.Errorf("%w: the free/busy response could not be read", ErrUnavailable))
	}

	cal, ok := out.Calendars[g.calendarID()]
	if !ok {
		// Google answers per calendar; a missing entry means it did not
		// answer for the one asked about. Treating that as "nothing is
		// busy" would offer the owner's whole week.
		return nil, g.report(fmt.Errorf("%w: no free/busy answer for %q", ErrUnavailable, g.calendarID()))
	}
	if len(cal.Errors) > 0 {
		return nil, g.report(fmt.Errorf("%w: free/busy error: %s", ErrUnavailable, cal.Errors[0].Reason))
	}

	busy := make([]Interval, 0, len(cal.Busy))
	for _, b := range cal.Busy {
		start, err1 := time.Parse(time.RFC3339, b.Start)
		end, err2 := time.Parse(time.RFC3339, b.End)
		if err1 != nil || err2 != nil {
			return nil, g.report(fmt.Errorf("%w: unreadable busy interval", ErrUnavailable))
		}
		busy = append(busy, Interval{Start: start, End: end})
	}
	_ = g.report(nil)
	return busy, nil
}

// Create puts a meeting on the calendar and returns its id.
func (g *Google) Create(ctx context.Context, e Event) (string, error) {
	body := map[string]any{
		"summary":     e.Summary,
		"description": e.Description,
		"start":       map[string]string{"dateTime": e.Start.UTC().Format(time.RFC3339)},
		"end":         map[string]string{"dateTime": e.End.UTC().Format(time.RFC3339)},
	}
	// The owner is the calendar's owner and is never listed on his own
	// event; the attendee is the member who booked, so they receive the
	// invitation rather than only seeing it on this site.
	if e.Attendee != "" {
		body["attendees"] = []map[string]any{{"email": e.Attendee}}
	}

	endpoint := fmt.Sprintf(googleEventsURL, url.PathEscape(g.calendarID()))
	// sendUpdates=all so the member actually receives the invitation
	// rather than the event appearing only on the owner's calendar.
	out, err := g.do(ctx, http.MethodPost, endpoint+"?sendUpdates=all", body)
	if err != nil {
		return "", g.report(err)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out, &created); err != nil || created.ID == "" {
		return "", g.report(fmt.Errorf("%w: the created event had no id", ErrUnavailable))
	}
	_ = g.report(nil)
	return created.ID, nil
}

// Cancel removes a meeting this application created.
func (g *Google) Cancel(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	endpoint := fmt.Sprintf(googleEventsURL, url.PathEscape(g.calendarID())) +
		"/" + url.PathEscape(id) + "?sendUpdates=all"
	_, err := g.do(ctx, http.MethodDelete, endpoint, nil)
	// An event already gone is the state the caller wanted. Google
	// answers 404 or 410; both mean there is nothing left to cancel.
	if err != nil && (strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "410")) {
		return nil
	}
	return g.report(err)
}

// Healthy reports whether the provider can be used right now.
//
// It mints a token rather than making a calendar call: that exercises
// the credential, which is the thing that actually expires or gets
// revoked, without asking Google about the owner's week every time a
// page loads.
func (g *Google) Healthy(ctx context.Context) error {
	if g.RefreshToken == "" {
		return ErrNotConnected
	}
	_, err := g.token(ctx)
	return g.report(err)
}

// GoogleAuthURL builds the consent URL the owner visits to connect.
//
// access_type=offline with prompt=consent is what makes Google return a
// refresh token. Without prompt=consent it returns one only on the very
// first authorisation, so reconnecting after a revoke would silently
// yield no refresh token and the connection would work until the first
// access token expired and then stop.
func GoogleAuthURL(clientID, redirectURI, state string) string {
	q := url.Values{
		"client_id":              {clientID},
		"redirect_uri":           {redirectURI},
		"response_type":          {"code"},
		"scope":                  {GoogleScopes},
		"access_type":            {"offline"},
		"prompt":                 {"consent"},
		"state":                  {state},
		"include_granted_scopes": {"true"},
	}
	return "https://accounts.google.com/o/oauth2/v2/auth?" + q.Encode()
}

// GoogleExchange trades an authorisation code for a refresh token.
type GoogleExchange struct {
	RefreshToken string
	AccessToken  string
	Scopes       string
	Email        string
}

// ErrNoRefreshToken means Google authorised but returned no refresh
// token, which makes the connection useless within the hour.
var ErrNoRefreshToken = errors.New("Google returned no refresh token; disconnect the app at myaccount.google.com and connect again")

// ExchangeGoogleCode completes the OAuth handshake.
func ExchangeGoogleCode(ctx context.Context, client *http.Client, clientID, clientSecret, code, redirectURI string) (GoogleExchange, error) {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	form := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirectURI},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return GoogleExchange{}, fmt.Errorf("build the exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return GoogleExchange{}, fmt.Errorf("%w: reaching Google: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return GoogleExchange{}, fmt.Errorf("Google refused the authorisation code (%d): %s",
			resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Scope        string `json:"scope"`
		IDToken      string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return GoogleExchange{}, fmt.Errorf("the exchange response could not be read: %w", err)
	}
	if out.RefreshToken == "" {
		return GoogleExchange{}, ErrNoRefreshToken
	}
	return GoogleExchange{
		RefreshToken: out.RefreshToken,
		AccessToken:  out.AccessToken,
		Scopes:       out.Scope,
		Email:        emailFromIDToken(out.IDToken),
	}, nil
}

// emailFromIDToken reads the email claim without verifying the
// signature.
//
// Safe here and nowhere else: this token arrived over TLS directly from
// Google's token endpoint in response to our own request, so it is not
// attacker-supplied, and the value is used only to show the owner which
// account he connected. It is never an authorisation decision. Returns
// empty rather than failing, because a missing label must not stop a
// working connection being saved.
func emailFromIDToken(idToken string) string {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64RawURLDecode(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return claims.Email
}

// base64RawURLDecode decodes a JWT segment, tolerating the padding some
// encoders add even though the spec says raw.
func base64RawURLDecode(s string) ([]byte, error) {
	if m := len(s) % 4; m != 0 {
		s += strings.Repeat("=", 4-m)
	}
	return base64.URLEncoding.DecodeString(s)
}
