package handlers

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/hiring"
)

// The clarification loop's surface (FSD FR-HF-13, 14, 22 and 24; Phase
// 0 step 7). These six methods are the candidate's side of the loop:
// reading the follow-up questions a run wrote, answering them, and
// submitting the answers, which starts the run that judges the answered
// requirements again.
//
// In Phase 0 the candidate is the owner, who is the one admin, so the
// methods are admin methods. That is a deviation from 14 §3.3, which
// gives them to the candidate's own session, and it is the honest
// level for now: there is no candidate account for them to belong to.
// What a candidate is sent is already what it will be then. A question
// carries the posting's own words, the question and the inputs it
// offers, and never a threshold, an evidence standard, a determination
// or a score.

// clarificationRunner is what starts the run a submitted questionnaire
// owes: the scorer, or a stand-in under test.
type clarificationRunner interface {
	CloseClarification(ctx context.Context, q hiring.ClarificationRequest, adminID int64)
}

// SetClarifications wires the runner of closing runs.
func (h *Hiring) SetClarifications(r clarificationRunner) { h.clarify = r }

// clarifyError maps the loop's errors onto Connect codes with the
// sentence the package wrote for the person answering.
func clarifyError(err error) error {
	switch {
	case errors.Is(err, hiring.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("those questions were not found"))
	case errors.Is(err, hiring.ErrInvalidAnswer):
		return connect.NewError(connect.CodeInvalidArgument,
			errors.New(strings.TrimPrefix(err.Error(), hiring.ErrInvalidAnswer.Error()+": ")))
	case errors.Is(err, hiring.ErrClarificationClosed):
		return connect.NewError(connect.CodeFailedPrecondition,
			errors.New(strings.TrimPrefix(err.Error(), hiring.ErrClarificationClosed.Error()+": ")))
	case errors.Is(err, hiring.ErrNothingToAsk):
		return connect.NewError(connect.CodeFailedPrecondition,
			errors.New("there is nothing left to ask about on this record"))
	}
	return connect.NewError(connect.CodeInternal, errors.New("the questions could not be read or saved"))
}

func timeProto(t *time.Time) *timestamppb.Timestamp {
	if t == nil || t.IsZero() {
		return nil
	}
	return timestamppb.New(*t)
}

func clarificationAnswerProto(a hiring.Answer) *v1.ClarificationAnswer {
	out := &v1.ClarificationAnswer{Status: a.Status, FreeText: a.FreeText}
	if out.Status == "" {
		out.Status = hiring.ItemPending
	}
	for name, raw := range a.Values {
		var list []string
		switch x := raw.(type) {
		case string:
			list = []string{x}
		case float64:
			list = []string{strconv.FormatFloat(x, 'f', -1, 64)}
		case []string:
			list = x
		case []any:
			for _, v := range x {
				if s, ok := v.(string); ok {
					list = append(list, s)
				}
			}
		}
		if len(list) == 0 {
			continue
		}
		if out.Values == nil {
			out.Values = map[string]*v1.StringList{}
		}
		out.Values[name] = &v1.StringList{Values: list}
	}
	return out
}

// clarificationItemProto renders a question as the candidate sees it.
// Nothing here is read from the judgment or from the requirement's
// thresholds: the uncertainty code that chose the question stays on the
// row and is not sent.
func clarificationItemProto(it hiring.ClarificationItem) *v1.ClarificationItem {
	quote := strings.TrimSpace(it.SourceQuote)
	if quote == "" {
		quote = strings.TrimSpace(it.RequirementText)
	}
	out := &v1.ClarificationItem{
		Id: it.ID, RequirementLocalId: it.LocalID, SourceQuote: quote,
		AnswerType: it.AnswerType, Question: it.QuestionText,
		FreeTextRequired: hiring.FreeTextRequired(it.AnswerType), FreeTextMax: hiring.MaxAnswerFreeText,
		Answer: clarificationAnswerProto(it.Answer), AnsweredAt: timeProto(it.AnsweredAt),
	}
	for _, f := range it.Fields {
		pf := &v1.ClarificationField{
			Name: f.Name, Label: f.Label, Kind: f.Kind, Required: f.Required,
			Max: f.Max, MaxLen: int32(f.MaxLen), Hint: f.Hint,
		}
		for _, o := range f.Options {
			pf.Options = append(pf.Options, &v1.ClarificationOption{Value: o.Value, Label: o.Label, Source: o.Source})
		}
		out.Fields = append(out.Fields, pf)
	}
	if (it.Answer.Status == hiring.ItemAnswered || it.Answer.Status == hiring.ItemNotApplicable) && it.AnsweredAt != nil {
		out.Statement = hiring.AnswerStatement(it.Fields, it.Answer, *it.AnsweredAt)
	}
	return out
}

