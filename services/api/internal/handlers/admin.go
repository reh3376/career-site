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
	"github.com/reh3376/career-site/services/api/gen/career/v1/careerv1connect"
	"github.com/reh3376/career-site/services/api/internal/db"
	"github.com/reh3376/career-site/services/api/internal/db/adminquery"
	"github.com/reh3376/career-site/services/api/internal/ratelimit"
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
	pool     *db.Pool       // write-capable app pool
	roPool   *db.Pool       // read-only pool used by the /admin/db surface
	// queryLimiter caps how often a single admin can hit RunDbQuery.
	// The read-only role bounds the *effect* of a bad query; this bounds
	// the *rate*, so a compromised admin session (or a stuck client
	// hammering "Run") can't monopolise DB connections. Keyed on
	// admin user_id so a lockout is per-account, not per-IP.
	queryLimiter *ratelimit.Limiter
}

func NewAdmin(log *slog.Logger, repo *users.Repo, auth *Auth, decision *AdminDecision, pool *db.Pool, roPool *db.Pool) *Admin {
	if roPool == nil {
		roPool = pool
	}
	return &Admin{
		log:      log,
		users:    repo,
		auth:     auth,
		decision: decision,
		pool:     pool,
		roPool:   roPool,
		// 20-query burst, refills to 20 across a minute. Enough for
		// interactive exploration; well below what a hung client loop
		// would produce.
		queryLimiter: ratelimit.New(20, 20.0/60.0),
	}
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
// ListDbTables / RunDbQuery — /admin/db surface
// ---------------------------------------------------------------

func (a *Admin) ListDbTables(
	ctx context.Context,
	req *connect.Request[v1.ListDbTablesRequest],
) (*connect.Response[v1.ListDbTablesResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	tables, err := adminquery.ListTables(ctx, a.roPool)
	if err != nil {
		a.log.Error("ListTables failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("list tables failed"))
	}
	out := &v1.ListDbTablesResponse{
		Tables: make([]*v1.DbTable, 0, len(tables)),
	}
	for i := range tables {
		t := &tables[i]
		cols := make([]*v1.DbColumn, 0, len(t.Columns))
		for _, c := range t.Columns {
			cols = append(cols, &v1.DbColumn{
				Name:     c.Name,
				DataType: c.DataType,
				Nullable: c.Nullable,
			})
		}
		out.Tables = append(out.Tables, &v1.DbTable{
			Name:           t.Name,
			Columns:        cols,
			ApproxRowCount: t.ApproxRowCount,
		})
	}
	return connect.NewResponse(out), nil
}

func (a *Admin) RunDbQuery(
	ctx context.Context,
	req *connect.Request[v1.RunDbQueryRequest],
) (*connect.Response[v1.RunDbQueryResponse], error) {
	u, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	// Per-admin rate limit. Blocks a stuck-in-a-loop client and a
	// compromised admin session both — the read-only role means a bad
	// query can't write, but nothing else stops one from hammering
	// the DB. Bucket is keyed on user_id so shared IPs don't collide.
	if a.queryLimiter != nil {
		key := "adminquery:" + strconv.FormatInt(u.ID, 10)
		if ok, retry := a.queryLimiter.Allow(key); !ok {
			a.log.Warn("admin db query rate limited",
				slog.Int64("admin_user_id", u.ID),
				slog.Duration("retry_after", retry),
			)
			return nil, connect.NewError(connect.CodeResourceExhausted,
				errors.New("too many queries; slow down for a moment"))
		}
	}

	// Log every admin-run query with the admin's user_id so the
	// prod logs are an ad-hoc audit trail until we ship a proper
	// query-log table. Never log the SQL body at ERROR level — a
	// caller's typo could leak an email address or the like into a
	// higher-attention log.
	a.log.Info("admin db query",
		slog.Int64("admin_user_id", u.ID),
		slog.String("sql_head", firstN(req.Msg.Sql, 200)),
	)

	res, err := adminquery.Run(ctx, a.roPool, req.Msg.Sql, req.Msg.TimeoutMs)
	if err != nil {
		// The safety-rail rejections (SELECT-only, single-statement,
		// forbidden-token, empty) all come back as plain errors from
		// adminquery.Run. Surface them as InvalidArgument so the UI
		// can render the specific reason inline.
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	protoRows := make([]*v1.DbRow, 0, len(res.Rows))
	for _, row := range res.Rows {
		cells := make([]string, len(row))
		var mask uint64
		for i, v := range row {
			if v == nil {
				mask |= 1 << uint(i)
				cells[i] = ""
				continue
			}
			cells[i] = stringify(v)
		}
		protoRows = append(protoRows, &v1.DbRow{
			Cells:    cells,
			NullMask: mask,
		})
	}

	return connect.NewResponse(&v1.RunDbQueryResponse{
		Columns:     res.Columns,
		ColumnTypes: res.ColumnTypes,
		Rows:        protoRows,
		Truncated:   res.Truncated,
		RowCount:    int32(len(res.Rows)),
		ElapsedMs:   res.ElapsedMs,
	}), nil
}

func stringify(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return fmt.Sprintf("\\x%x", t)
	case time.Time:
		return t.UTC().Format(time.RFC3339Nano)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ---------------------------------------------------------------
// ExtendAccess
// ---------------------------------------------------------------

func (a *Admin) ExtendAccess(
	ctx context.Context,
	req *connect.Request[v1.ExtendAccessRequest],
) (*connect.Response[v1.ExtendAccessResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	id, err := strconv.ParseInt(req.Msg.MemberId, 10, 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid member_id"))
	}
	// Exactly one of extend_days / new_expires_at / permanent must be
	// set. Zero of them is InvalidArgument too — callers should be
	// explicit about intent, and the earlier ambiguity between
	// "leave as-is" and "make permanent" caused a real bug in an
	// early prototype.
	modes := 0
	if req.Msg.ExtendDays > 0 {
		modes++
	}
	if req.Msg.NewExpiresAt != nil {
		modes++
	}
	if req.Msg.Permanent {
		modes++
	}
	if modes != 1 {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("set exactly one of extend_days / new_expires_at / permanent"))
	}
	u, err := a.users.GetByID(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("member not found"))
	}

	var target *time.Time
	switch {
	case req.Msg.Permanent:
		target = nil
	case req.Msg.NewExpiresAt != nil:
		t := req.Msg.NewExpiresAt.AsTime()
		target = &t
	default: // ExtendDays
		base := time.Now().UTC()
		if u.ExpiresAt != nil && u.ExpiresAt.After(base) {
			base = *u.ExpiresAt
		}
		t := base.Add(time.Duration(req.Msg.ExtendDays) * 24 * time.Hour)
		target = &t
	}

	if err := a.users.SetExpiresAt(ctx, u.ID, target); err != nil {
		a.log.Error("SetExpiresAt failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("update failed"))
	}
	u.ExpiresAt = target
	// If the account was expired but the new expires_at is in the
	// future, flip it back to active so the member can sign in again.
	if u.Status == users.StatusExpired && target != nil && target.After(time.Now().UTC()) {
		if err := a.users.SetStatus(ctx, u.ID, users.StatusActive); err == nil {
			u.Status = users.StatusActive
		}
	}
	return connect.NewResponse(&v1.ExtendAccessResponse{
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
// ListSavedQueries / UpsertSavedQuery / DeleteSavedQuery —
// /admin/db saved-queries dropdown
// ---------------------------------------------------------------

func (a *Admin) ListSavedQueries(
	ctx context.Context,
	req *connect.Request[v1.ListSavedQueriesRequest],
) (*connect.Response[v1.ListSavedQueriesResponse], error) {
	u, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	rows, err := a.users.ListSavedQueries(ctx, u.ID)
	if err != nil {
		a.log.Error("ListSavedQueries failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("list failed"))
	}
	out := &v1.ListSavedQueriesResponse{
		Queries: make([]*v1.SavedQuery, 0, len(rows)),
	}
	for i := range rows {
		out.Queries = append(out.Queries, savedQueryRepoToProto(&rows[i]))
	}
	return connect.NewResponse(out), nil
}

func (a *Admin) UpsertSavedQuery(
	ctx context.Context,
	req *connect.Request[v1.UpsertSavedQueryRequest],
) (*connect.Response[v1.UpsertSavedQueryResponse], error) {
	u, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Msg.Name)
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	s, created, err := a.users.UpsertSavedQuery(ctx, u.ID, name, req.Msg.Sql)
	if err != nil {
		a.log.Error("UpsertSavedQuery failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("save failed"))
	}
	return connect.NewResponse(&v1.UpsertSavedQueryResponse{
		Query:   savedQueryRepoToProto(s),
		Created: created,
	}), nil
}

func (a *Admin) DeleteSavedQuery(
	ctx context.Context,
	req *connect.Request[v1.DeleteSavedQueryRequest],
) (*connect.Response[v1.DeleteSavedQueryResponse], error) {
	u, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	id, err := strconv.ParseInt(req.Msg.Id, 10, 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id"))
	}
	if err := a.users.DeleteSavedQuery(ctx, u.ID, id); err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("not found"))
		}
		a.log.Error("DeleteSavedQuery failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("delete failed"))
	}
	return connect.NewResponse(&v1.DeleteSavedQueryResponse{}), nil
}

// ---------------------------------------------------------------
// ListAccessGrants / UpsertAccessGrant / DeleteAccessGrant —
// /admin/access surface
// ---------------------------------------------------------------

func (a *Admin) ListAccessGrants(
	ctx context.Context,
	req *connect.Request[v1.ListAccessGrantsRequest],
) (*connect.Response[v1.ListAccessGrantsResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	res, err := a.users.ListGrants(ctx, users.ListGrantsFilter{Query: req.Msg.Query})
	if err != nil {
		a.log.Error("ListGrants failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("list failed"))
	}
	out := &v1.ListAccessGrantsResponse{
		Grants:       make([]*v1.AccessGrant, 0, len(res.Grants)),
		ActiveCount:  res.ActiveCount,
		ExpiredCount: res.ExpiredCount,
	}
	for i := range res.Grants {
		out.Grants = append(out.Grants, accessGrantRepoToProto(&res.Grants[i]))
	}
	return connect.NewResponse(out), nil
}

func (a *Admin) UpsertAccessGrant(
	ctx context.Context,
	req *connect.Request[v1.UpsertAccessGrantRequest],
) (*connect.Response[v1.UpsertAccessGrantResponse], error) {
	u, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	ttl := grantTTLProtoToRepo(req.Msg.DefaultTtl)
	if ttl == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("default_ttl is required"))
	}
	var expiresAt *time.Time
	if req.Msg.EntryExpiresAt != nil {
		t := req.Msg.EntryExpiresAt.AsTime()
		expiresAt = &t
	}
	adminID := u.ID
	g, created, err := a.users.UpsertGrant(
		ctx,
		req.Msg.Email,
		ttl,
		req.Msg.Notes,
		expiresAt,
		&adminID,
	)
	if err != nil {
		a.log.Error("UpsertGrant failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("save failed"))
	}
	return connect.NewResponse(&v1.UpsertAccessGrantResponse{
		Grant:   accessGrantRepoToProto(g),
		Created: created,
	}), nil
}

func (a *Admin) DeleteAccessGrant(
	ctx context.Context,
	req *connect.Request[v1.DeleteAccessGrantRequest],
) (*connect.Response[v1.DeleteAccessGrantResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	id, err := strconv.ParseInt(req.Msg.Id, 10, 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id"))
	}
	if err := a.users.DeleteGrant(ctx, id); err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("grant not found"))
		}
		a.log.Error("DeleteGrant failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("delete failed"))
	}
	return connect.NewResponse(&v1.DeleteAccessGrantResponse{}), nil
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
		Id:           strconv.FormatInt(u.ID, 10),
		Name:         u.Name,
		Email:        u.Email,
		Organization: u.Organization,
		StatedRole:   u.StatedRole,
		Status:       statusToProto(u.Status),
		Role:         roleToProto(u.Role),
	}
	if !u.CreatedAt.IsZero() {
		me.CreatedAt = timestamppb.New(u.CreatedAt)
	}
	if u.ExpiresAt != nil {
		me.ExpiresAt = timestamppb.New(*u.ExpiresAt)
	}
	me.LastNotificationKind = u.LastNotificationKind
	if u.LastNotificationAt != nil {
		me.LastNotificationAt = timestamppb.New(*u.LastNotificationAt)
	}
	if u.LastNotificationError != nil {
		me.LastNotificationError = *u.LastNotificationError
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

// ---------------------------------------------------------------
// Access-grant mappers
// ---------------------------------------------------------------

func grantTTLProtoToRepo(t v1.GrantTTL) users.GrantTTL {
	switch t {
	case v1.GrantTTL_GRANT_TTL_1D:
		return users.GrantTTL1d
	case v1.GrantTTL_GRANT_TTL_3D:
		return users.GrantTTL3d
	case v1.GrantTTL_GRANT_TTL_7D:
		return users.GrantTTL7d
	case v1.GrantTTL_GRANT_TTL_30D:
		return users.GrantTTL30d
	case v1.GrantTTL_GRANT_TTL_PERMANENT:
		return users.GrantTTLPermanent
	default:
		return ""
	}
}

func grantTTLRepoToProto(t users.GrantTTL) v1.GrantTTL {
	switch t {
	case users.GrantTTL1d:
		return v1.GrantTTL_GRANT_TTL_1D
	case users.GrantTTL3d:
		return v1.GrantTTL_GRANT_TTL_3D
	case users.GrantTTL7d:
		return v1.GrantTTL_GRANT_TTL_7D
	case users.GrantTTL30d:
		return v1.GrantTTL_GRANT_TTL_30D
	case users.GrantTTLPermanent:
		return v1.GrantTTL_GRANT_TTL_PERMANENT
	default:
		return v1.GrantTTL_GRANT_TTL_UNSPECIFIED
	}
}

func savedQueryRepoToProto(s *users.SavedQuery) *v1.SavedQuery {
	return &v1.SavedQuery{
		Id:        strconv.FormatInt(s.ID, 10),
		Name:      s.Name,
		Sql:       s.SQL,
		CreatedAt: timestamppb.New(s.CreatedAt),
		UpdatedAt: timestamppb.New(s.UpdatedAt),
	}
}

func accessGrantRepoToProto(g *users.AccessGrant) *v1.AccessGrant {
	out := &v1.AccessGrant{
		Id:         strconv.FormatInt(g.ID, 10),
		Email:      g.Email,
		DefaultTtl: grantTTLRepoToProto(g.DefaultTTL),
		Notes:      g.Notes,
		CreatedAt:  timestamppb.New(g.CreatedAt),
		UpdatedAt:  timestamppb.New(g.UpdatedAt),
	}
	if g.EntryExpiresAt != nil {
		out.EntryExpiresAt = timestamppb.New(*g.EntryExpiresAt)
	}
	if g.CreatedBy != nil {
		out.CreatedBy = strconv.FormatInt(*g.CreatedBy, 10)
	}
	return out
}
