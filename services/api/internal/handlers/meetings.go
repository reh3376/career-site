package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	careerv1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/gen/career/v1/careerv1connect"
	"github.com/reh3376/career-site/services/api/internal/calendar"
	"github.com/reh3376/career-site/services/api/internal/scheduling"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Meetings implements careerv1connect.MeetingServiceHandler: the
// members-only booking flow against the owner's own calendar.
//
// Members-only is the owner's decision (2026-09-29) and the reason is
// specific: the target is his main personal calendar, so an
// unauthenticated visitor could hold real hours on it. Every RPC here
// resolves a session first, and the booking member is recorded on the
// row.
type Meetings struct {
	careerv1connect.UnimplementedMeetingServiceHandler

	log      *slog.Logger
	users    *users.Repo
	auth     *Auth // session lookup; booking is members-only
	settings *scheduling.SettingsStore
	cal      calendar.Provider
}

// NewMeetings wires the handler. A nil calendar provider is not an
// error: the pages still render and say booking is unavailable, which
// is what happens before the owner has connected his calendar.
func NewMeetings(
	log *slog.Logger,
	repo *users.Repo,
	auth *Auth,
	settings *scheduling.SettingsStore,
	cal calendar.Provider,
) *Meetings {
	if cal == nil {
		cal = calendar.NotConnected{}
	}
	return &Meetings{log: log, users: repo, auth: auth, settings: settings, cal: cal}
}

// GetMeetingOptions returns what the member may choose and the clock it
// is quoted in, so the page is built from the owner's live settings.
func (h *Meetings) GetMeetingOptions(
	ctx context.Context,
	req *connect.Request[careerv1.GetMeetingOptionsRequest],
) (*connect.Response[careerv1.GetMeetingOptionsResponse], error) {
	if _, err := h.requireMember(ctx, req); err != nil {
		return nil, err
	}
	set := h.settings.Get(ctx)

	out := &careerv1.GetMeetingOptionsResponse{
		DurationMinutes: int32Slice(set.Durations),
		Zone:            set.Zone,
		ZoneLabel:       zoneLabel(set.Zone),
		HorizonDays:     int32(set.HorizonDays),
		LeadHours:       int32(set.LeadHours),
		HoursSummary:    hoursSummary(set),
		Available:       true,
	}
	// Asking the calendar whether it can answer at all is cheaper than
	// discovering it cannot halfway through rendering a month, and it
	// lets the page say "not connected" instead of showing an empty
	// calendar that reads as "never free".
	if err := h.cal.Healthy(ctx); err != nil {
		out.Available = false
		out.UnavailableReason = unavailableReason(err)
	}
	return connect.NewResponse(out), nil
}

// GetAvailability returns the start times bookable for one length.
func (h *Meetings) GetAvailability(
	ctx context.Context,
	req *connect.Request[careerv1.GetAvailabilityRequest],
) (*connect.Response[careerv1.GetAvailabilityResponse], error) {
	if _, err := h.requireMember(ctx, req); err != nil {
		return nil, err
	}
	set := h.settings.Get(ctx)
	dur := int(req.Msg.GetDurationMinutes())
	if !set.Allows(dur) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("%d minutes is not a meeting length on offer", dur))
	}

	now := time.Now()
	from, to := h.window(set, now, req.Msg.GetFrom().AsTime(), req.Msg.GetTo().AsTime(),
		req.Msg.GetFrom() != nil, req.Msg.GetTo() != nil)

	// Free/busy only, never event contents: this application must never
	// hold the titles of the owner's private appointments.
	busy, err := h.cal.FreeBusy(ctx, from, to)
	if err != nil {
		h.log.Warn("meetings: free/busy unavailable",
			slog.String("error", err.Error()))
		return connect.NewResponse(&careerv1.GetAvailabilityResponse{
			Zone:              set.Zone,
			Available:         false,
			UnavailableReason: unavailableReason(err),
		}), nil
	}
	booked, err := h.activeBookings(ctx, from, to)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read existing bookings"))
	}

	slots, err := scheduling.Slots(set, dur, asIntervals(busy), booked, now)
	if err != nil {
		// Invalid settings, not a bad request: the member asked a fair
		// question and the owner's configuration cannot answer it.
		h.log.Error("meetings: settings cannot produce slots", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("the calendar is misconfigured"))
	}

	out := &careerv1.GetAvailabilityResponse{Zone: set.Zone, Available: true}
	for _, s := range slots {
		out.Slots = append(out.Slots, &careerv1.MeetingSlot{
			Start: timestamppb.New(s.Start),
			End:   timestamppb.New(s.End),
		})
	}
	return connect.NewResponse(out), nil
}

