package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// CalendarConnection is the owner's link to the calendar this
// application books against (migration 00043). One row by construction.
//
// It holds a credential and health, never anything about what is on the
// calendar: free/busy is read live and never stored.
type CalendarConnection struct {
	Provider     string
	CalendarID   string
	AccountEmail string
	// RefreshTokenSealed is ciphertext, not a token. Open it through
	// internal/secrets with the context "google_refresh_token". Empty
	// means the connection was revoked, which is different from there
	// being no row at all.
	RefreshTokenSealed string
	Scopes             string
	ConnectedAt        time.Time
	ConnectedBy        *int64
	LastOKAt           *time.Time
	LastError          string
	UpdatedAt          time.Time
}

// Connected reports whether there is a usable credential.
func (c CalendarConnection) Connected() bool { return c.RefreshTokenSealed != "" }

// GetCalendarConnection returns the connection, or ok false when the
// owner has never connected one.
func (r *Repo) GetCalendarConnection(ctx context.Context) (CalendarConnection, bool, error) {
	var c CalendarConnection
	err := r.pool.QueryRow(ctx, `
    SELECT provider, calendar_id, account_email, refresh_token_sealed, scopes,
           connected_at, connected_by, last_ok_at, last_error, updated_at
      FROM calendar_connection WHERE id = 1`).Scan(
		&c.Provider, &c.CalendarID, &c.AccountEmail, &c.RefreshTokenSealed, &c.Scopes,
		&c.ConnectedAt, &c.ConnectedBy, &c.LastOKAt, &c.LastError, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return CalendarConnection{}, false, nil
	}
	if err != nil {
		return CalendarConnection{}, false, fmt.Errorf("get calendar connection: %w", err)
	}
	return c, true, nil
}

// SaveCalendarConnection stores a new connection, replacing any
// previous one.
//
// The health columns are reset rather than carried over: a new
// authorisation has not succeeded or failed yet, and keeping the old
// error would show a stale complaint against a credential that has
// never been tried.
func (r *Repo) SaveCalendarConnection(ctx context.Context, c CalendarConnection, by int64) error {
	if c.CalendarID == "" {
		c.CalendarID = "primary"
	}
	if c.Provider == "" {
		c.Provider = "google"
	}
	_, err := r.pool.Exec(ctx, `
    INSERT INTO calendar_connection
      (id, provider, calendar_id, account_email, refresh_token_sealed, scopes,
       connected_at, connected_by, last_ok_at, last_error, updated_at)
    VALUES (1, $1, $2, $3, $4, $5, now(), NULLIF($6, 0), NULL, '', now())
    ON CONFLICT (id) DO UPDATE SET
      provider = EXCLUDED.provider,
      calendar_id = EXCLUDED.calendar_id,
      account_email = EXCLUDED.account_email,
      refresh_token_sealed = EXCLUDED.refresh_token_sealed,
      scopes = EXCLUDED.scopes,
      connected_at = now(),
      connected_by = EXCLUDED.connected_by,
      last_ok_at = NULL,
      last_error = '',
      updated_at = now()`,
		c.Provider, c.CalendarID, c.AccountEmail, c.RefreshTokenSealed, c.Scopes, by)
	if err != nil {
		return fmt.Errorf("save calendar connection: %w", err)
	}
	return nil
}

// MarkCalendarOK records a successful call, clearing any previous
// error. Best-effort by the caller: losing this must never fail the
// operation that succeeded.
func (r *Repo) MarkCalendarOK(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
    UPDATE calendar_connection
       SET last_ok_at = now(), last_error = '', updated_at = now()
     WHERE id = 1`)
	if err != nil {
		return fmt.Errorf("mark calendar ok: %w", err)
	}
	return nil
}

// MarkCalendarError records why the last call failed, leaving
// last_ok_at alone so the admin surface can say "worked until Tuesday"
// rather than only "broken".
func (r *Repo) MarkCalendarError(ctx context.Context, reason string) error {
	if len(reason) > 500 {
		reason = reason[:500]
	}
	_, err := r.pool.Exec(ctx, `
    UPDATE calendar_connection
       SET last_error = $1, updated_at = now()
     WHERE id = 1`, reason)
	if err != nil {
		return fmt.Errorf("mark calendar error: %w", err)
	}
	return nil
}

// DisconnectCalendar clears the credential and keeps the row.
//
// The row survives so the admin surface can still say which account was
// connected and when it stopped, which is what someone asks first after
// bookings start failing. Deleting it would answer "never connected",
// which is a different and wrong story.
func (r *Repo) DisconnectCalendar(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
    UPDATE calendar_connection
       SET refresh_token_sealed = '', last_error = 'disconnected by the owner', updated_at = now()
     WHERE id = 1`)
	if err != nil {
		return fmt.Errorf("disconnect calendar: %w", err)
	}
	return nil
}
