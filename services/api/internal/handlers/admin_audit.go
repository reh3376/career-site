package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// GetAudit lists audit_log entries (FR-ADM-09, FR-ADM-11).
//
// The RPC and its messages have been in the contract since the first
// proto, with the comment "Lists audit-log entries", and had no handler
// at all: the call returned UNIMPLEMENTED from the embedded default
// while two requirement rows cited it as the record of admin actions.
// It is implemented here at the same time as the writer, because a read
// surface arriving later is how a write-only table becomes the next
// control nobody checks.
func (a *Admin) GetAudit(
	ctx context.Context,
	req *connect.Request[v1.GetAuditRequest],
) (*connect.Response[v1.GetAuditResponse], error) {
	msg := req.Msg

	q := users.AuditQuery{
		ActionPrefix: msg.ActionPrefix,
		Limit:        int(msg.GetPage().GetPageSize()),
	}
	if msg.ActorId != "" {
		id, err := strconv.ParseInt(msg.ActorId, 10, 64)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				errors.New("actor_id must be a member id"))
		}
		q.ActorUserID = &id
	}
	if msg.From != nil {
		t := msg.From.AsTime()
		q.From = &t
	}
	if msg.To != nil {
		t := msg.To.AsTime()
		q.To = &t
	}
	// page_token is an offset here rather than an opaque cursor. The log
	// is append-only and read newest-first by a single admin, so the
	// drift a cursor protects against (rows inserted mid-pagination
	// shifting the window) is one admin's own actions appearing while
	// they page, which is harmless and arguably correct.
	if tok := msg.GetPage().GetPageToken(); tok != "" {
		off, err := strconv.Atoi(tok)
		if err != nil || off < 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				errors.New("page_token is not a valid offset"))
		}
		q.Offset = off
	}

	rows, total, err := a.users.ListAuditEntries(ctx, q)
	if err != nil {
		a.log.Error("ListAuditEntries failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("list failed"))
	}

	out := &v1.GetAuditResponse{
		Entries: make([]*v1.AuditEntry, 0, len(rows)),
		Page:    &v1.PageResponse{TotalCount: int32(total)},
	}
	for _, r := range rows {
		e := &v1.AuditEntry{
			Id:         strconv.FormatInt(r.ID, 10),
			Action:     r.Action,
			TargetType: r.TargetType,
			TargetId:   r.TargetID,
			ActorName:  r.ActorName,
			OccurredAt: timestamppb.New(r.OccurredAt.UTC()),
		}
		if r.ActorUserID != nil {
			e.ActorId = strconv.FormatInt(*r.ActorUserID, 10)
		}
		if len(r.Payload) > 0 {
			if blob, mErr := json.Marshal(r.Payload); mErr == nil {
				e.PayloadJson = string(blob)
			}
			// outcome is stored inside the payload rather than in its own
			// column, so the table needed no migration. It is lifted onto
			// its own field here because every row has one and a reader
			// should not have to parse JSON to see whether the action
			// succeeded.
			if v, ok := r.Payload["outcome"].(string); ok {
				e.Outcome = v
			}
		}
		out.Entries = append(out.Entries, e)
	}
	if q.Offset+len(rows) < total {
		out.Page.NextPageToken = strconv.Itoa(q.Offset + len(rows))
	}
	return connect.NewResponse(out), nil
}