func clarificationRequestProto(q hiring.ClarificationRequest, items, pending int, withItems bool) *v1.ClarificationRequest {
	out := &v1.ClarificationRequest{
		Id: q.ID, SubmissionId: strconv.FormatInt(q.SubmissionID, 10), RunId: q.RunID, Attempt: int32(q.Attempt),
		RoleTitle: q.RoleTitle, Employer: q.Employer, Status: q.Status, Held: q.Held, AskedBy: q.AskedBy,
		GeneratedAt: timestamppb.New(q.GeneratedAt), ExpiresAt: timeProto(q.ExpiresAt),
		AnsweredAt: timeProto(q.AnsweredAt), ClosedAt: timeProto(q.ClosedAt), ClosedReason: q.ClosedReason,
		ItemCount: int32(items), PendingCount: int32(pending),
	}
	if withItems {
		for _, it := range q.Items {
			out.Items = append(out.Items, clarificationItemProto(it))
		}
	}
	return out
}

func fullClarificationRequestProto(q hiring.ClarificationRequest) *v1.ClarificationRequest {
	return clarificationRequestProto(q, len(q.Items), q.Pending(), true)
}

// ListClarificationRequests lists the questionnaires, newest first.
func (h *Hiring) ListClarificationRequests(
	ctx context.Context,
	req *connect.Request[v1.ListClarificationRequestsRequest],
) (*connect.Response[v1.ListClarificationRequestsResponse], error) {
	if _, err := RequireCaller(ctx); err != nil {
		return nil, err
	}
	list, counts, err := h.repo.ListClarificationRequests(ctx, req.Msg.GetOpenOnly(), 100)
	if err != nil {
		h.log.Error("hiring: list clarification requests", slog.String("error", err.Error()))
		return nil, clarifyError(err)
	}
	out := &v1.ListClarificationRequestsResponse{}
	for _, q := range list {
		c := counts[q.ID]
		out.Requests = append(out.Requests, clarificationRequestProto(q, c[0], c[1], false))
	}
	return connect.NewResponse(out), nil
}

// GetClarificationRequest reads one questionnaire with its questions.
func (h *Hiring) GetClarificationRequest(
	ctx context.Context,
	req *connect.Request[v1.GetClarificationRequestRequest],
) (*connect.Response[v1.GetClarificationRequestResponse], error) {
	if _, err := RequireCaller(ctx); err != nil {
		return nil, err
	}
	q, err := h.repo.GetClarificationRequest(ctx, strings.TrimSpace(req.Msg.GetClarificationRequestId()))
	if err != nil {
		if !errors.Is(err, hiring.ErrNotFound) {
			h.log.Error("hiring: clarification request", slog.String("error", err.Error()))
		}
		return nil, clarifyError(err)
	}
	return connect.NewResponse(&v1.GetClarificationRequestResponse{Request: fullClarificationRequestProto(q)}), nil
}

