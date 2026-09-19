// Package handlers implements ConnectRPC service handlers for the API.
package handlers

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/gen/career/v1/careerv1connect"
	"github.com/reh3376/career-site/services/api/internal/build"
)

// System implements careerv1connect.SystemServiceHandler.
type System struct {
	careerv1connect.UnimplementedSystemServiceHandler
}

func NewSystem() *System { return &System{} }

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
