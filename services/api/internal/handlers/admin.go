package handlers

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/gen/career/v1/careerv1connect"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Admin implements careerv1connect.AdminServiceHandler. Right now
// only the contact-message surface is wired; the rest of the AdminService
// stays on UnimplementedAdminServiceHandler defaults (returning
// codes.Unimplemented, which the frontend hides behind the placeholder
// admin console pages) until each surface lands.
//
// Every method here MUST call requireAdmin() to gate on session +
// admin role. The proto declares AUTH_LEVEL_ADMIN + mfa_fresh, but
// enforcement lives in the handler until the auth interceptor lands.
type Admin struct {
	careerv1connect.UnimplementedAdminServiceHandler

	log      *slog.Logger
	users    *users.Repo
	auth     *Auth
	decision *AdminDecision // reused for ApproveUser / DeclineUser business logic
}

func NewAdmin(log *slog.Logger, repo *users.Repo, auth *Auth, decision *AdminDecision) *Admin {
	return &Admin{log: log, users: repo, auth: auth, decision: decision}
}

// requireAdmin gates a call on session + admin role. Called at the
// top of every RPC that returns admin-only data. The MFA-fresh check
// declared in admin.proto comes later with the TOTP hookup
// (FR-AUTH-11); today we only enforce role.
func requireAdmin[T any](a *Admin, ctx context.Context, req *connect.Request[T]) (*users.User, error) {
	u, err := a.auth.LookupSessionUser(ctx, req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("session lookup failed"))
	}
	if u == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not signed in"))
	}
	if u.Role != users.RoleAdmin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("admin role required"))
	}
	return u, nil
}

// ---------------------------------------------------------------
// ListMembers
// ---------------------------------------------------------------

