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
