package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/events"
	"github.com/reh3376/career-site/services/api/internal/ratelimit"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// AnonCookieName is the first-party anonymous id the web proxy sets on
// first visit (13 months). The api reads it, never sets it.
const AnonCookieName = "career_anon"

// Events accepts the browser beacon's batches.
type Events struct {
	careerv1connectUnimplementedEvents

	log     *slog.Logger
	writer  *events.Writer
	auth    *Auth
	limiter *ratelimit.Limiter
}

// careerv1connectUnimplementedEvents is a local alias so this file
// compiles before the generated code lands in editors; the real embed
// is in events_embed.go.
type careerv1connectUnimplementedEvents = eventsUnimplemented

// NewEvents wires the handler. auth may be nil (anonymous only).
func NewEvents(log *slog.Logger, writer *events.Writer, auth *Auth) *Events {
	return &Events{
		log: log, writer: writer, auth: auth,
		// 120 events per minute per client address, refilled continuously.
		limiter: ratelimit.New(120, 2),
	}
}

// Record stores a batch. Identity and address come from the request,
// never from the payload; unknown names are dropped silently so an old
// beacon build cannot fail loudly on a new server.
func (h *Events) Record(
	ctx context.Context,
	req *connect.Request[v1.RecordRequest],
) (*connect.Response[v1.RecordResponse], error) {
	if h.writer == nil {
		return connect.NewResponse(&v1.RecordResponse{}), nil
	}
	addr := ClientIP(req)
	if ok, _ := h.limiter.Allow("events:" + addr); !ok {
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("too many events"))
	}
	var member *users.User
	if h.auth != nil {
		if u, err := h.auth.LookupSessionUser(ctx, req); err == nil && u != nil {
			member = u
		}
	}
	anon := cookieValue(req.Header().Get("Cookie"), AnonCookieName)
	device := events.DeviceClass(req.Header().Get("User-Agent"))

	var accepted int32
	for _, be := range req.Msg.GetEvents() {
		name := strings.TrimSpace(be.GetName())
		spec, ok := events.Registry[name]
		if !ok || !spec.Browser {
			continue
		}
		props := map[string]any{}
		if pj := strings.TrimSpace(be.GetPropsJson()); pj != "" {
			_ = json.Unmarshal([]byte(pj), &props)
		}
		e := events.Event{
			EventID:    be.GetEventId(),
			Name:       name,
			AnonID:     anon,
			UIMode:     strings.TrimSpace(be.GetUiMode()),
			Path:       cleanPath(be.GetPath()),
			Referrer:   be.GetReferrer(),
			Device:     device,
			ClientAddr: addr,
			Props:      props,
		}
		if member != nil {
			e.UserID = member.ID
		}
		if ms := be.GetClientTsMs(); ms > 0 {
			t := time.UnixMilli(ms).UTC()
			e.ClientTS = &t
		}
		if q := utmFromPath(be.GetPath()); len(q) > 0 {
			e.UTM = q
		}
		inserted, err := h.writer.Record(ctx, e)
		if err != nil {
			h.log.Warn("events: record failed", slog.String("name", name), slog.String("error", err.Error()))
			continue
		}
		if inserted {
			accepted++
		}
	}
	return connect.NewResponse(&v1.RecordResponse{Accepted: accepted}), nil
}

// cookieValue reads one cookie from a raw Cookie header.
func cookieValue(header, name string) string {
	if header == "" {
		return ""
	}
	r := http.Request{Header: http.Header{"Cookie": {header}}}
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

// cleanPath keeps the path only; the query is parsed for utm and
// otherwise dropped, so nothing personal in a query string is stored.
func cleanPath(p string) string {
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	if p == "" || !strings.HasPrefix(p, "/") {
		return ""
	}
	return p
}

func utmFromPath(p string) map[string]string {
	i := strings.Index(p, "?")
	if i < 0 {
		return nil
	}
	q, err := url.ParseQuery(strings.TrimPrefix(p[i:], "?"))
	if err != nil {
		return nil
	}
	return events.UTMFromQuery(q)
}
