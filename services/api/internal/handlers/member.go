package handlers

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/gen/career/v1/careerv1connect"
)

// Member implements careerv1connect.MemberServiceHandler. Only GetMe is
// live today; the rest fall through to the embedded UNIMPLEMENTED stubs.
type Member struct {
	careerv1connect.UnimplementedMemberServiceHandler

	auth *Auth
}

func NewMember(auth *Auth) *Member { return &Member{auth: auth} }

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
