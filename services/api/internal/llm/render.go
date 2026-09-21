package llm

import (
	"context"
	"errors"

	"github.com/reh3376/career-site/services/api/internal/sidecar"
)

// SidecarRenderer adapts sidecar.Client to jd.Renderer.
type SidecarRenderer struct {
	Client *sidecar.Client
}

func (s SidecarRenderer) RenderResume(ctx context.Context, resumeJSON, ownerPassword, traceID string) ([]byte, int32, error) {
	if s.Client == nil {
		return nil, 0, errors.New("renderer: sidecar not dialled")
	}
	resp, err := s.Client.RenderResume(ctx, resumeJSON, ownerPassword, traceID)
	if err != nil {
		return nil, 0, err
	}
	return resp.Pdf, resp.Pages, nil
}
