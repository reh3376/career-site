package handlers

import (
	"net/http"
	"strings"

	"connectrpc.com/connect"

	"github.com/reh3376/career-site/services/api/internal/calendar"
	"github.com/reh3376/career-site/services/api/internal/events"
	"github.com/reh3376/career-site/services/api/internal/jd"
	"github.com/reh3376/career-site/services/api/internal/scheduling"
	"github.com/reh3376/career-site/services/api/internal/secrets"
)

// SetEvents installs the product event writer on each handler that has
// server-side emit points (docs/events/README.md). Nil leaves the
// handler silent; every emit is best-effort and never fails a request.
func (h *Auth) SetEvents(w *events.Writer)            { h.events = w }
func (h *AdminDecision) SetEvents(w *events.Writer)   { h.events = w }
func (h *Contact) SetEvents(w *events.Writer)         { h.events = w }
func (h *Jd) SetEvents(w *events.Writer)              { h.events = w }
func (a *Admin) SetEvents(w *events.Writer)           { a.events = w }
func (j *ExpiryJobs) SetEvents(w *events.Writer)      { j.events = w }
func (j *AutoDeclineJobs) SetEvents(w *events.Writer) { j.events = w }

// SetEvaluator installs the golden-set evaluator on the admin handler.
// Nil disables the evaluation job, which is dev without a sidecar.
func (a *Admin) SetEvaluator(e *jd.Evaluator) { a.evaluator = e }

// SetJdLimits installs the submission-limit store on the admin handler,
// so the console edits the same value the JD handler enforces.
func (a *Admin) SetJdLimits(s *jd.LimitStore) { a.jdLimits = s }

// SetScheduler installs the meeting-scheduler settings store and the
// calendar provider on the admin handler, so the console edits the same
// settings the booking flow reads and can say whether a calendar is
// actually connected behind them. Nil leaves both surfaces answering
// Unavailable rather than showing windows nobody can book.
func (a *Admin) SetScheduler(s *scheduling.SettingsStore, cal calendar.Provider) {
	a.scheduler = s
	a.calendar = cal
}

// SetCalendarConnect installs everything the Google connect flow needs.
// Without it the surface reports "not configured", which is a different
// message from "not connected" and needs a different fix.
func (a *Admin) SetCalendarConnect(
	p *scheduling.Provider,
	clientID, clientSecret string,
	sealer *secrets.Sealer,
	stateSecret []byte,
	webBaseURL string,
) {
	a.calProvider = p
	a.googleClientID = clientID
	a.googleClientSecret = clientSecret
	a.sealer = sealer
	a.stateSecret = stateSecret
	a.webBaseURL = webBaseURL
}

// uiModeCookie is the web app's mode cookie (apps/web/src/lib/ui-mode.ts).
const uiModeCookie = "ui_mode"

// requestEvent builds an event from a Connect request: anonymous id
// and UI mode from cookies, device class from the user agent, client
// address for the salted hash. The caller sets the user id it knows.
func requestEvent(req connect.AnyRequest, name string, userID int64, props map[string]any) events.Event {
	cookies := req.Header().Get("Cookie")
	return events.Event{
		Name:       name,
		UserID:     userID,
		AnonID:     cookieValue(cookies, AnonCookieName),
		UIMode:     cookieValue(cookies, uiModeCookie),
		Device:     events.DeviceClass(req.Header().Get("User-Agent")),
		ClientAddr: ClientIP(req),
		Props:      props,
	}
}

// httpEvent is requestEvent for plain net/http handlers.
func httpEvent(r *http.Request, name string, userID int64, props map[string]any) events.Event {
	cookies := r.Header.Get("Cookie")
	return events.Event{
		Name:       name,
		UserID:     userID,
		AnonID:     cookieValue(cookies, AnonCookieName),
		UIMode:     cookieValue(cookies, uiModeCookie),
		Device:     events.DeviceClass(r.UserAgent()),
		ClientAddr: clientIPHTTP(r),
		Props:      props,
	}
}

// clientIPHTTP is ClientIP for plain net/http requests.
func clientIPHTTP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.Index(xff, ","); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}
