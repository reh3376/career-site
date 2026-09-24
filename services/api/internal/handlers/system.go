// Package handlers implements ConnectRPC service handlers for the API.
package handlers

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/gen/career/v1/careerv1connect"
	"github.com/reh3376/career-site/services/api/internal/build"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// System implements careerv1connect.SystemServiceHandler.
type System struct {
	careerv1connect.UnimplementedSystemServiceHandler
	// users is nil until SetUsers is called; GetReviewerStatus then
	// answers Unavailable rather than pretending to have numbers.
	users *users.Repo
	log   *slog.Logger
}

func NewSystem() *System { return &System{} }

// SetUsers wires the repository the public reviewer status reads from.
func (h *System) SetUsers(log *slog.Logger, repo *users.Repo) {
	h.log, h.users = log, repo
}

// GetReviewerStatus publishes how the reviewer is measuring.
//
// It is public on purpose. The page it feeds claims the reviewer is
// honest about what it cannot evidence, and a claim like that is worth
// less than the numbers it is currently failing on. Everything here is
// already on /admin/gate; what is deliberately not here is anything
// about visitors, members or submissions, because publishing a
// disagreement rate says something about the reviewer and publishing a
// funnel says something about other people.
func (h *System) GetReviewerStatus(
	ctx context.Context,
	_ *connect.Request[v1.GetReviewerStatusRequest],
) (*connect.Response[v1.GetReviewerStatusResponse], error) {
	if h.users == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("metrics not wired"))
	}
	st, err := h.users.ReviewerStatus(ctx)
	if err != nil {
		h.log.Error("GetReviewerStatus failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read the reviewer status"))
	}
	resp := &v1.GetReviewerStatusResponse{
		Graded:            int32(st.Graded),
		Agreed:            int32(st.Agreed),
		AgreementPct:      st.AgreementPct,
		HardDisagreements: int32(st.HardDisagreements),
		TooHarsh:          int32(st.TooHarsh),
		TooGenerous:       int32(st.TooGenerous),
		Postings:          int32(st.Postings),
		PostingsRandom:    int32(st.PostingsRandom),
		Scored:            int32(st.Scored),
		GateCorrect:       int32(st.GateCorrect),
		Inversions:        int32(st.Inversions),
		Margin:            st.Margin,
		Model:             st.Model,
	}
	if st.EvaluatedAt != nil {
		resp.EvaluatedAt = timestamppb.New(*st.EvaluatedAt)
	}
	if b := st.Bands; b != nil {
		resp.Bands = &v1.JdFitBandsPublic{
			VeryStrong: b.VeryStrong, Strong: b.Strong,
			Possible: b.Possible, Weak: b.Weak,
		}
	}
	return connect.NewResponse(resp), nil
}

func (h *System) GetVersion(_ context.Context, _ *connect.Request[v1.GetVersionRequest]) (*connect.Response[v1.GetVersionResponse], error) {
	resp := &v1.GetVersionResponse{
		Version:   build.Version,
		Commit:    build.Commit,
		GoVersion: build.GoVersion(),
	}
	if t := parseBuildTime(build.BuiltAt); t != nil {
		resp.BuiltAt = timestamppb.New(*t)
	}
	return connect.NewResponse(resp), nil
}
