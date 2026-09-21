// Package sidecar is the API's client for the Python gRPC sidecar. The API
// dials the sidecar over an insecure channel because the sidecar port is
// reachable only on the Compose network (FSD §8.9); network policy is the
// boundary, not TLS.
package sidecar

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	sidecarv1 "github.com/reh3376/career-site/services/api/gen/career/sidecar/v1"
)

type Client struct {
	conn    *grpc.ClientConn
	stub    sidecarv1.SidecarServiceClient
	timeout time.Duration
}

func Dial(addr string, timeout time.Duration) (*Client, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	return &Client{
		conn:    conn,
		stub:    sidecarv1.NewSidecarServiceClient(conn),
		timeout: timeout,
	}, nil
}

func (c *Client) Close() error { return c.conn.Close() }

// Health reports the sidecar's readiness. Callers should treat any non-nil
// error as "sidecar unavailable" and degrade retrieval quality rather than
// failing the request; this is the FSD §8.1 design rule.
func (c *Client) Health(ctx context.Context) (*sidecarv1.HealthResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	return c.stub.Health(ctx, &sidecarv1.HealthRequest{})
}

// Embed returns one embedding per input text, in request order. Uses a
// longer timeout than Health because embedding a batch can take seconds
// (Ollama on CPU) — 60s cap keeps a stuck request from wedging a caller.
func (c *Client) Embed(ctx context.Context, texts []string, purpose sidecarv1.EmbedPurpose) (*sidecarv1.EmbedResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return c.stub.Embed(ctx, &sidecarv1.EmbedRequest{Texts: texts, Purpose: purpose})
}

// Generate runs one chat completion. No client-side timeout beyond the
// caller's ctx: CPU inference of a page of text can take minutes, and
// the background callers bound it themselves.
func (c *Client) Generate(ctx context.Context, req *sidecarv1.GenerateRequest) (*sidecarv1.GenerateResponse, error) {
	return c.stub.Generate(ctx, req)
}
