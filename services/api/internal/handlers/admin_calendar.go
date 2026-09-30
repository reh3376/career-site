package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/calendar"
	"github.com/reh3376/career-site/services/api/internal/scheduling"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Connecting the owner's Google Calendar.
//
// The client secret never leaves this process: the browser is sent to
// Google with only the client id, comes back with a one-time code, and
// the code is exchanged here. The refresh token that comes out is
// sealed before it touches the database (migration 00043).

// calendarStateTTL bounds how long a consent round trip may take. Long
// enough for a real sign-in with MFA, short enough that a state lifted
// from a browser history is useless.
const calendarStateTTL = 15 * time.Minute

func (a *Admin) GetCalendarConnectURL(
	ctx context.Context,
	req *connect.Request[v1.GetCalendarConnectURLRequest],
) (*connect.Response[v1.GetCalendarConnectURLResponse], error) {
	admin, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	if a.calProvider == nil || !a.calProvider.Configured() {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("this deployment has no Google client credentials or no sealing key configured"))
	}
	state := a.signCalendarState(admin.ID, time.Now().Add(calendarStateTTL))
	url := calendar.GoogleAuthURL(a.googleClientID, a.calendarRedirectURI(), state)
	return connect.NewResponse(&v1.GetCalendarConnectURLResponse{Url: url}), nil
}

func (a *Admin) ConnectCalendar(
	ctx context.Context,
	req *connect.Request[v1.ConnectCalendarRequest],
) (*connect.Response[v1.ConnectCalendarResponse], error) {
	admin, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	if a.calProvider == nil || !a.calProvider.Configured() {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("this deployment cannot connect a calendar"))
	}
	// Verified before the code is spent, so a code delivered with a
	// state this server never issued is never exchanged.
	if err := a.verifyCalendarState(req.Msg.GetState(), admin.ID); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}

	ex, err := calendar.ExchangeGoogleCode(ctx, nil,
		a.googleClientID, a.googleClientSecret, req.Msg.GetCode(), a.calendarRedirectURI())
	if err != nil {
		if errors.Is(err, calendar.ErrNoRefreshToken) {
			// Worth its own message: the connection would appear to work
			// and then stop within the hour.
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		a.log.Warn("calendar: the authorisation code could not be exchanged",
			"admin", admin.ID, "error", err.Error())
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("Google would not accept that authorisation"))
	}

	sealed, err := a.sealer.Seal([]byte(ex.RefreshToken), scheduling.SealContext)
	if err != nil {
		// Storing it in plaintext is not an option this application
		// offers, so refusing is the right outcome.
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("the credential could not be encrypted: %w", err))
	}
	if err := a.users.SaveCalendarConnection(ctx, users.CalendarConnection{
		Provider:           "google",
		CalendarID:         "primary",
		AccountEmail:       ex.Email,
		RefreshTokenSealed: sealed,
		Scopes:             ex.Scopes,
	}, admin.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("the connection could not be stored"))
	}
	// So the next booking uses the new credential rather than waiting
	// out the cache.
	a.calProvider.Invalidate()
	a.log.Info("calendar connected", "admin", admin.ID, "account", ex.Email)

	return connect.NewResponse(&v1.ConnectCalendarResponse{Status: a.calendarStatus(ctx)}), nil
}

func (a *Admin) GetCalendarStatus(
	ctx context.Context,
	req *connect.Request[v1.GetCalendarStatusRequest],
) (*connect.Response[v1.GetCalendarStatusResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.GetCalendarStatusResponse{Status: a.calendarStatus(ctx)}), nil
}

func (a *Admin) DisconnectCalendar(
	ctx context.Context,
	req *connect.Request[v1.DisconnectCalendarRequest],
) (*connect.Response[v1.DisconnectCalendarResponse], error) {
	admin, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	// The row survives so the surface can still say which account was
	// connected and when it stopped. Existing meetings are left alone,
	// on the calendar and here: disconnecting stops new bookings, it is
	// not a way to cancel what is already agreed.
	if err := a.users.DisconnectCalendar(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("the calendar could not be disconnected"))
	}
	if a.calProvider != nil {
		a.calProvider.Invalidate()
	}
	a.log.Info("calendar disconnected", "admin", admin.ID)
	return connect.NewResponse(&v1.DisconnectCalendarResponse{Status: a.calendarStatus(ctx)}), nil
}

func (a *Admin) calendarStatus(ctx context.Context) *v1.CalendarStatus {
	out := &v1.CalendarStatus{
		Configured: a.calProvider != nil && a.calProvider.Configured(),
	}
	conn, ok, err := a.users.GetCalendarConnection(ctx)
	if err != nil || !ok {
		return out
	}
	out.Connected = conn.Connected()
	out.AccountEmail = conn.AccountEmail
	out.CalendarId = conn.CalendarID
	out.Scopes = conn.Scopes
	out.ConnectedAt = timestamppb.New(conn.ConnectedAt)
	out.LastError = conn.LastError
	if conn.LastOKAt != nil {
		out.LastOkAt = timestamppb.New(*conn.LastOKAt)
	}
	return out
}