// RequestClarification writes the questions for everything the newest
// record of a posting left unresolved, at the candidate's own request
// (FR-HF-13).
func (h *Hiring) RequestClarification(
	ctx context.Context,
	req *connect.Request[v1.RequestClarificationRequest],
) (*connect.Response[v1.RequestClarificationResponse], error) {
	caller, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := strconv.ParseInt(strings.TrimSpace(req.Msg.GetSubmissionId()), 10, 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid submission_id"))
	}
	s, err := h.users.GetJdSubmission(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("submission not found"))
	}
	if s.Status == "scoring" || s.Status == "generating" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the posting is being scored; ask when that run has finished"))
	}
	runID, attempt, latest, ok, err := h.repo.LatestRecordRun(ctx, id)
	if err != nil {
		h.log.Error("hiring: latest record run", slog.Int64("submission", id), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the record"))
	}
	if !ok {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the posting has no match record to ask about; re-score it first"))
	}
	if attempt != latest {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the newest run of this posting left no record; re-score it first"))
	}
	// A run that is already waiting on the candidate keeps its terms:
	// the new questions join the ones it asked. Any other run has its
	// fit, and questions asked now do not take it away.
	in := hiring.GenerateInput{RunID: runID, All: true, AskedBy: hiring.AskedByCandidate}
	if s.Status == hiring.RunAwaitingCandidate {
		in.Held = true
		expires := time.Now().Add(h.settings.Get(ctx).ClarificationLimit())
		in.ExpiresAt = &expires
	}
	q, added, err := h.repo.GenerateClarification(ctx, in)
	if err != nil {
		if !errors.Is(err, hiring.ErrNothingToAsk) && !errors.Is(err, hiring.ErrClarificationClosed) {
			h.log.Error("hiring: generate clarification", slog.Int64("submission", id), slog.String("error", err.Error()))
		}
		return nil, clarifyError(err)
	}
	if added == 0 {
		return nil, clarifyError(hiring.ErrNothingToAsk)
	}
	h.log.Info("hiring: follow-up questions written at the candidate's request", slog.Int64("submission", id),
		slog.String("request", q.ID), slog.Int("added", added), slog.Int64("admin_id", caller.ID))
	h.events.Emit(ctx, requestEvent(req, "clarification.requested", caller.ID, map[string]any{
		"submission_id": id, "questions": added,
	}))
	return connect.NewResponse(&v1.RequestClarificationResponse{Request: fullClarificationRequestProto(q), Added: int32(added)}), nil
}

// AnswerClarificationItem saves one answer (FR-HF-14).
func (h *Hiring) AnswerClarificationItem(
	ctx context.Context,
	req *connect.Request[v1.AnswerClarificationItemRequest],
) (*connect.Response[v1.AnswerClarificationItemResponse], error) {
	caller, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	in := req.Msg.GetAnswer()
	if in == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("an answer is required"))
	}
	a := hiring.Answer{Status: strings.TrimSpace(in.GetStatus()), FreeText: in.GetFreeText()}
	if len(in.GetValues()) > 0 {
		a.Values = map[string]any{}
		for name, list := range in.GetValues() {
			a.Values[name] = append([]string{}, list.GetValues()...)
		}
	}
	it, err := h.repo.SaveClarificationAnswer(ctx, strings.TrimSpace(req.Msg.GetClarificationItemId()), a, time.Now())
	if err != nil {
		if !errors.Is(err, hiring.ErrNotFound) && !errors.Is(err, hiring.ErrInvalidAnswer) && !errors.Is(err, hiring.ErrClarificationClosed) {
			h.log.Error("hiring: save clarification answer", slog.String("error", err.Error()))
		}
		return nil, clarifyError(err)
	}
	out := &v1.AnswerClarificationItemResponse{Item: clarificationItemProto(it)}
	var submissionID int64
	if q, qErr := h.repo.GetClarificationRequest(ctx, it.RequestID); qErr == nil {
		out.PendingCount = int32(q.Pending())
		submissionID = q.SubmissionID
	}
	// What was answered is in the decision log, written with the answer.
	// The event says only that an answer of a kind was given.
	h.events.Emit(ctx, requestEvent(req, "clarification.answered", caller.ID, map[string]any{
		"submission_id": submissionID, "answer_type": it.AnswerType, "status": it.Answer.Status,
	}))
	return connect.NewResponse(out), nil
}

