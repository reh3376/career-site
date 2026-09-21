package ingest

import (
	"context"

	sidecarv1 "github.com/reh3376/career-site/services/api/gen/career/sidecar/v1"
	"github.com/reh3376/career-site/services/api/internal/sidecar"
)

// SidecarEmbed adapts the sidecar.Client to the local EmbedClient
// interface. Keeps `ingest` free of the generated proto types so its
// tests don't need the gRPC stack.
type SidecarEmbed struct {
	Client *sidecar.Client
}

func (s SidecarEmbed) Embed(ctx context.Context, texts []string, purpose EmbedPurpose) ([][]float32, string, error) {
	p := sidecarv1.EmbedPurpose_EMBED_PURPOSE_DOCUMENT
	if purpose == PurposeQuery {
		p = sidecarv1.EmbedPurpose_EMBED_PURPOSE_QUERY
	}
	resp, err := s.Client.Embed(ctx, texts, p)
	if err != nil {
		return nil, "", err
	}
	out := make([][]float32, 0, len(resp.Embeddings))
	for _, e := range resp.Embeddings {
		out = append(out, e.Values)
	}
	return out, resp.Model, nil
}
