package handlers

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
)

// SetMemberPilotApproved opens or closes the pilot area to one member
// (FSD FR-PLT-01, FR-PLT-02). The flag is normally copied from the
// whitelist entry at auto-approval; this is the owner's override on
// the member page, in either direction.
func (a *Admin) SetMemberPilotApproved(
	ctx context.Context,
	req *connect.Request[v1.SetMemberPilotApprovedRequest],
) (*connect.Response[v1.SetMemberPilotApprovedResponse], error) {
	id, err := strconv.ParseInt(req.Msg.MemberId, 10, 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid member_id"))
	}
	u, err := a.users.GetByID(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("member not found"))
	}
	if err := a.users.SetPilotApproved(ctx, u.ID, req.Msg.Approved); err != nil {
		a.log.Error("SetPilotApproved failed", slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("update failed"))
	}
	u.PilotApproved = req.Msg.Approved
	a.events.Emit(ctx, requestEvent(req, "pilot.flag_changed", u.ID,
		map[string]any{"user_id": u.ID, "approved": req.Msg.Approved}))
	return connect.NewResponse(&v1.SetMemberPilotApprovedResponse{
		Member: memberRecordRepoToProto(u),
	}), nil
}