// SubmitClarificationAnswers closes a questionnaire to answers and
// starts the run it owes (FR-HF-14, FR-HF-22).
func (h *Hiring) SubmitClarificationAnswers(
	ctx context.Context,
	req *connect.Request[v1.SubmitClarificationAnswersRequest],
) (*connect.Response[v1.SubmitClarificationAnswersResponse], error) {
	caller, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	q, err := h.repo.SubmitClarificationRequest(ctx, strings.TrimSpace(req.Msg.GetClarificationRequestId()), time.Now())
	if err != nil {
		if !errors.Is(err, hiring.ErrNotFound) && !errors.Is(err, hiring.ErrInvalidAnswer) && !errors.Is(err, hiring.ErrClarificationClosed) {
			h.log.Error("hiring: submit clarification answers", slog.String("error", err.Error()))
		}
		return nil, clarifyError(err)
	}
	out := &v1.SubmitClarificationAnswersResponse{Request: fullClarificationRequestProto(q)}
	if h.clarify != nil && q.SubmissionID > 0 {
		// Marked before the run starts, as a release does, so the page
		// the candidate returns to already shows it in flight.
		if err := h.users.UpdateJdScoring(ctx, q.SubmissionID, "scoring", nil, ""); err != nil {
			h.log.Warn("hiring: could not mark the posting as scoring", slog.Int64("id", q.SubmissionID), slog.String("error", err.Error()))
		}
		go h.clarify.CloseClarification(context.WithoutCancel(ctx), q, caller.ID)
		out.RunStarted = true
	}
	answered := 0
	for _, it := range q.Items {
		if it.Answer.Status == hiring.ItemAnswered || it.Answer.Status == hiring.ItemNotApplicable {
			answered++
		}
	}
	h.log.Info("hiring: clarification answers submitted", slog.String("request", q.ID), slog.Int64("submission", q.SubmissionID),
		slog.Int("answered", answered), slog.Int("questions", len(q.Items)), slog.Bool("run_started", out.RunStarted))
	h.events.Emit(ctx, requestEvent(req, "clarification.submitted", caller.ID, map[string]any{
		"submission_id": q.SubmissionID, "questions": len(q.Items), "answered": answered, "run_started": out.RunStarted,
	}))
	return connect.NewResponse(out), nil
}

// WithdrawClarificationAnswer takes an answer back (FR-HF-14).
func (h *Hiring) WithdrawClarificationAnswer(
	ctx context.Context,
	req *connect.Request[v1.WithdrawClarificationAnswerRequest],
) (*connect.Response[v1.WithdrawClarificationAnswerResponse], error) {
	caller, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	it, stale, err := h.repo.WithdrawClarificationAnswer(ctx, strings.TrimSpace(req.Msg.GetClarificationItemId()), time.Now())
	if err != nil {
		if !errors.Is(err, hiring.ErrNotFound) && !errors.Is(err, hiring.ErrInvalidAnswer) {
			h.log.Error("hiring: withdraw clarification answer", slog.String("error", err.Error()))
		}
		return nil, clarifyError(err)
	}
	var submissionID int64
	if q, qErr := h.repo.GetClarificationRequest(ctx, it.RequestID); qErr == nil {
		submissionID = q.SubmissionID
	}
	h.log.Info("hiring: clarification answer withdrawn", slog.String("item", it.ID), slog.Int64("stale_judgments", stale))
	h.events.Emit(ctx, requestEvent(req, "clarification.withdrawn", caller.ID, map[string]any{
		"submission_id": submissionID, "stale_judgments": stale,
	}))
	return connect.NewResponse(&v1.WithdrawClarificationAnswerResponse{
		Item: clarificationItemProto(it), StaleJudgments: int32(stale),
	}), nil
}
