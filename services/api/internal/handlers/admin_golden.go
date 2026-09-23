package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// The golden set (data layer D4): fixed postings with a stated
// expectation, re-scored to answer the question that comes up on every
// prompt or model change, which is whether the change helped.

// ListGoldenPostings returns the set with each posting's last result.
func (a *Admin) ListGoldenPostings(
	ctx context.Context,
	req *connect.Request[v1.ListGoldenPostingsRequest],
) (*connect.Response[v1.ListGoldenPostingsResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	set, err := a.users.ListGoldenPostings(ctx, req.Msg.GetActiveOnly())
	if err != nil {
		a.log.Error("ListGoldenPostings failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the golden set"))
	}
	out := make([]*v1.GoldenPosting, 0, len(set))
	for _, g := range set {
		p := &v1.GoldenPosting{
			Id:           g.ID,
			Name:         g.Name,
			JdText:       g.JdText,
			RoleHint:     g.RoleHint,
			EmployerHint: g.EmployerHint,
			ExpectedGate: g.ExpectedGate,
			ExpectedBand: g.ExpectedBand,
			Note:         g.Note,
			Active:       g.Active,
			LastScore:    g.LastScore,
		}
		if g.LastPassed != nil {
			p.LastPassed = *g.LastPassed
		}
		if g.LastEvalAt != nil {
			p.LastEvalAt = timestamppb.New(*g.LastEvalAt)
		}
		out = append(out, p)
	}
	return connect.NewResponse(&v1.ListGoldenPostingsResponse{Postings: out}), nil
}

// UpsertGoldenPosting adds or replaces a posting, keyed by name.
func (a *Admin) UpsertGoldenPosting(
	ctx context.Context,
	req *connect.Request[v1.UpsertGoldenPostingRequest],
) (*connect.Response[v1.UpsertGoldenPostingResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	gate := strings.ToLower(strings.TrimSpace(req.Msg.GetExpectedGate()))
	if gate != "above" && gate != "below" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("expected_gate must be above or below"))
	}
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	id, err := a.users.UpsertGoldenPosting(ctx, users.GoldenPosting{
		Name:         name,
		JdText:       req.Msg.GetJdText(),
		RoleHint:     strings.TrimSpace(req.Msg.GetRoleHint()),
		EmployerHint: strings.TrimSpace(req.Msg.GetEmployerHint()),
		ExpectedGate: gate,
		ExpectedBand: strings.ToLower(strings.TrimSpace(req.Msg.GetExpectedBand())),
		Note:         strings.TrimSpace(req.Msg.GetNote()),
		Active:       true,
	})
	if err != nil {
		a.log.Error("UpsertGoldenPosting failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not save the posting"))
	}
	return connect.NewResponse(&v1.UpsertGoldenPostingResponse{Id: id}), nil
}

// SetGoldenActive retires or restores a posting.
func (a *Admin) SetGoldenActive(
	ctx context.Context,
	req *connect.Request[v1.SetGoldenActiveRequest],
) (*connect.Response[v1.SetGoldenActiveResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	if err := a.users.SetGoldenActive(ctx, req.Msg.GetId(), req.Msg.GetActive()); err != nil {
		a.log.Error("SetGoldenActive failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not update the posting"))
	}
	return connect.NewResponse(&v1.SetGoldenActiveResponse{}), nil
}

// ListEvalRuns returns evaluations newest first.
func (a *Admin) ListEvalRuns(
	ctx context.Context,
	req *connect.Request[v1.ListEvalRunsRequest],
) (*connect.Response[v1.ListEvalRunsResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	runs, err := a.users.ListEvalRuns(ctx, int(req.Msg.GetLimit()))
	if err != nil {
		a.log.Error("ListEvalRuns failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read evaluations"))
	}
	out := make([]*v1.EvalRun, 0, len(runs))
	for _, r := range runs {
		out = append(out, evalRunToProto(r))
	}
	return connect.NewResponse(&v1.ListEvalRunsResponse{Runs: out}), nil
}

// GetEvalRun returns one evaluation with its per-posting results.
func (a *Admin) GetEvalRun(
	ctx context.Context,
	req *connect.Request[v1.GetEvalRunRequest],
) (*connect.Response[v1.GetEvalRunResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	run, err := a.users.GetEvalRun(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("evaluation not found"))
	}
	return connect.NewResponse(&v1.GetEvalRunResponse{Run: evalRunToProto(*run)}), nil
}

func evalRunToProto(r users.EvalRun) *v1.EvalRun {
	promptsJSON := "{}"
	if len(r.Prompts) > 0 {
		if b, err := json.Marshal(r.Prompts); err == nil {
			promptsJSON = string(b)
		}
	}
	out := &v1.EvalRun{
		Id:                r.ID,
		EvalId:            r.EvalID,
		Status:            r.Status,
		Note:              r.Note,
		AppCommit:         r.AppCommit,
		Host:              r.Host,
		Model:             r.Model,
		NumCtx:            int32(r.NumCtx),
		EmbedderModel:     r.EmbedderModel,
		PromptsJson:       promptsJSON,
		CorpusFingerprint: r.CorpusFingerprint,
		Threshold:         r.Threshold,
		Total:             int32(r.Total),
		Scored:            int32(r.Scored),
		GateCorrect:       int32(r.GateCorrect),
		OrderViolations:   int32(r.OrderViolations),
		Margin:            r.Margin,
		Errors:            int32(r.Errors),
		StartedAt:         timestamppb.New(r.StartedAt),
	}
	if r.FinishedAt != nil {
		out.FinishedAt = timestamppb.New(*r.FinishedAt)
	}
	for _, it := range r.Items {
		out.Items = append(out.Items, &v1.EvalItem{
			GoldenId:     it.GoldenID,
			GoldenName:   it.GoldenName,
			ExpectedGate: it.ExpectedGate,
			RunId:        it.RunID,
			SubmissionId: it.SubmissionID,
			MatchScore:   it.MatchScore,
			Fit:          it.Fit,
			GateSide:     it.GateSide,
			Passed:       it.Passed,
			Error:        it.Error,
		})
	}
	return out
}

// evalJobSummary is the one-line result the job runner shows.
func evalJobSummary(gateCorrect, scored, violations int, margin *float64) string {
	s := fmt.Sprintf("%d of %d on the expected side, %d inversions", gateCorrect, scored, violations)
	if margin != nil {
		s += fmt.Sprintf(", margin %.3f", *margin)
	}
	return s
}