// BookMeeting claims a slot and creates the calendar event.
func (h *Meetings) BookMeeting(
	ctx context.Context,
	req *connect.Request[careerv1.BookMeetingRequest],
) (*connect.Response[careerv1.BookMeetingResponse], error) {
	member, err := h.requireMember(ctx, req)
	if err != nil {
		return nil, err
	}
	set := h.settings.Get(ctx)
	dur := int(req.Msg.GetDurationMinutes())
	if !set.Allows(dur) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("%d minutes is not a meeting length on offer", dur))
	}
	start := req.Msg.GetStart().AsTime()

	svc := &scheduling.Service{
		Cal:   h.cal,
		Store: &scheduling.RepoStore{Repo: h.users, Gap: set.GapMins},
		Log:   h.log,
	}
	booked, err := svc.Book(ctx, set, scheduling.Booking{
		UserID:  member.ID,
		Name:    member.Name,
		Email:   member.Email,
		Note:    req.Msg.GetTopic(),
		Start:   start,
		DurMins: dur,
	})
	// Two ways the same thing happens: the database refused the claim
	// (another member got there first), or the calendar's second look
	// showed the owner had filled it himself. Both are the same answer
	// to the member.
	var taken scheduling.ErrSlotTaken
	switch {
	case errors.Is(err, scheduling.ErrAlreadyClaimed), errors.As(err, &taken):
		// The member did nothing wrong; the list they were shown went
		// stale while they were deciding. The page re-fetches on this.
		return nil, connect.NewError(connect.CodeAlreadyExists,
			errors.New("that time has just been taken; please pick another"))
	case errors.Is(err, calendar.ErrNotConnected):
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("booking is not available at the moment"))
	case err != nil:
		h.log.Error("meetings: booking failed",
			slog.Int64("user", member.ID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("the meeting could not be booked"))
	}

	return connect.NewResponse(&careerv1.BookMeetingResponse{
		Meeting: toProto(booked, set.Zone),
	}), nil
}

// ListMyMeetings returns the caller's own bookings, soonest first.
func (h *Meetings) ListMyMeetings(
	ctx context.Context,
	req *connect.Request[careerv1.ListMyMeetingsRequest],
) (*connect.Response[careerv1.ListMyMeetingsResponse], error) {
	member, err := h.requireMember(ctx, req)
	if err != nil {
		return nil, err
	}
	set := h.settings.Get(ctx)
	rows, err := h.users.BookingsForUser(ctx, member.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read your meetings"))
	}

	now := time.Now()
	out := &careerv1.ListMyMeetingsResponse{}
	for _, r := range rows {
		b := scheduling.FromRow(r)
		past := b.Start.Before(now) || b.Canceled != nil
		if past && !req.Msg.GetIncludePast() {
			continue
		}
		out.Meetings = append(out.Meetings, toProto(b, set.Zone))
	}
	sort.Slice(out.Meetings, func(i, j int) bool {
		return out.Meetings[i].GetStart().AsTime().Before(out.Meetings[j].GetStart().AsTime())
	})
	return connect.NewResponse(out), nil
}

// CancelMeeting cancels the caller's own meeting and frees the time.
func (h *Meetings) CancelMeeting(
	ctx context.Context,
	req *connect.Request[careerv1.CancelMeetingRequest],
) (*connect.Response[careerv1.CancelMeetingResponse], error) {
	member, err := h.requireMember(ctx, req)
	if err != nil {
		return nil, err
	}
	set := h.settings.Get(ctx)

	// Ownership is checked before anything is changed. Without this a
	// member could cancel a stranger's meeting by guessing an id.
	rows, err := h.users.BookingsForUser(ctx, member.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read your meetings"))
	}
	var owned *users.Booking
	for i := range rows {
		if rows[i].ID == req.Msg.GetId() {
			owned = &rows[i]
			break
		}
	}
	if owned == nil {
		// Not found rather than permission denied: whether a meeting
		// exists is not something a stranger should be able to probe.
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such meeting"))
	}
	// Cancelling something already cancelled succeeds, so a double
	// click or a stale tab is not an error the member has to read.
	if owned.CancelledAt != nil {
		return connect.NewResponse(&careerv1.CancelMeetingResponse{
			Meeting: toProto(scheduling.FromRow(*owned), set.Zone),
		}), nil
	}

	row, err := h.users.CancelBooking(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("the meeting could not be cancelled"))
	}
	// The hold is released whatever the calendar says; a failure to
	// remove the event leaves a stale entry on the owner's calendar,
	// which is visible and fixable, rather than time nobody can book.
	if row.EventID != "" {
		if err := h.cal.Cancel(ctx, row.EventID); err != nil {
			h.log.Warn("meetings: booking cancelled but the calendar event remains",
				slog.Int64("booking", row.ID), slog.String("error", err.Error()))
		}
	}
	return connect.NewResponse(&careerv1.CancelMeetingResponse{
		Meeting: toProto(scheduling.FromRow(row), set.Zone),
	}), nil
}

