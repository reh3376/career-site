package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

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

// ListDecisionTestRuns returns runs, newest first.
func (a *Admin) ListDecisionTestRuns(
	ctx context.Context, req *connect.Request[v1.ListDecisionTestRunsRequest],
) (*connect.Response[v1.ListDecisionTestRunsResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	runs, err := a.users.DTListRuns(ctx, req.Msg.GetIncludeSynthetic())
	if err != nil {
		a.log.Error("decision test: list runs", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("could not read the runs"))
	}
	out := make([]*v1.DecisionTestRun, 0, len(runs))
	for _, r := range runs {
		out = append(out, toProtoRun(r))
	}
	return connect.NewResponse(&v1.ListDecisionTestRunsResponse{Runs: out}), nil
}

// GetDecisionTestRun returns one run in full.
func (a *Admin) GetDecisionTestRun(
	ctx context.Context, req *connect.Request[v1.GetDecisionTestRunRequest],
) (*connect.Response[v1.GetDecisionTestRunResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	run, blocks, answers, err := a.users.DTGetRun(ctx, req.Msg.GetSessionKey())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no such run"))
	}
	pb := make([]*v1.DecisionTestBlock, 0, len(blocks))
	for _, b := range blocks {
		pb = append(pb, &v1.DecisionTestBlock{
			BlockNo: int32(b.BlockNo), Load: b.Load,
			Correct: int32(b.Correct), Total: int32(b.Total),
			Lure: int32(b.Lure), Expired: int32(b.Expired),
			MeanConfidence: int32(b.MeanConfidence), MeanLatencyMs: int32(b.MeanLatencyMs),
			PresentedDigits: b.PresentedDigits, ExpectedDigits: b.ExpectedDigits,
			ResponseDigits: b.ResponseDigits, RecallOutcome: b.RecallOutcome,
			MemoryFailurePct: int32(b.MemoryFailure * 100),
		})
	}
	pa := make([]*v1.DecisionTestAnswer, 0, len(answers))
	for _, x := range answers {
		pa = append(pa, &v1.DecisionTestAnswer{
			Position: int32(x.Position), BlockNo: int32(x.BlockNo),
			ItemCode: x.ItemCode, ItemFamily: x.ItemFamily, Outcome: x.Outcome,
			Confidence: int32(x.Confidence), LatencyMs: int32(x.LatencyMs),
		})
	}
	return connect.NewResponse(&v1.GetDecisionTestRunResponse{
		Run: toProtoRun(*run), Blocks: pb, Answers: pa,
	}), nil
}

// ExportDecisionTestData hands over the curated dataset as CSV.
//
// Thin on purpose. Everything that decides what the dataset contains is
// in v_dt_answers and in users.DTExportCSV, so there is no place here
// where a column could be added, filtered or renamed on its way out.
func (a *Admin) ExportDecisionTestData(
	ctx context.Context, req *connect.Request[v1.ExportDecisionTestDataRequest],
) (*connect.Response[v1.ExportDecisionTestDataResponse], error) {
	// This call was shipped without it on 2026-10-05 and the entire
	// dataset was downloadable by anyone, with no session, for about
	// five hours. The proto declares AUTH_LEVEL_ADMIN and that is not
	// enforcement: the comment on the Admin type says so in as many
	// words, because the auth interceptor has not landed and every
	// method here gates itself.
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	if a.users == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("not configured"))
	}
	out, err := a.users.DTExportCSV(ctx, req.Msg.GetIncludeSynthetic())
	if err != nil {
		a.log.Error("decision test: export", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("could not build the export"))
	}
	a.log.Info("decision test exported",
		slog.Int("rows", out.Rows), slog.Int("sessions", out.Sessions),
		slog.Bool("include_synthetic", req.Msg.GetIncludeSynthetic()))
	return connect.NewResponse(&v1.ExportDecisionTestDataResponse{
		Csv: out.CSV, Filename: out.Filename,
		Rows: int32(out.Rows), Sessions: int32(out.Sessions),
	}), nil
}

// toProtoRun copies a run onto the wire.
//
// gave_email rather than the address itself: the admin list needs to
// know whether results can be sent, not what the address is, and a
// mailing list is not what this surface is for.
func toProtoRun(r users.DTRun) *v1.DecisionTestRun {
	return &v1.DecisionTestRun{
		SessionKey: r.SessionKey, Status: r.Status,
		DisplayName: r.DisplayName, AgeRange: r.AgeRange,
		Education: r.Education, Occupation: r.Occupation, GaveEmail: r.GaveEmail,
		AudioMode: r.AudioMode, DeviceClass: r.DeviceClass,
		TapCheckPassed: r.TapCheckPassed, BaselineRtMs: int32(r.BaselineRTMs),
		IsRepeat: r.IsRepeat, IsSynthetic: r.IsSynthetic,
		InstrumentVersion: r.InstrumentVersion, ItemSetVersion: r.ItemSetVersion,
		Correct: int32(r.Correct), Answered: int32(r.Answered), Expired: int32(r.Expired),
		MeanConfidence: int32(r.MeanConfidence), DurationS: int32(r.DurationS),
		StartedAt: r.StartedAt.UTC().Format(time.RFC3339),
	}
}
