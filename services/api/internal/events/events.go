// Package events is the product event stream: one append-only table,
// one registry of names, one writer used by both the browser beacon
// (through handlers.Events) and the api's own code paths. See
// docs/events/README.md for the registry and the reasoning.
package events

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/reh3376/career-site/services/api/internal/build"
	"github.com/reh3376/career-site/services/api/internal/tenant"
)

// Registry lists every event name the stream accepts and the prop
// keys each may carry. Anything else is dropped at the edge, so the
// table never accumulates ad-hoc shapes. Browser-only names are the
// ones a beacon may send; the rest are written by the api itself.
var Registry = map[string]struct {
	Browser bool
	Props   []string
}{
	// Landing and navigation
	"page.view":           {Browser: true, Props: []string{"title"}},
	"page.leave":          {Browser: true, Props: []string{"dwell_ms"}},
	"landing.mode_switch": {Browser: true, Props: []string{"from", "to"}},
	"landing.cta_click":   {Browser: true, Props: []string{"cta", "href"}},
	"link.external":       {Browser: true, Props: []string{"host"}},
	// Registration and session (server-side)
	"register.submit":        {Props: []string{"user_id"}},
	"verify.success":         {Props: []string{"user_id"}},
	"verify.already_used":    {},
	"approval.requested":     {Props: []string{"user_id"}},
	"approval.decided":       {Props: []string{"user_id", "decision", "channel"}},
	"approval.auto_declined": {Props: []string{"user_id"}},
	"login.success":          {Props: []string{"user_id"}},
	"login.failed":           {Props: []string{"reason"}},
	"logout":                 {},
	"access.expired":         {Props: []string{"user_id"}},
	// Content
	"article.view":       {Browser: true, Props: []string{"slug"}},
	"gallery.view":       {Browser: true, Props: []string{}},
	"gallery.photo_open": {Browser: true, Props: []string{"id"}},
	"repo.click":         {Browser: true, Props: []string{"name"}},
	// JD reviewer (server-side)
	"jd.view":           {Browser: true, Props: []string{}},
	"jd.submitted":      {Props: []string{"submission_id", "chars", "has_apply_url"}},
	"jd.finished":       {Props: []string{"submission_id", "outcome", "score", "threshold", "fit"}},
	"jd.result_viewed":  {Browser: true, Props: []string{"submission_id", "via"}},
	"jd.pdf_downloaded": {Props: []string{"submission_id"}},
	// Ground truth and judgment (data layer D3).
	"jd.outcome_recorded":  {Props: []string{"submission_id", "status"}},
	"jd.feedback_recorded": {Props: []string{"submission_id", "target", "rating"}},
	"jd.poll_abandoned":    {Browser: true, Props: []string{"submission_id", "waited_ms"}},
	// Contact
	"contact.submitted": {Props: []string{"category", "has_jd"}},
	// Admin
	"admin.decision_reviewed": {Props: []string{"decision_id", "verdict"}},
	"admin.rescore":           {Props: []string{"submission_id"}},
	"admin.fit_bands_changed": {},
	// Backfill from activity_events keeps whatever kind it had.
	"activity.download": {}, "activity.search": {}, "activity.save": {},
	"activity.chat": {}, "activity.escalate": {}, "activity.logout": {},
}

// Event is one row to write. Identity fields are filled by the
// writer from the request context, never by the caller.
type Event struct {
	EventID    string // empty: minted here
	Name       string
	ClientTS   *time.Time
	AnonID     string
	UserID     int64
	SessionID  int64
	UIMode     string
	Path       string
	Referrer   string
	UTM        map[string]string
	Device     string
	ClientAddr string
	Props      map[string]any
}

// Writer inserts events. Salt keys the client-address hash so the
// stored value is not a rainbow-table lookup of visitor addresses.
type Writer struct {
	pool *pgxpool.Pool
	log  *slog.Logger
	salt string
}

// New makes a writer. An empty salt falls back to a fixed dev salt and
// logs it once, so prod never runs unsalted by accident without notice.
func New(pool *pgxpool.Pool, log *slog.Logger, salt string) *Writer {
	if salt == "" {
		log.Warn("events: EVENT_IP_SALT is empty; using the development salt")
		salt = "career-site-dev-salt"
	}
	return &Writer{pool: pool, log: log, salt: salt}
}

// ErrUnknownEvent is returned for names outside the registry.
var ErrUnknownEvent = errors.New("unknown event name")