// window clamps the requested range to the lead time and the horizon,
// so a caller cannot ask about last week or next year.
func (h *Meetings) window(set scheduling.Settings, now, from, to time.Time, hasFrom, hasTo bool) (time.Time, time.Time) {
	earliest := now.Add(time.Duration(set.LeadHours) * time.Hour)
	latest := now.AddDate(0, 0, set.HorizonDays)
	if !hasFrom || from.Before(earliest) {
		from = earliest
	}
	if !hasTo || to.After(latest) {
		to = latest
	}
	if !to.After(from) {
		to = from
	}
	return from, to
}

// asIntervals converts the calendar's busy spans to this package's.
// Two types rather than one on purpose: the calendar's is what a
// provider returns, and keeping them distinct is what stops a provider
// type leaking into the availability arithmetic.
func asIntervals(in []calendar.Interval) []scheduling.Interval {
	out := make([]scheduling.Interval, 0, len(in))
	for _, i := range in {
		out = append(out, scheduling.Interval{Start: i.Start, End: i.End})
	}
	return out
}

func (h *Meetings) activeBookings(ctx context.Context, from, to time.Time) ([]scheduling.Interval, error) {
	rows, err := h.users.ActiveBookings(ctx, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]scheduling.Interval, 0, len(rows))
	for _, r := range rows {
		out = append(out, scheduling.Interval{
			Start: r.StartsAt,
			End:   r.StartsAt.Add(time.Duration(r.DurationMin) * time.Minute),
		})
	}
	return out, nil
}

// requireMember resolves the caller's session or fails the RPC. The
// proto declares AUTH_LEVEL_MEMBER; enforcement lives here until the
// auth interceptor lands, matching the JD handler.
func (h *Meetings) requireMember(ctx context.Context, req connect.AnyRequest) (*users.User, error) {
	if h.auth == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("auth not wired"))
	}
	u, err := h.auth.LookupSessionUser(ctx, req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("session lookup failed"))
	}
	if u == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("sign in to book a meeting"))
	}
	if u.Status != users.StatusActive {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("account is not active"))
	}
	return u, nil
}

func toProto(b scheduling.Booking, zone string) *careerv1.Meeting {
	m := &careerv1.Meeting{
		Id:              b.ID,
		Start:           timestamppb.New(b.Start),
		End:             timestamppb.New(b.Start.Add(time.Duration(b.DurMins) * time.Minute)),
		DurationMinutes: int32(b.DurMins),
		Topic:           b.Note,
		Zone:            zone,
		IcsUrl:          fmt.Sprintf("/api/meetings/%d.ics", b.ID),
	}
	if b.Canceled != nil {
		m.CancelledAt = timestamppb.New(*b.Canceled)
	}
	return m
}

func int32Slice(in []int) []int32 {
	out := make([]int32, 0, len(in))
	for _, v := range in {
		out = append(out, int32(v))
	}
	return out
}

// unavailableReason turns a provider error into something a member can
// read. The distinction that matters is "not set up yet" against
// "temporarily broken", because only one of them is worth retrying.
func unavailableReason(err error) string {
	switch {
	case errors.Is(err, calendar.ErrNotConnected):
		return "Booking is not switched on yet."
	case errors.Is(err, calendar.ErrUnavailable):
		return "The calendar is not reachable right now. Please try again shortly."
	default:
		return "Booking is unavailable right now."
	}
}

// zoneLabel is the wording to print beside a time. Kept to the zones
// actually in use rather than a table of every IANA name, and falling
// back to the name itself, which is wrong in style but never wrong in
// fact.
func zoneLabel(zone string) string {
	switch zone {
	case "America/New_York":
		return "Eastern time"
	case "America/Chicago":
		return "Central time"
	case "America/Denver":
		return "Mountain time"
	case "America/Los_Angeles":
		return "Pacific time"
	case "UTC":
		return "UTC"
	}
	return zone
}

// hoursSummary describes the owner's stated hours in a sentence, so a
// member reads intent rather than decoding a grid of windows.
func hoursSummary(set scheduling.Settings) string {
	if len(set.Windows) == 0 {
		return ""
	}
	seen := map[time.Weekday]bool{}
	var days []time.Weekday
	for _, w := range set.Windows {
		if !seen[w.Weekday] {
			seen[w.Weekday] = true
			days = append(days, w.Weekday)
		}
	}
	sort.Slice(days, func(i, j int) bool { return days[i] < days[j] })

	names := make([]string, 0, len(days))
	for _, d := range days {
		names = append(names, d.String())
	}
	switch len(names) {
	case 1:
		return names[0] + "s"
	case 2:
		return names[0] + "s and " + names[1] + "s"
	default:
		// Contiguous runs read as a range; anything else is listed.
		if int(days[len(days)-1]-days[0]) == len(days)-1 {
			return names[0] + " to " + names[len(names)-1]
		}
		return joinWithAnd(names)
	}
}

func joinWithAnd(names []string) string {
	out := ""
	for i, n := range names {
		switch {
		case i == 0:
			out = n
		case i == len(names)-1:
			out += " and " + n
		default:
			out += ", " + n
		}
	}
	return out
}