// calendarRedirectURI must match the one registered with Google
// exactly, including scheme and any trailing path. It is derived from
// the configured web base so dev and production do not need separate
// constants.
func (a *Admin) calendarRedirectURI() string {
	return strings.TrimRight(a.webBaseURL, "/") + "/admin/scheduler/callback"
}

// signCalendarState builds an opaque, tamper-evident state.
//
// It carries the admin who started the flow and an expiry, so a
// callback can be checked against both: a code delivered under someone
// else's state, or a state from last week, is refused before the code
// is spent. Signed with the same server secret as the decision tokens
// rather than stored, because a value that verifies itself needs no row
// and no cleanup.
func (a *Admin) signCalendarState(adminID int64, expires time.Time) string {
	payload := fmt.Sprintf("v1.%d.%d", adminID, expires.Unix())
	return payload + "." + base64.RawURLEncoding.EncodeToString(a.calendarStateMAC(payload))
}

func (a *Admin) calendarStateMAC(payload string) []byte {
	m := hmac.New(sha256.New, a.stateSecret)
	m.Write([]byte(payload))
	return m.Sum(nil)
}

var errCalendarState = errors.New("that authorisation did not come from this session; start again")

func (a *Admin) verifyCalendarState(state string, adminID int64) error {
	parts := strings.Split(state, ".")
	if len(parts) != 4 || parts[0] != "v1" {
		return errCalendarState
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return errCalendarState
	}
	payload := strings.Join(parts[:3], ".")
	// Constant time, so the signature cannot be discovered by timing.
	if !hmac.Equal(sig, a.calendarStateMAC(payload)) {
		return errCalendarState
	}
	gotAdmin, err1 := strconv.ParseInt(parts[1], 10, 64)
	exp, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		return errCalendarState
	}
	if gotAdmin != adminID {
		return errCalendarState
	}
	if time.Now().After(time.Unix(exp, 0)) {
		return errors.New("that authorisation took too long; start again")
	}
	return nil
}

// The owner's view of what has been booked.
//
// It carries the member's name and email, which the member-facing
// Meeting message deliberately does not: a member has no business
// knowing who else booked, and the owner has every reason to know who
// he is meeting.

func (a *Admin) ListMeetings(
	ctx context.Context,
	req *connect.Request[v1.ListMeetingsRequest],
) (*connect.Response[v1.ListMeetingsResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	rows, err := a.users.AllBookings(ctx, req.Msg.GetIncludePast())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("the meetings could not be read"))
	}
	out := &v1.ListMeetingsResponse{Zone: a.schedulerZone(ctx)}
	for _, r := range rows {
		out.Meetings = append(out.Meetings, adminMeetingToProto(r))
	}
	return connect.NewResponse(out), nil
}

func (a *Admin) CancelMeetingAsAdmin(
	ctx context.Context,
	req *connect.Request[v1.CancelMeetingAsAdminRequest],
) (*connect.Response[v1.CancelMeetingAsAdminResponse], error) {
	admin, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	existing, ok, err := a.users.BookingByID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("the meeting could not be read"))
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such meeting"))
	}
	// Cancelling something already cancelled succeeds, so a double
	// click or a stale tab is not an error to explain.
	if existing.CancelledAt != nil {
		return connect.NewResponse(&v1.CancelMeetingAsAdminResponse{
			Meeting: adminMeetingToProto(existing),
		}), nil
	}

	row, err := a.users.CancelBooking(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("the meeting could not be cancelled"))
	}
	// The hold is released whatever the calendar says. A failure to
	// remove the event leaves a stale entry on the owner's calendar,
	// which he can see and delete, rather than time nobody can book.
	if row.EventID != "" && a.calendar != nil {
		if err := a.calendar.Cancel(ctx, row.EventID); err != nil {
			a.log.Warn("meetings: cancelled here but the calendar event remains",
				"booking", row.ID, "error", err.Error())
		}
	}
	a.log.Info("meeting cancelled by the owner", "admin", admin.ID, "booking", row.ID)
	return connect.NewResponse(&v1.CancelMeetingAsAdminResponse{
		Meeting: adminMeetingToProto(row),
	}), nil
}

// schedulerZone is the zone the console renders times in, taken from
// the settings so it cannot disagree with the member-facing page.
func (a *Admin) schedulerZone(ctx context.Context) string {
	if a.scheduler == nil {
		return "America/New_York"
	}
	return a.scheduler.Get(ctx).Zone
}

func adminMeetingToProto(b users.Booking) *v1.AdminMeeting {
	m := &v1.AdminMeeting{
		Id:              b.ID,
		Start:           timestamppb.New(b.StartsAt),
		End:             timestamppb.New(b.StartsAt.Add(time.Duration(b.DurationMin) * time.Minute)),
		DurationMinutes: int32(b.DurationMin),
		Topic:           b.Note,
		MemberName:      b.Name,
		MemberEmail:     b.Email,
		EventId:         b.EventID,
		CreatedAt:       timestamppb.New(b.CreatedAt),
	}
	if b.UserID != nil {
		m.MemberId = *b.UserID
	}
	if b.CancelledAt != nil {
		m.CancelledAt = timestamppb.New(*b.CancelledAt)
	}
	return m
}
