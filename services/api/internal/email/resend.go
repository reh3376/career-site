package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Resend struct {
	apiKey string
	from   string
	client *http.Client
}

const resendEndpoint = "https://api.resend.com/emails"

func NewResend(apiKey, from string) (*Resend, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("%w: RESEND_API_KEY is required", ErrNotConfigured)
	}
	return &Resend{
		apiKey: apiKey,
		from:   from,
		client: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (r *Resend) Name() string { return "resend" }

type resendPayload struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	Text    string `json:"text,omitempty"`
	HTML    string `json:"html,omitempty"`
}

func (r *Resend) Send(ctx context.Context, msg Message) error {
	from := msg.From
	if from == "" {
		from = r.from
	}
	body, err := json.Marshal(resendPayload{
		From:    from,
		To:      msg.To,
		Subject: msg.Subject,
		Text:    msg.TextBody,
		HTML:    msg.HTMLBody,
	})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendEndpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("resend status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
