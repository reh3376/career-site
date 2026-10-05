package handlers

import (
	"context"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// The decision test's timings, on the admin console.
//
// These were compiled-in constants until the owner had taken the test
// twice and moved them both times. Each move cost a build, a CI run and
// a deploy to change one number, which is the wrong shape for a value
// that is a judgement made from watching people rather than a property
// of the code. Same reasoning as the JD fit bands.
//
// The response carries the derived instrument version and a count of
// sessions already recorded under it, because the cost of changing a
// timing is not the edit, it is that every run before the change becomes
// a different instrument. Showing the count makes that cost visible at
// the moment of deciding rather than at the moment of analysis.

// GetDecisionTestSettings reads the timings.
func (a *Admin) GetDecisionTestSettings(
	ctx context.Context, req *connect.Request[v1.GetDecisionTestSettingsRequest],
) (*connect.Response[v1.GetDecisionTestSettingsResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	s := a.users.DTGetSettings(ctx)
	version := s.InstrumentVersion()
	n, err := a.users.DTSessionsOnVersion(ctx, version)
	if err != nil {
		a.log.Warn("decision test settings: session count", slog.String("error", err.Error()))
	}
	return connect.NewResponse(&v1.GetDecisionTestSettingsResponse{
		MemoriseMs:            int32(s.MemoriseMs),
		QuestionMs:            int32(s.QuestionMs),
		RecallMs:              int32(s.RecallMs),
		InstrumentVersion:     version,
		SessionsOnThisVersion: int32(n),
	}), nil
}

// SetDecisionTestSettings stores them.
func (a *Admin) SetDecisionTestSettings(
	ctx context.Context, req *connect.Request[v1.SetDecisionTestSettingsRequest],
) (*connect.Response[v1.SetDecisionTestSettingsResponse], error) {
	admin, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	s := users.DTSettings{
		MemoriseMs: int(req.Msg.GetMemoriseMs()),
		QuestionMs: int(req.Msg.GetQuestionMs()),
		RecallMs:   int(req.Msg.GetRecallMs()),
	}
	if err := a.users.DTSetSettings(ctx, s, admin.ID); err != nil {
		a.log.Error("decision test settings: save", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("could not save the timings"))
	}
	// Read back rather than echo: the values are clamped on write, so
	// what was asked for and what is in force can differ.
	saved := a.users.DTGetSettings(ctx)
	a.log.Info("decision test timings changed",
		slog.String("instrument_version", saved.InstrumentVersion()),
		slog.Int("memorise_ms", saved.MemoriseMs),
		slog.Int("question_ms", saved.QuestionMs),
		slog.Int("recall_ms", saved.RecallMs))
	return connect.NewResponse(&v1.SetDecisionTestSettingsResponse{
		InstrumentVersion: saved.InstrumentVersion(),
	}), nil
}
