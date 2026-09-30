package scheduling

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/reh3376/career-site/services/api/internal/calendar"
	"github.com/reh3376/career-site/services/api/internal/secrets"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// SealContext binds the stored refresh token to its purpose. A value
// moved into that column from anywhere else fails to open.
const SealContext = "google_refresh_token"

// Provider resolves the live calendar from what is stored, so the rest
// of the application can hold one thing for the life of the process and
// still see a connection that was made, revoked or re-made while it was
// running.
//
// It is itself a calendar.Provider, so callers never learn whether a
// calendar is connected: they call FreeBusy and get an answer or
// ErrNotConnected, which is the same shape either way.
type Provider struct {
	log    *slog.Logger
	users  *users.Repo
	sealer *secrets.Sealer

	clientID     string
	clientSecret string

	// The built client is cached because constructing it re-reads the
	// database and re-opens the sealed token, and the credential changes
	// only when the owner reconnects. The cache also preserves the
	// in-memory access token, which would otherwise be thrown away on
	// every call and minted afresh.
	mu       sync.Mutex
	cached   calendar.Provider
	loadedAt time.Time
	// sealedAt identifies which credential the cache was built from, so
	// a reconnect invalidates it without needing a signal.
	sealedAt string
}

const providerCacheTTL = 60 * time.Second

// NewProvider wires the resolver. Empty client credentials or no
// sealing key are not an error: booking is simply off, and the pages
// say so.
func NewProvider(log *slog.Logger, repo *users.Repo, sealer *secrets.Sealer, clientID, clientSecret string) *Provider {
	return &Provider{
		log: log, users: repo, sealer: sealer,
		clientID: clientID, clientSecret: clientSecret,
	}
}

var _ calendar.Provider = (*Provider)(nil)

// Configured reports whether this deployment could connect a calendar
// at all, which is a different question from whether one is connected.
// The admin surface needs both: one is "set the environment
// variables", the other is "press connect".
func (p *Provider) Configured() bool {
	return p != nil && p.clientID != "" && p.clientSecret != "" && p.sealer.Enabled()
}

// resolve returns the live client, building it if needed.
func (p *Provider) resolve(ctx context.Context) (calendar.Provider, error) {
	if !p.Configured() {
		return nil, calendar.ErrNotConnected
	}
	conn, ok, err := p.users.GetCalendarConnection(ctx)
	if err != nil {
		return nil, err
	}
	if !ok || !conn.Connected() {
		return nil, calendar.ErrNotConnected
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	// Same credential and still fresh: reuse, which keeps the access
	// token alive too.
	if p.cached != nil && p.sealedAt == conn.RefreshTokenSealed &&
		time.Since(p.loadedAt) < providerCacheTTL {
		return p.cached, nil
	}

	token, err := p.sealer.Open(conn.RefreshTokenSealed, SealContext)
	if err != nil {
		// A stored token that will not open is a configuration problem
		// (usually a rotated key), not a transient one, and retrying it
		// forever would hide that.
		p.log.Error("calendar: the stored credential could not be opened",
			slog.String("error", err.Error()))
		return nil, calendar.ErrNotConnected
	}

	g := &calendar.Google{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		RefreshToken: string(token),
		CalendarID:   conn.CalendarID,
	}
	// Health is recorded from here rather than by every caller, so the
	// admin surface can say "worked until Tuesday" without the booking
	// path knowing anything about a database.
	g.OnResult = func(callErr error) { p.record(callErr) }

	p.cached, p.sealedAt, p.loadedAt = g, conn.RefreshTokenSealed, time.Now()
	return g, nil
}

// record stores the outcome of a Google call. Best-effort and detached
// from the request: failing to write health must never fail a booking
// that worked.
func (p *Provider) record(callErr error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), 5*time.Second)
	defer cancel()
	var err error
	if callErr == nil {
		err = p.users.MarkCalendarOK(ctx)
	} else {
		err = p.users.MarkCalendarError(ctx, callErr.Error())
	}
	if err != nil {
		p.log.Warn("calendar: could not record connection health", slog.String("error", err.Error()))
	}
}

// Invalidate drops the cached client, so a reconnect or disconnect
// takes effect immediately rather than within the TTL.
func (p *Provider) Invalidate() {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.cached, p.sealedAt, p.loadedAt = nil, "", time.Time{}
	p.mu.Unlock()
}

func (p *Provider) FreeBusy(ctx context.Context, from, to time.Time) ([]calendar.Interval, error) {
	c, err := p.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return c.FreeBusy(ctx, from, to)
}

func (p *Provider) Create(ctx context.Context, e calendar.Event) (string, error) {
	c, err := p.resolve(ctx)
	if err != nil {
		return "", err
	}
	return c.Create(ctx, e)
}

func (p *Provider) Cancel(ctx context.Context, id string) error {
	c, err := p.resolve(ctx)
	if err != nil {
		return err
	}
	return c.Cancel(ctx, id)
}

func (p *Provider) Healthy(ctx context.Context) error {
	c, err := p.resolve(ctx)
	if err != nil {
		return err
	}
	return c.Healthy(ctx)
}
