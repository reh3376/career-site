package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Decision log: the owner's human-in-the-loop review of what the JD
// reviewer decided. See docs/decision-log.md.

// ListDecisionLog returns logged decisions for /admin/decisions.
func (a *Admin) ListDecisionLog(
	ctx context.Context,
	req *connect.Request[v1.ListDecisionLogRequest],
) (*connect.Response[v1.ListDecisionLogResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	f := users.DecisionFilter{
		Kind:           strings.TrimSpace(req.Msg.GetKind()),
		UnreviewedOnly: req.Msg.GetUnreviewedOnly(),
		Limit:          int(req.Msg.GetLimit()),
	}
	if s := strings.TrimSpace(req.Msg.GetRefId()); s != "" {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("ref_id must be numeric"))
		}
		f.RefID = id
	}
	rows, err := a.users.ListDecisions(ctx, f)
	if err != nil {
		a.log.Error("ListDecisionLog failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("list failed"))
	}
	total, reviewed, err := a.users.CountDecisions(ctx)
	if err != nil {
		a.log.Error("CountDecisions failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("count failed"))
	}
	out := &v1.ListDecisionLogResponse{
		Decisions:     make([]*v1.DecisionLogRow, 0, len(rows)),
		TotalCount:    total,
		ReviewedCount: reviewed,
	}
	for i := range rows {
		out.Decisions = append(out.Decisions, decisionToProto(&rows[i]))
	}
	return connect.NewResponse(out), nil
}

// ReviewDecision stores the owner's label on one row.
func (a *Admin) ReviewDecision(
	ctx context.Context,
	req *connect.Request[v1.ReviewDecisionRequest],
) (*connect.Response[v1.ReviewDecisionResponse], error) {
	me, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	id, err := strconv.ParseInt(strings.TrimSpace(req.Msg.GetId()), 10, 64)
	if err != nil || id <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must be numeric"))
	}
	verdict := strings.ToLower(strings.TrimSpace(req.Msg.GetHumanVerdict()))
	switch verdict {
	// insufficient_evidence applies to both kinds: it is the reviewer
	// saying they could not judge this from what they were shown, which
	// is neither agreement nor disagreement with the model. Without it,
	// an ungradeable row forces a wrong label, and a wrong label is
	// worse than no label because the agreement rate then counts it.
	// It is also the most actionable grade there is: a pile of them
	// means retrieval is not putting the right documents in front of
	// the judge, which no prompt change will fix.
	case "met", "partial", "unmet", "above_threshold", "below_threshold", "insufficient_evidence":
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("human_verdict must be one of met, partial, unmet (verdicts), above_threshold, below_threshold (gate), or insufficient_evidence (either)"))
	}
	note := strings.TrimSpace(req.Msg.GetHumanNote())
	if len(note) > 4000 {
		note = note[:4000]
	}
	if err := a.users.ReviewDecision(ctx, id, me.ID, verdict, note); err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("decision not found"))
		}
		a.log.Error("ReviewDecision failed", slog.Int64("id", id), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("review failed"))
	}
	a.events.Emit(ctx, requestEvent(req, "admin.decision_reviewed", me.ID, map[string]any{"decision_id": id, "verdict": verdict}))
	return connect.NewResponse(&v1.ReviewDecisionResponse{}), nil
}

// ExportDecisionLog renders the log as JSON Lines for training.
func (a *Admin) ExportDecisionLog(
	ctx context.Context,
	req *connect.Request[v1.ExportDecisionLogRequest],
) (*connect.Response[v1.ExportDecisionLogResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	rows, err := a.users.ExportDecisions(ctx, req.Msg.GetReviewedOnly())
	if err != nil {
		a.log.Error("ExportDecisionLog failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("export failed"))
	}
	var b strings.Builder
	for i := range rows {
		line, err := decisionExportLine(&rows[i])
		if err != nil {
			a.log.Warn("decision export line skipped", slog.Int64("id", rows[i].ID), slog.String("error", err.Error()))
			continue
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return connect.NewResponse(&v1.ExportDecisionLogResponse{
		Jsonl:    b.String(),
		RowCount: int32(len(rows)),
	}), nil
}

func decisionToProto(d *users.Decision) *v1.DecisionLogRow {
	row := &v1.DecisionLogRow{
		Id:               strconv.FormatInt(d.ID, 10),
		Kind:             d.Kind,
		RefKind:          d.RefKind,
		RefId:            strconv.FormatInt(d.RefID, 10),
		Key:              d.Key,
		Model:            d.Model,
		PromptId:         d.PromptID,
		PromptVersion:    int32(d.PromptVersion),
		NumCtx:           int32(d.NumCtx),
		InputJson:        string(d.Input),
		OutputJson:       string(d.Output),
		PromptText:       d.PromptText,
		ResponseText:     d.ResponseText,
		PromptTokens:     d.PromptTokens,
		CompletionTokens: d.CompletionTok,
		LatencyMs:        d.LatencyMs,
		CreatedAt:        timestamppb.New(d.CreatedAt),
		HumanVerdict:     d.HumanVerdict,
		HumanNote:        d.HumanNote,
	}
	if d.ReviewedBy != nil {
		row.ReviewedBy = strconv.FormatInt(*d.ReviewedBy, 10)
	}
	if d.ReviewedAt != nil {
		row.ReviewedAt = timestamppb.New(*d.ReviewedAt)
	}
	return row
}

// decisionExportLine is the training-export shape documented in
// docs/decision-log.md. input/output are embedded as JSON, not strings.
func decisionExportLine(d *users.Decision) ([]byte, error) {
	type ref struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
		Key  string `json:"key,omitempty"`
	}
	type prompt struct {
		ID      string `json:"id,omitempty"`
		Version int    `json:"version,omitempty"`
		NumCtx  int    `json:"num_ctx,omitempty"`
	}
	type human struct {
		Verdict    string `json:"verdict"`
		Note       string `json:"note,omitempty"`
		ReviewedAt string `json:"reviewed_at"`
	}
	line := map[string]any{
		"id":            strconv.FormatInt(d.ID, 10),
		"kind":          d.Kind,
		"ref":           ref{Kind: d.RefKind, ID: strconv.FormatInt(d.RefID, 10), Key: d.Key},
		"model":         d.Model,
		"prompt":        prompt{ID: d.PromptID, Version: d.PromptVersion, NumCtx: d.NumCtx},
		"input":         json.RawMessage(nonEmptyJSON(d.Input)),
		"output":        json.RawMessage(nonEmptyJSON(d.Output)),
		"prompt_text":   d.PromptText,
		"response_text": d.ResponseText,
		"usage": map[string]any{
			"prompt_tokens": d.PromptTokens, "completion_tokens": d.CompletionTok, "latency_ms": d.LatencyMs,
		},
		"created_at": d.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if d.ReviewedAt != nil {
		line["human"] = human{Verdict: d.HumanVerdict, Note: d.HumanNote, ReviewedAt: d.ReviewedAt.UTC().Format("2006-01-02T15:04:05Z07:00")}
	} else {
		line["human"] = nil
	}
	return json.Marshal(line)
}

func nonEmptyJSON(b []byte) []byte {
	if len(b) == 0 {
		return []byte(`{}`)
	}
	return b
}
