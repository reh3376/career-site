package handlers

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/gen/career/v1/careerv1connect"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Member implements careerv1connect.MemberServiceHandler. Only GetMe +
// GetHistory are live today; the rest fall through to the embedded
// UNIMPLEMENTED stubs.
type Member struct {
	careerv1connect.UnimplementedMemberServiceHandler

	auth  *Auth
	users *users.Repo
}

func NewMember(auth *Auth, repo *users.Repo) *Member {
	return &Member{auth: auth, users: repo}
}

// GetMe resolves the caller's session cookie and returns their Me. No
// session or an invalid one returns Unauthenticated so the client can
// route to /login.
func (m *Member) GetMe(
	ctx context.Context,
	req *connect.Request[v1.GetMeRequest],
) (*connect.Response[v1.GetMeResponse], error) {
	u, err := m.auth.LookupSessionUser(ctx, req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("session lookup failed"))
	}
	if u == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not signed in"))
	}
	return connect.NewResponse(&v1.GetMeResponse{Me: m.auth.buildMe(u)}), nil
}

// GetHistory returns the caller's recent activity, newest first. When
// `kind` is set on the request, filters to that kind server-side.
// Powers the personalised /home "recent activity" panel.
func (m *Member) GetHistory(
	ctx context.Context,
	req *connect.Request[v1.GetHistoryRequest],
) (*connect.Response[v1.GetHistoryResponse], error) {
	u, err := m.auth.LookupSessionUser(ctx, req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("session lookup failed"))
	}
	if u == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not signed in"))
	}
	// The proto's PageRequest carries page_size; treat missing / <=0
	// as the default 50 (matches admin surface).
	limit := 50
	if p := req.Msg.Page; p != nil && p.PageSize > 0 {
		limit = int(p.PageSize)
	}
	events, err := m.users.RecentActivity(ctx, u.ID, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("history lookup failed"))
	}
	// Optional kind filter — cheap client-side pass; the volume per
	// user is small enough that a dedicated query isn't worth it.
	wantKind := activityKindProtoToRepo(req.Msg.Kind)
	out := make([]*v1.ActivityEvent, 0, len(events))
	for i := range events {
		if wantKind != "" && events[i].Kind != wantKind {
			continue
		}
		out = append(out, activityEventRepoToProto(&events[i]))
	}
	return connect.NewResponse(&v1.GetHistoryResponse{
		Events: out,
		Page:   &v1.PageResponse{TotalCount: int32(len(out))},
	}), nil
}
