package handlers

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/calendar"
	"github.com/reh3376/career-site/services/api/internal/scheduling"
)

// The meeting-scheduler settings are the owner's hours: which windows a
// member may book into, how long a meeting may be, and the clearance
// between them. They live in app_settings and take effect within
// seconds, because "not next Tuesday" should not need a deploy.

func (a *Admin) GetSchedulerSettings(
	ctx context.Context,
	req *connect.Request[v1.GetSchedulerSettingsRequest],
) (*connect.Response[v1.GetSchedulerSettingsResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	if a.scheduler == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("scheduler not wired"))
	}
	out := &v1.GetSchedulerSettingsResponse{
		Settings:          schedulerToProto(a.scheduler.Get(ctx)),
		CalendarConnected: true,
	}
	// Whether the windows are live is a different question from what
	// they say. Without this the surface would show a full week of
	// availability that nobody can actually book.
	if a.calendar == nil {
		out.CalendarConnected = false
		out.CalendarStatus = "No calendar is connected yet, so nothing can be booked."
	} else if err := a.calendar.Healthy(ctx); err != nil {
		out.CalendarConnected = false
		out.CalendarStatus = calendarStatus(err)
	}
	return connect.NewResponse(out), nil
}

func (a *Admin) SetSchedulerSettings(
	ctx context.Context,
	req *connect.Request[v1.SetSchedulerSettingsRequest],
) (*connect.Response[v1.SetSchedulerSettingsResponse], error) {
	admin, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	if a.scheduler == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("scheduler not wired"))
	}
	next := schedulerFromProto(req.Msg.GetSettings())

	// Validated and refused whole. A half-applied calendar is worse
	// than an unchanged one, and the message is the validator's own,
	// which names the field, because "invalid settings" helps nobody
	// staring at a form.
	if err := a.scheduler.Put(ctx, next, admin.ID); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	a.log.Info("scheduler settings updated",
		"admin", admin.ID, "zone", next.Zone, "windows", len(next.Windows))
	return connect.NewResponse(&v1.SetSchedulerSettingsResponse{
		Settings: schedulerToProto(a.scheduler.Get(ctx)),
	}), nil
}

func schedulerToProto(s scheduling.Settings) *v1.SchedulerSettings {
	out := &v1.SchedulerSettings{
		Zone:            s.Zone,
		DurationMinutes: int32Slice(s.Durations),
		GapMinutes:      int32(s.GapMins),
		StepMinutes:     int32(s.StepMins),
		MaxPerDay:       int32(s.MaxPerDay),
		LeadHours:       int32(s.LeadHours),
		HorizonDays:     int32(s.HorizonDays),
	}
	for _, w := range s.Windows {
		out.Windows = append(out.Windows, &v1.SchedulerWindow{
			Weekday:      int32(w.Weekday),
			StartMinutes: int32(w.StartMins),
			EndMinutes:   int32(w.EndMins),
		})
	}
	return out
}

func schedulerFromProto(p *v1.SchedulerSettings) scheduling.Settings {
	if p == nil {
		return scheduling.Settings{}
	}
	out := scheduling.Settings{
		Zone:        p.GetZone(),
		GapMins:     int(p.GetGapMinutes()),
		StepMins:    int(p.GetStepMinutes()),
		MaxPerDay:   int(p.GetMaxPerDay()),
		LeadHours:   int(p.GetLeadHours()),
		HorizonDays: int(p.GetHorizonDays()),
	}
	for _, d := range p.GetDurationMinutes() {
		out.Durations = append(out.Durations, int(d))
	}
	for _, w := range p.GetWindows() {
		out.Windows = append(out.Windows, scheduling.Window{
			Weekday:   time.Weekday(w.GetWeekday()),
			StartMins: int(w.GetStartMinutes()),
			EndMins:   int(w.GetEndMinutes()),
		})
	}
	return out
}

// calendarStatus turns a provider error into something the owner can
// act on. The distinction that matters to him is "I have not connected
// it" against "it is connected and currently broken".
func calendarStatus(err error) string {
	switch {
	case errors.Is(err, calendar.ErrNotConnected):
		return "No calendar is connected yet, so nothing can be booked."
	case errors.Is(err, calendar.ErrUnavailable):
		return "The calendar is connected but not reachable right now."
	default:
		return "The calendar is not usable right now."
	}
}
