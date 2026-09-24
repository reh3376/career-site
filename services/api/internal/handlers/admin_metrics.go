package handlers

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// GetMetrics returns the state of the reviewer (data layer D5).
//
// Every number is selected from a view. Nothing is computed here, and
// nothing should be computed in the page either: two definitions of the
// same metric is how a dashboard starts disagreeing with itself and
// nobody can say which one is lying. When a number looks wrong, the
// definition is in migration 00028 and there is one of it.
func (a *Admin) GetMetrics(
	ctx context.Context,
	req *connect.Request[v1.GetMetricsRequest],
) (*connect.Response[v1.GetMetricsResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	m, err := a.users.GetMetrics(ctx)
	if err != nil {
		a.log.Error("GetMetrics failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the metrics"))
	}

	out := &v1.GetMetricsResponse{
		RecentRuns:          int32(m.RecentRuns),
		RecentCompleted:     int32(m.RecentCompleted),
		RecentFailed:        int32(m.RecentFailed),
		RecentStuck:         int32(m.RecentStuck),
		Reviewed:            int32(m.Reviewed),
		Gradeable:           int32(m.Gradeable),
		Ungradeable:         int32(m.Ungradeable),
		Agreed:              int32(m.Agreed),
		AgreementPct:        m.AgreementPct,
		SoftDisagreements:   int32(m.SoftDisagreements),
		HardDisagreements:   int32(m.HardDisagreements),
		TooHarsh:            int32(m.TooHarsh),
		TooGenerous:         int32(m.TooGenerous),
		FinishedRuns:        int32(m.FinishedRuns),
		MedianMinutes:       m.MedianMinutes,
		P95Minutes:          m.P95Minutes,
		MedianQueuedMinutes: m.MedianQueuedMinutes,
		Landed:              int32(m.Landed),
		ReadWriting:         int32(m.ReadWriting),
		Clicked:             int32(m.Clicked),
		Registered:          int32(m.Registered),
		Verified:            int32(m.Verified),
		SignedIn:            int32(m.SignedIn),
		Submitted:           int32(m.Submitted),
		Calls:               int32(m.Calls),
		CallFailures:        int32(m.CallFailures),
		PromptTokens:        m.PromptTokens,
		CompletionTokens:    m.CompletionTokens,
	}
	if e := m.LatestEval; e != nil {
		out.LatestEval = &v1.EvalRun{
			Id:              e.ID,
			Note:            e.Note,
			GateCorrect:     int32(e.GateCorrect),
			Scored:          int32(e.Scored),
			OrderViolations: int32(e.OrderViolations),
			Margin:          e.Margin,
		}
	}
	for _, o := range m.Outcomes {
		out.Outcomes = append(out.Outcomes, &v1.OutcomeByFit{
			Fit:         o.Fit,
			Outcome:     o.Outcome,
			Submissions: int32(o.Submissions),
		})
	}
	return connect.NewResponse(out), nil
}

// metricsUnused keeps the users import honest if the struct ever moves.
var _ = users.Metrics{}

// GetGate returns the criteria as a gate (RR-13, RR-14).
//
// /admin/analytics shows these numbers arranged for reading. This
// answers the question a reader of a dashboard has to answer for
// themselves, which is whether each one is good enough yet, and a
// dashboard can be read favourably on a bad day where a pass cannot.
//
// The rows come from views, like every other metric. Pass may be unset,
// meaning the criterion cannot be answered rather than that it failed.
func (a *Admin) GetGate(
	ctx context.Context,
	req *connect.Request[v1.GetGateRequest],
) (*connect.Response[v1.GetGateResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	rows, err := a.users.Gate(ctx)
	if err != nil {
		a.log.Error("GetGate failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the gate"))
	}
	out := make([]*v1.GateRow, 0, len(rows))
	for _, g := range rows {
		row := &v1.GateRow{
			Criterion: g.Criterion,
			Value:     g.Value,
			Target:    g.Target,
			Detail:    g.Detail,
			Pass:      g.Pass,
			Gated:     g.Gated,
		}
		if g.AsOf != nil {
			row.AsOf = timestamppb.New(*g.AsOf)
		}
		out = append(out, row)
	}
	return connect.NewResponse(&v1.GetGateResponse{Rows: out}), nil
}
