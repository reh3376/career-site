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

// The submission limit is the other owner's number on that page: how
// many postings one member may send in a rolling day. It belongs next
// to the bands because both are judgments about what the box can do,
// and both have to be changeable without a deploy.

func (a *Admin) GetJdSubmissionLimit(
	ctx context.Context,
	req *connect.Request[v1.GetJdSubmissionLimitRequest],
) (*connect.Response[v1.GetJdSubmissionLimitResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	if a.jdLimits == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("JD pipeline not wired"))
	}
	return connect.NewResponse(&v1.GetJdSubmissionLimitResponse{
		Limit:       int32(a.jdLimits.Get(ctx)),
		WindowHours: jdQuotaWindowHours,
	}), nil
}

func (a *Admin) SetJdSubmissionLimit(
	ctx context.Context,
	req *connect.Request[v1.SetJdSubmissionLimitRequest],
) (*connect.Response[v1.SetJdSubmissionLimitResponse], error) {
	admin, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	if a.jdLimits == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("JD pipeline not wired"))
	}
	limit := int(req.Msg.GetLimit())
	if err := a.jdLimits.Set(ctx, limit, admin.ID); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	a.events.Emit(ctx, requestEvent(req, "admin.jd_limit_changed", admin.ID,
		map[string]any{"limit": limit}))
	return connect.NewResponse(&v1.SetJdSubmissionLimitResponse{
		Limit:       int32(limit),
		WindowHours: jdQuotaWindowHours,
	}), nil
}
