// Package llm is the API's gateway to text generation. The sidecar owns
// the provider (Ollama locally / on-host, a hosted API later, stub in
// CI); this package owns the request shape the rest of the API codes
// against, so callers never import the generated sidecar types.
package llm

import (
	"context"
	"errors"

	sidecarv1 "github.com/reh3376/career-site/services/api/gen/career/sidecar/v1"
	"github.com/reh3376/career-site/services/api/internal/sidecar"
)

// Request is one completion. System and User are already-rendered
// prompt text from the prompts registry; untrusted content inside User
// must be delimited by the caller.
type Request struct {
	System      string
	User        string
	MaxTokens   int
	Temperature float32
	JSON        bool
	JSONSchema  string // implies JSON; enforced by providers that support constrained decoding
	TraceID     string
}

// Response mirrors the sidecar's GenerateResponse.
type Response struct {
	Text             string
	Model            string
	PromptTokens     int32
	CompletionTokens int32
	LatencyMs        int64
	FinishReason     string
}

// Client is what generation callers depend on; a fake satisfies it in
// tests without the gRPC stack.
type Client interface {
	Generate(ctx context.Context, req Request) (*Response, error)
}

// SidecarLLM adapts sidecar.Client to Client.
type SidecarLLM struct {
	Client *sidecar.Client
}

func (s SidecarLLM) Generate(ctx context.Context, req Request) (*Response, error) {
	if s.Client == nil {
		return nil, errors.New("llm: sidecar not dialled")
	}
	resp, err := s.Client.Generate(ctx, &sidecarv1.GenerateRequest{
		System:      req.System,
		User:        req.User,
		MaxTokens:   int32(req.MaxTokens),
		Temperature: req.Temperature,
		Json:        req.JSON || req.JSONSchema != "",
		JsonSchema:  req.JSONSchema,
		TraceId:     req.TraceID,
	})
	if err != nil {
		return nil, err
	}
	return &Response{
		Text:             resp.Text,
		Model:            resp.Model,
		PromptTokens:     resp.PromptTokens,
		CompletionTokens: resp.CompletionTokens,
		LatencyMs:        resp.LatencyMs,
		FinishReason:     resp.FinishReason,
	}, nil
}