// Record stores one event. Unknown names and props are rejected here
// so callers get an error in dev; the beacon handler drops silently.
// The bool is false when the event id was already stored (a retried
// batch), which is not an error.
func (w *Writer) Record(ctx context.Context, e Event) (bool, error) {
	if w == nil {
		return false, nil
	}
	spec, ok := Registry[e.Name]
	if !ok {
		return false, ErrUnknownEvent
	}
	props := map[string]any{}
	for _, k := range spec.Props {
		if v, ok := e.Props[k]; ok {
			props[k] = v
		}
	}
	raw, _ := json.Marshal(props)
	if len(raw) > 2048 {
		return false, errors.New("props too large")
	}
	id := strings.ToLower(strings.TrimSpace(e.EventID))
	if id == "" {
		id = NewUUID()
	} else if !IsUUID(id) {
		// Client ids that are not UUIDs are hashed into one so the
		// column stays a uuid and retries still de-duplicate.
		sum := sha256.Sum256([]byte(id))
		id = uuidFromBytes(sum[:16])
	}
	var anon any
	if a := strings.ToLower(strings.TrimSpace(e.AnonID)); IsUUID(a) {
		anon = a
	}
	var utm any
	if len(e.UTM) > 0 {
		b, _ := json.Marshal(e.UTM)
		utm = string(b)
	}
	tag, err := w.pool.Exec(ctx, `
    INSERT INTO events
      (tenant_id, event_id, name, occurred_at, client_ts, anon_id, user_id, session_id, ui_mode,
       path, referrer_host, utm, device, ip_hash, app_commit, props)
    VALUES ($1, $2, $3, now(), $4, $5, NULLIF($6, 0), NULLIF($7, 0), NULLIF($8, ''),
            NULLIF($9, ''), NULLIF($10, ''), $11, NULLIF($12, ''), NULLIF($13, ''), $14, $15)
    ON CONFLICT (event_id) DO NOTHING`,
		tenant.FromContext(ctx).Int64(),
		id, e.Name, e.ClientTS, anon, e.UserID, e.SessionID, e.UIMode,
		truncate(e.Path, 512), referrerHost(e.Referrer), utm, e.Device, w.hashAddr(e.ClientAddr), build.Commit, string(raw))
	if err != nil {
		return false, fmt.Errorf("insert event: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// Emit is Record for the api's own code paths: it logs a failure and
// never blocks the caller.
func (w *Writer) Emit(ctx context.Context, e Event) {
	if w == nil {
		return
	}
	if _, err := w.Record(context.WithoutCancel(ctx), e); err != nil {
		w.log.Warn("events: emit failed", slog.String("name", e.Name), slog.String("error", err.Error()))
	}
}

// AnonymizeOlderThan drops identity from old rows (retention rule in
// docs/events/README.md): the row stays for aggregate counts.
func (w *Writer) AnonymizeOlderThan(ctx context.Context, age time.Duration) (int64, error) {
	tag, err := w.pool.Exec(ctx, `
    UPDATE events SET user_id = NULL, session_id = NULL, ip_hash = NULL, anon_id = NULL
     WHERE occurred_at < now() - $1::interval
       AND (user_id IS NOT NULL OR session_id IS NOT NULL OR ip_hash IS NOT NULL OR anon_id IS NOT NULL)`,
		fmt.Sprintf("%d seconds", int64(age.Seconds())))
	if err != nil {
		return 0, fmt.Errorf("anonymize events: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (w *Writer) hashAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	// Strip a port if one rode along.
	if i := strings.LastIndex(addr, ":"); i > 0 && strings.Count(addr, ":") == 1 {
		addr = addr[:i]
	}
	sum := sha256.Sum256([]byte(w.salt + "|" + addr))
	return hex.EncodeToString(sum[:8])
}

func referrerHost(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil || u.Host == "" {
		return truncate(ref, 100)
	}
	return truncate(u.Host, 253)
}

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// IsUUID reports whether s is a lower-case hyphenated UUID.
func IsUUID(s string) bool { return uuidRe.MatchString(s) }

// NewUUID mints a random (v4) UUID without pulling in a module for it.
func NewUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("events: crypto/rand unavailable: " + err.Error())
	}
	return uuidFromBytes(b[:])
}

func uuidFromBytes(b []byte) string {
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:16])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// DeviceClass is a coarse device bucket from the user agent, enough
// for a funnel by form factor and nothing more.
func DeviceClass(ua string) string {
	u := strings.ToLower(ua)
	switch {
	case u == "":
		return ""
	case strings.Contains(u, "ipad") || (strings.Contains(u, "android") && !strings.Contains(u, "mobile")) || strings.Contains(u, "tablet"):
		return "tablet"
	case strings.Contains(u, "mobi") || strings.Contains(u, "iphone") || strings.Contains(u, "android"):
		return "phone"
	default:
		return "desktop"
	}
}

// UTMFromQuery keeps the five utm_* parameters, truncated.
func UTMFromQuery(q url.Values) map[string]string {
	out := map[string]string{}
	for _, k := range []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content"} {
		if v := strings.TrimSpace(q.Get(k)); v != "" {
			out[k] = truncate(v, 100)
		}
	}
	return out
}
