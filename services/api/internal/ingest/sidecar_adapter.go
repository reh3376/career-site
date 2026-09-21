package ingest

import (
	"context"

	"github.com/reh3376/career-site/services/api/internal/sidecar"
)

// SidecarEmbed adapts the sidecar.Client to the local EmbedClient
// interface. Keeps `ingest` free of the generated proto types so its
// tests don't need the gRPC stack.
type SidecarEmbed struct {
	Client *sidecar.Client
}

func (s SidecarEmbed) Embed(ctx context.Context, texts []string) ([][]float32, string, error) {
	resp, err := s.Client.Embed(ctx, texts)
	if err != nil {
		return nil, "", err
	}
	out := make([][]float32, 0, len(resp.Embeddings))
	for _, e := range resp.Embeddings {
		out = append(out, e.Values)
	}
	return out, resp.Model, nil
}
