package email

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

type SMTP struct {
	addr string
	auth smtp.Auth
	from string
}

func NewSMTP(host string, port int, user, pass, from string) (*SMTP, error) {
	if host == "" {
		return nil, fmt.Errorf("%w: SMTP_HOST is required", ErrNotConfigured)
	}
	if port == 0 {
		port = 25
	}
	s := &SMTP{
		addr: net.JoinHostPort(host, strconv.Itoa(port)),
		from: from,
	}
	if user != "" {
		s.auth = smtp.PlainAuth("", user, pass, host)
	}
	return s, nil
}

func (s *SMTP) Name() string { return "smtp" }

func (s *SMTP) Send(ctx context.Context, msg Message) error {
	from := msg.From
	if from == "" {
		from = s.from
	}
	body := buildMIME(from, msg)

	// net/smtp's SendMail has no context; wrap in a goroutine with a timeout
	// so a hung SMTP dial does not block the request path.
	done := make(chan error, 1)
	go func() {
		done <- smtp.SendMail(s.addr, s.auth, extractAddr(from), []string{msg.To}, body)
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("smtp send: %w", err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Second):
		return fmt.Errorf("smtp send: timeout")
	}
}

// buildMIME writes a multipart/alternative message so clients that support
// HTML render it and text-only clients still get something readable.
func buildMIME(from string, msg Message) []byte {
	var b strings.Builder
	boundary := "career-site-boundary-2ff7"
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", msg.To)
	fmt.Fprintf(&b, "Subject: %s\r\n", msg.Subject)
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%s\r\n\r\n", boundary)

	fmt.Fprintf(&b, "--%s\r\n", boundary)
	fmt.Fprintf(&b, "Content-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n\r\n", msg.TextBody)

	if msg.HTMLBody != "" {
		fmt.Fprintf(&b, "--%s\r\n", boundary)
		fmt.Fprintf(&b, "Content-Type: text/html; charset=utf-8\r\n\r\n%s\r\n\r\n", msg.HTMLBody)
	}

	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return []byte(b.String())
}

// extractAddr strips a "Name <addr>" wrapper down to just addr.
func extractAddr(s string) string {
	if i := strings.LastIndex(s, "<"); i >= 0 {
		if j := strings.Index(s[i:], ">"); j > 0 {
			return s[i+1 : i+j]
		}
	}
	return s
}
