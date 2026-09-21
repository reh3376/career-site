package email

import (
	"context"
	"log/slog"
	"time"
)

const TriggeredBySystem = "system"

// Delivery is one send attempt, success or failure. Error is empty
// when the provider accepted the message.
type Delivery struct {
	UserID      int64 // 0 → not tied to a member
	Kind        string
	Recipient   string
	Provider    string
	TriggeredBy string
	DurationMs  int64
	Error       string
}

// DeliverySink persists a Delivery. Implemented by users.Repo.
type DeliverySink interface {
	RecordDelivery(ctx context.Context, d Delivery) error
}

// Audited wraps a Provider so every Send lands in the sink whether it
// succeeded or not. The send result is returned unchanged; a sink
// failure is logged and swallowed because the mail has already gone
// (or already failed) and the audit row is not worth failing the
// caller over.
type Audited struct {
	Inner Provider
	Sink  DeliverySink
	Log   *slog.Logger
}

func (a Audited) Name() string { return a.Inner.Name() }

func (a Audited) Send(ctx context.Context, msg Message) error {
	start := time.Now()
	sendErr := a.Inner.Send(ctx, msg)

	d := Delivery{
		UserID:      msg.UserID,
		Kind:        msg.Kind,
		Recipient:   msg.To,
		Provider:    a.Inner.Name(),
		TriggeredBy: msg.TriggeredBy,
		DurationMs:  time.Since(start).Milliseconds(),
	}
	if d.TriggeredBy == "" {
		d.TriggeredBy = TriggeredBySystem
	}
	if d.Kind == "" {
		d.Kind = "unknown"
	}
	if sendErr != nil {
		d.Error = truncate(sendErr.Error(), 500)
	}
	// The caller's ctx may be nearly spent after a slow provider; give
	// the audit write its own short budget so it doesn't get cancelled
	// alongside a timed-out send.
	auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := a.Sink.RecordDelivery(auditCtx, d); err != nil && a.Log != nil {
		a.Log.Warn("record delivery failed",
			slog.String("kind", d.Kind), slog.Int64("user_id", d.UserID),
			slog.String("error", err.Error()))
	}
	return sendErr
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
