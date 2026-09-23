package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// SetJdOutcome records what happened after a review.
//
// This is the only signal in the whole system that says whether a score
// predicted anything. Everything else measures whether the reasoning
// was sound, which is a different question: a posting can score 0.92
// with every requirement properly evidenced and still be a job that
// goes nowhere.
func (a *Admin) SetJdOutcome(
	ctx context.Context,
	req *connect.Request[v1.SetJdOutcomeRequest],
) (*connect.Response[v1.SetJdOutcomeResponse], error) {
	admin, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	id, err := strconv.ParseInt(strings.TrimSpace(req.Msg.GetSubmissionId()), 10, 64)
	if err != nil || id <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("submission_id must be numeric"))
	}
	status := strings.ToLower(strings.TrimSpace(req.Msg.GetStatus()))
	if !users.ValidOutcomeStatus(status) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("status must be one of %s", strings.Join(users.OutcomeStatuses, ", ")))
	}

	out := users.JdOutcome{
		SubmissionID: id,
		Status:       status,
		Note:         strings.TrimSpace(req.Msg.GetNote()),
		RecordedBy:   admin.ID,
	}
	// A day the person knows, not the day they typed it in. Empty is
	// legitimate: "this was rejected at some point" is still worth more
	// than no record at all, so a missing date is not an error.
	if raw := strings.TrimSpace(req.Msg.GetDecidedOn()); raw != "" {
		day, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("decided_on must be YYYY-MM-DD"))
		}
		out.DecidedOn = &day
	}

	if err := a.users.SetJdOutcome(ctx, out); err != nil {
		a.log.Error("SetJdOutcome failed", slog.Int64("id", id), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not record the outcome"))
	}
	a.events.Emit(ctx, requestEvent(req, "jd.outcome_recorded", admin.ID,
		map[string]any{"submission_id": id, "status": status}))

	stored, _ := a.users.GetJdOutcome(ctx, id)
	return connect.NewResponse(&v1.SetJdOutcomeResponse{Outcome: outcomeToProto(stored)}), nil
}

// RecordJdFeedback stores the owner's judgment of a run's output.
func (a *Admin) RecordJdFeedback(
	ctx context.Context,
	req *connect.Request[v1.RecordJdFeedbackRequest],
) (*connect.Response[v1.RecordJdFeedbackResponse], error) {
	admin, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	id, err := strconv.ParseInt(strings.TrimSpace(req.Msg.GetSubmissionId()), 10, 64)
	if err != nil || id <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("submission_id must be numeric"))
	}
	target := strings.ToLower(strings.TrimSpace(req.Msg.GetTarget()))
	rating := strings.ToLower(strings.TrimSpace(req.Msg.GetRating()))
	if !users.ValidFeedback(target, rating) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("rating %q is not valid for target %q", rating, target))
	}

	run := strings.TrimSpace(req.Msg.GetRunId())
	if run == "" {
		// Attach to the newest run, which is what the page was showing.
		if runs, rErr := a.users.ListJdRuns(ctx, id); rErr == nil && len(runs) > 0 {
			run = runs[0].RunID
		}
	}

	if err := a.users.RecordJdFeedback(ctx, users.JdFeedback{
		SubmissionID: id,
		RunID:        run,
		Source:       "owner",
		UserID:       admin.ID,
		Target:       target,
		Rating:       rating,
		Note:         strings.TrimSpace(req.Msg.GetNote()),
	}); err != nil {
		a.log.Error("RecordJdFeedback failed", slog.Int64("id", id), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not record the feedback"))
	}
	a.events.Emit(ctx, requestEvent(req, "jd.feedback_recorded", admin.ID,
		map[string]any{"submission_id": id, "target": target, "rating": rating}))

	return connect.NewResponse(&v1.RecordJdFeedbackResponse{}), nil
}

func outcomeToProto(o *users.JdOutcome) *v1.JdOutcome {
	if o == nil {
		return nil
	}
	out := &v1.JdOutcome{
		Status:    o.Status,
		Note:      o.Note,
		UpdatedAt: timestamppb.New(o.UpdatedAt),
	}
	if o.DecidedOn != nil {
		out.DecidedOn = o.DecidedOn.Format("2006-01-02")
	}
	return out
}

func feedbackToProto(rows []users.JdFeedback) []*v1.JdFeedback {
	out := make([]*v1.JdFeedback, 0, len(rows))
	for _, f := range rows {
		out = append(out, &v1.JdFeedback{
			RunId:     f.RunID,
			Source:    f.Source,
			Target:    f.Target,
			Rating:    f.Rating,
			Note:      f.Note,
			CreatedAt: timestamppb.New(f.CreatedAt),
		})
	}
	return out
}
