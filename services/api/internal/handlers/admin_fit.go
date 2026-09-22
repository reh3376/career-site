package handlers

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/jd"
)

// The JD fit bands are the owner's numbers: what score counts as very
// strong / strong / possible / weak. "Strong" is also the résumé gate.
// They live in app_settings and take effect within seconds.

func (a *Admin) GetJdFitBands(
	ctx context.Context,
	req *connect.Request[v1.GetJdFitBandsRequest],
) (*connect.Response[v1.GetJdFitBandsResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	if a.jdScorer == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("JD pipeline not wired"))
	}
	return connect.NewResponse(&v1.GetJdFitBandsResponse{Bands: bandsToProto(a.jdScorer.Bands(ctx))}), nil
}

func (a *Admin) SetJdFitBands(
	ctx context.Context,
	req *connect.Request[v1.SetJdFitBandsRequest],
) (*connect.Response[v1.SetJdFitBandsResponse], error) {
	admin, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	if a.jdScorer == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("JD pipeline not wired"))
	}
	in := req.Msg.GetBands()
	if in == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("bands are required"))
	}
	b := jd.Bands{VeryStrong: in.GetVeryStrong(), Strong: in.GetStrong(), Possible: in.GetPossible(), Weak: in.GetWeak()}
	if err := a.jdScorer.BandsStore().Set(ctx, b, admin.ID); err != nil {
		if errors.Is(err, jd.ErrInvalidBands) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	a.events.Emit(ctx, requestEvent(req, "admin.fit_bands_changed", admin.ID, nil))
	return connect.NewResponse(&v1.SetJdFitBandsResponse{Bands: bandsToProto(b)}), nil
}