func (a *Admin) ListMembers(
	ctx context.Context,
	req *connect.Request[v1.ListMembersRequest],
) (*connect.Response[v1.ListMembersResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	msg := req.Msg
	f := users.ListMembersFilter{
		Query:  msg.Query,
		Status: memberStatusProtoToRepo(msg.Status),
	}
	if p := msg.Page; p != nil {
		f.Limit = p.PageSize
	}
	res, err := a.users.ListMembers(ctx, f)
	if err != nil {
		a.log.Error("ListMembers failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("list failed"))
	}
	out := &v1.ListMembersResponse{
		Members: make([]*v1.MemberRecord, 0, len(res.Members)),
		Page: &v1.PageResponse{
			TotalCount: res.Total,
		},
	}
	for i := range res.Members {
		out.Members = append(out.Members, memberRecordRepoToProto(&res.Members[i]))
	}
	return connect.NewResponse(out), nil
}

// ---------------------------------------------------------------
// GetMember / SetMemberStatus
// ---------------------------------------------------------------

func (a *Admin) GetMember(
	ctx context.Context,
	req *connect.Request[v1.GetMemberRequest],
) (*connect.Response[v1.GetMemberResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	id, err := strconv.ParseInt(req.Msg.MemberId, 10, 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid member_id"))
	}
	u, err := a.users.GetByID(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("member not found"))
	}
	// Activity events + conversations + notes come online with the
	// activity ingest work (Phase 3+). For now the detail page has
	// enough for status-transition actions on the member itself.
	return connect.NewResponse(&v1.GetMemberResponse{
		Member: memberRecordRepoToProto(u),
	}), nil
}

func (a *Admin) SetMemberStatus(
	ctx context.Context,
	req *connect.Request[v1.SetMemberStatusRequest],
) (*connect.Response[v1.SetMemberStatusResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	id, err := strconv.ParseInt(req.Msg.MemberId, 10, 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid member_id"))
	}
	u, err := a.users.GetByID(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("member not found"))
	}
	// The proto validation locks status to ACTIVE (2) or DISABLED (4);
	// the map handles both plus a fallthrough that returns InvalidArgument
	// so a rare wire-level bypass can't set a state the DB wouldn't
	// accept anyway.
	target := memberStatusProtoToRepo(req.Msg.Status)
	if target != users.StatusActive && target != users.StatusDisabled {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("status must be ACTIVE or DISABLED"))
	}
	if err := a.users.SetStatus(ctx, u.ID, target); err != nil {
		a.log.Error("SetStatus failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("update failed"))
	}
	u.Status = target
	return connect.NewResponse(&v1.SetMemberStatusResponse{
		Member: memberRecordRepoToProto(u),
	}), nil
}

// ---------------------------------------------------------------
// ApproveRegistration / DeclineRegistration
// ---------------------------------------------------------------

func (a *Admin) ApproveRegistration(
	ctx context.Context,
	req *connect.Request[v1.ApproveRegistrationRequest],
) (*connect.Response[v1.ApproveRegistrationResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	id, err := strconv.ParseInt(req.Msg.MemberId, 10, 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid member_id"))
	}
	u, err := a.users.GetByID(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("member not found"))
	}
	if u.Status != users.StatusPendingApproval {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("member is not in pending_approval"))
	}
	if _, err := a.decision.ApproveUser(ctx, u, "console", nil); err != nil {
		a.log.Error("ApproveUser failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("approve failed"))
	}
	return connect.NewResponse(&v1.ApproveRegistrationResponse{
		Member: memberRecordRepoToProto(u),
	}), nil
}

func (a *Admin) DeclineRegistration(
	ctx context.Context,
	req *connect.Request[v1.DeclineRegistrationRequest],
) (*connect.Response[v1.DeclineRegistrationResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	id, err := strconv.ParseInt(req.Msg.MemberId, 10, 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid member_id"))
	}
	u, err := a.users.GetByID(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("member not found"))
	}
	if u.Status != users.StatusPendingApproval {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("member is not in pending_approval"))
	}
	if err := a.decision.DeclineUser(ctx, u, "console", nil); err != nil {
		a.log.Error("DeclineUser failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("decline failed"))
	}
	return connect.NewResponse(&v1.DeclineRegistrationResponse{
		Member: memberRecordRepoToProto(u),
	}), nil
}

// ---------------------------------------------------------------
// ListContactMessages
// ---------------------------------------------------------------

func (a *Admin) ListContactMessages(
	ctx context.Context,
	req *connect.Request[v1.ListContactMessagesRequest],
) (*connect.Response[v1.ListContactMessagesResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	msg := req.Msg
	f := users.ListSupportFilter{
		Query:    msg.Query,
		Category: supportCategoryProtoToRepo(msg.Category),
		Status:   supportStatusProtoToRepo(msg.Status),
	}
	if p := msg.Page; p != nil {
		f.Limit = p.PageSize
		// Cursor-based paging isn't wired here yet; page_token becomes
		// an offset later. For now we serve the first page only.
		f.Offset = 0
	}
	res, err := a.users.ListSupport(ctx, f)
	if err != nil {
		a.log.Error("ListSupport failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("list failed"))
	}
	out := &v1.ListContactMessagesResponse{
		Messages:      make([]*v1.SupportMessage, 0, len(res.Messages)),
		OpenCount:     res.OpenCount,
		ResolvedCount: res.ResolvedCount,
		Page: &v1.PageResponse{
			TotalCount: res.Total,
		},
	}
	for i := range res.Messages {
		out.Messages = append(out.Messages, supportMessageRepoToProto(&res.Messages[i]))
	}
	return connect.NewResponse(out), nil
}

// ---------------------------------------------------------------
// ResolveContactMessage
// ---------------------------------------------------------------

func (a *Admin) ResolveContactMessage(
	ctx context.Context,
	req *connect.Request[v1.ResolveContactMessageRequest],
) (*connect.Response[v1.ResolveContactMessageResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	id, err := strconv.ParseInt(req.Msg.Id, 10, 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id"))
	}
	status := supportStatusProtoToRepo(req.Msg.Status)
	if status == "" {
		status = "resolved"
	}
	m, err := a.users.SetSupportStatus(ctx, id, status)
	if err != nil {
		a.log.Error("SetSupportStatus failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("update failed"))
	}
	return connect.NewResponse(&v1.ResolveContactMessageResponse{
		Message: supportMessageRepoToProto(m),
	}), nil
}

// ---------------------------------------------------------------
// helpers — proto <-> repo enum mapping
// ---------------------------------------------------------------

func supportCategoryProtoToRepo(c v1.SupportCategory) users.SupportCategory {
	switch c {
	case v1.SupportCategory_SUPPORT_CATEGORY_GENERAL_QUESTION:
		return users.SupportCategoryGeneralQuestion
	case v1.SupportCategory_SUPPORT_CATEGORY_BUG_REPORT:
		return users.SupportCategoryBugReport
	case v1.SupportCategory_SUPPORT_CATEGORY_FEATURE_REQUEST:
		return users.SupportCategoryFeatureRequest
	case v1.SupportCategory_SUPPORT_CATEGORY_CONTRIBUTOR_ACCESS:
		return users.SupportCategoryContributorAccess
	case v1.SupportCategory_SUPPORT_CATEGORY_PRESS_INQUIRY:
		return users.SupportCategoryPressInquiry
	case v1.SupportCategory_SUPPORT_CATEGORY_OTHER:
		return users.SupportCategoryOther
	default:
		return "" // unspecified = "any"
	}
}

func supportCategoryRepoToProto(c users.SupportCategory) v1.SupportCategory {
	switch c {
	case users.SupportCategoryGeneralQuestion:
		return v1.SupportCategory_SUPPORT_CATEGORY_GENERAL_QUESTION
	case users.SupportCategoryBugReport:
		return v1.SupportCategory_SUPPORT_CATEGORY_BUG_REPORT
	case users.SupportCategoryFeatureRequest:
		return v1.SupportCategory_SUPPORT_CATEGORY_FEATURE_REQUEST
	case users.SupportCategoryContributorAccess:
		return v1.SupportCategory_SUPPORT_CATEGORY_CONTRIBUTOR_ACCESS
	case users.SupportCategoryPressInquiry:
		return v1.SupportCategory_SUPPORT_CATEGORY_PRESS_INQUIRY
	case users.SupportCategoryOther:
		return v1.SupportCategory_SUPPORT_CATEGORY_OTHER
	default:
		return v1.SupportCategory_SUPPORT_CATEGORY_UNSPECIFIED
	}
}

func supportStatusProtoToRepo(s v1.SupportStatus) string {
	switch s {
	case v1.SupportStatus_SUPPORT_STATUS_OPEN:
		return "open"
	case v1.SupportStatus_SUPPORT_STATUS_RESOLVED:
		return "resolved"
	default:
		return "" // unspecified = "any" for list, defaulted to "resolved" for update
	}
}

func supportStatusRepoToProto(s string) v1.SupportStatus {
	switch s {
	case "open":
		return v1.SupportStatus_SUPPORT_STATUS_OPEN
	case "resolved":
		return v1.SupportStatus_SUPPORT_STATUS_RESOLVED
	default:
		return v1.SupportStatus_SUPPORT_STATUS_UNSPECIFIED
	}
}

// ---------------------------------------------------------------
// Member mappers
// ---------------------------------------------------------------

func memberStatusProtoToRepo(s v1.MemberStatus) users.Status {
	switch s {
	case v1.MemberStatus_MEMBER_STATUS_UNVERIFIED:
		return users.StatusUnverified
	case v1.MemberStatus_MEMBER_STATUS_PENDING_APPROVAL:
		return users.StatusPendingApproval
	case v1.MemberStatus_MEMBER_STATUS_ACTIVE:
		return users.StatusActive
	case v1.MemberStatus_MEMBER_STATUS_DECLINED:
		return users.StatusDeclined
	case v1.MemberStatus_MEMBER_STATUS_EXPIRED:
		return users.StatusExpired
	case v1.MemberStatus_MEMBER_STATUS_DISABLED:
		return users.StatusDisabled
	default:
		return "" // any
	}
}

func memberRecordRepoToProto(u *users.User) *v1.MemberRecord {
	me := &v1.Me{
		Id:     strconv.FormatInt(u.ID, 10),
		Name:   u.Name,
		Email:  u.Email,
		Status: statusToProto(u.Status),
		Role:   roleToProto(u.Role),
	}
	rec := &v1.MemberRecord{
		Me:     me,
		Counts: &v1.MemberCounts{}, // activity counts land with the activity ingest work
	}
	if !u.CreatedAt.IsZero() {
		rec.FirstSeenAt = timestamppb.New(u.CreatedAt)
	}
	return rec
}

// ---------------------------------------------------------------
// Support message mapper
// ---------------------------------------------------------------

func supportMessageRepoToProto(m *users.SupportMessage) *v1.SupportMessage {
	out := &v1.SupportMessage{
		Id:          strconv.FormatInt(m.ID, 10),
		TicketId:    m.TicketID,
		Category:    supportCategoryRepoToProto(m.Category),
		Status:      supportStatusRepoToProto(m.Status),
		Subject:     m.Subject,
		Body:        m.Body,
		SenderName:  m.SenderName,
		SenderEmail: m.SenderEmail,
		CreatedAt:   timestamppb.New(m.CreatedAt),
		UpdatedAt:   timestamppb.New(m.UpdatedAt),
	}
	if m.UserID != nil {
		out.UserId = strconv.FormatInt(*m.UserID, 10)
	}
	if m.ResolvedAt != nil {
		out.ResolvedAt = timestamppb.New(*m.ResolvedAt)
	}
	return out
}
