package handlers

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"strings"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/auth"
	"github.com/reh3376/career-site/services/api/internal/email"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Forgot-password by emailed code (FSD FR-AUTH-19).
//
// Until 2026-11-08 the email carried a single-use link valid for an
// hour. The owner replaced it with a 5-digit code the member types
// beside the new password. A code that short has 100,000 values, so
// three things bound it: it lives fifteen minutes, five wrong attempts
// burn it, and the per-(ip, email) limiter that guards sign-in guards
// this too. The response to the address step is identical for known
// and unknown addresses, as it always was (FR-AUTH-19's enumeration
// rule carries over).

const (
	// resetCodeDigits is the owner's choice: short enough to type from
	// a phone while reading it on a laptop.
	resetCodeDigits = 5
	// resetCodeMaxAttempts: the fifth wrong code burns every live code
	// for the account, and the member asks for a new one.
	resetCodeMaxAttempts = 5
)

// badResetCode is the one answer for a wrong code, a wrong address, and
// an account that does not exist, so none of them is distinguishable.
var badResetCode = connect.NewError(connect.CodeInvalidArgument,
	errors.New("that email and code do not match; check the email we sent and try again"))

// ForgotPassword mints a reset code, emails it, and returns a fixed
// message whether or not the address is known.
func (h *Auth) ForgotPassword(
	ctx context.Context,
	req *connect.Request[v1.ForgotPasswordRequest],
) (*connect.Response[v1.ForgotPasswordResponse], error) {
	addr := strings.ToLower(strings.TrimSpace(req.Msg.Email))
	if addr == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("email is required"))
	}

	// Reuse the login limiter: a single (ip, email) tuple gets at most
	// 5 attempts per 15 min, shared with sign-in so an attacker cannot
	// split a budget between the two flows on the same target.
	if h.loginLimiter != nil {
		key := "reset:" + ClientIP(req) + "|" + addr
		if ok, retry := h.loginLimiter.Allow(key); !ok {
			h.log.Warn("reset rate limited", slog.String("email", addr), slog.Duration("retry_after", retry))
			return connect.NewResponse(&v1.ForgotPasswordResponse{Message: fixedResetMessage()}), nil
		}
	}

	u, err := h.users.GetByEmail(ctx, addr)
	if err == nil {
		if sendErr := h.sendPasswordResetCode(ctx, u); sendErr != nil {
			h.log.Warn("send password_reset failed",
				slog.Int64("user_id", u.ID), slog.String("error", sendErr.Error()))
		}
	} else {
		h.log.Info("reset requested for unknown email", slog.String("email", addr))
	}

	return connect.NewResponse(&v1.ForgotPasswordResponse{Message: fixedResetMessage()}), nil
}

// fixedResetMessage is the exact response the caller sees whether
// their email matched an account or not.
func fixedResetMessage() string {
	return "If an account exists for that email, we've sent a 5-digit reset code. Enter it with your new password within 15 minutes."
}

func (h *Auth) sendPasswordResetCode(ctx context.Context, u *users.User) error {
	code, codeHash, err := auth.NewDigitCode(resetCodeDigits)
	if err != nil {
		return err
	}
	// email_tokens.token_hash is NOT NULL UNIQUE from the link era. The
	// link is gone and nothing reads this value; a random token fills
	// the column so the constraint holds.
	_, tokenHash, err := auth.NewToken()
	if err != nil {
		return err
	}
	if _, err := h.users.CreateToken(ctx, u.ID, users.PurposeReset, tokenHash, codeHash, h.cfg.ResetCodeTTL); err != nil {
		return err
	}

	text, htmlBody, err := email.PasswordResetTemplate.Render(map[string]any{
		"Name":       u.Name,
		"Code":       code,
		"Minutes":    int(h.cfg.ResetCodeTTL.Minutes()),
		"Attempts":   resetCodeMaxAttempts,
		"OwnerEmail": h.cfg.OwnerContactEmail,
	})
	if err != nil {
		return err
	}
	return h.email.Send(ctx, email.Message{
		To:       u.Email,
		From:     h.cfg.MailFrom,
		Subject:  "Your career-site reset code",
		TextBody: text,
		HTMLBody: htmlBody,
		Kind:     "password_reset",
		UserID:   u.ID,
	})
}

// ResetPassword checks the emailed code against the account's live
// reset codes, then sets the new password, revokes every existing
// session and signs the member in on the same page.
//
// Order matters: the code is matched first but consumed only after the
// new password has passed the breach check, so a rejected password does
// not cost the member the code they just typed. A wrong code counts
// against every live code for the account; the fifth burns them.
func (h *Auth) ResetPassword(
	ctx context.Context,
	req *connect.Request[v1.ResetPasswordRequest],
) (*connect.Response[v1.ResetPasswordResponse], error) {
	msg := req.Msg
	addr := strings.ToLower(strings.TrimSpace(msg.Email))
	code := strings.TrimSpace(msg.Code)
	if addr == "" || code == "" || msg.NewPassword == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("email, code and new_password are required"))
	}
	if !isDigits(code, resetCodeDigits) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("the code is the 5 digits from the email"))
	}

	// The same bucket as sign-in and the address step, so guessing codes
	// is bounded by the limiter before it is bounded by the attempt cap.
	if h.loginLimiter != nil {
		key := "resetcode:" + ClientIP(req) + "|" + addr
		if ok, retry := h.loginLimiter.Allow(key); !ok {
			h.log.Warn("reset code rate limited", slog.String("email", addr), slog.Duration("retry_after", retry))
			return nil, connect.NewError(connect.CodeResourceExhausted,
				errors.New("too many attempts; try again shortly"))
		}
	}

	u, err := h.users.GetByEmail(ctx, addr)
	if err != nil {
		return nil, badResetCode
	}
	live, err := h.users.LiveResetTokens(ctx, u.ID)
	if err != nil {
		h.log.Error("live reset tokens lookup failed", slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("reset failed"))
	}
	want := auth.HashToken(code)
	var matched *users.EmailToken
	for i := range live {
		if subtle.ConstantTimeCompare(live[i].CodeHash, want) == 1 {
			matched = &live[i]
			break
		}
	}
	if matched == nil {
		remaining, failErr := h.users.FailResetAttempt(ctx, u.ID, resetCodeMaxAttempts)
		if failErr != nil {
			h.log.Warn("count reset attempt failed", slog.Int64("user_id", u.ID), slog.String("error", failErr.Error()))
		}
		if remaining == 0 && len(live) > 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				errors.New("that code is wrong, and after five tries no code is live any more; request a new one"))
		}
		return nil, badResetCode
	}

	// A disabled or declined account should not be revivable by reset.
	if u.Status == users.StatusDisabled || u.Status == users.StatusDeclined {
		return nil, connect.NewError(connect.CodePermissionDenied,
			errors.New("this account is not permitted to sign in"))
	}

	// HIBP + argon2 before the code is consumed, so a rejected password
	// leaves the code usable. FailOpen mirrors Register: a checker
	// outage should not block a legitimate reset.
	if breached, err := (auth.FailOpen{Inner: h.pwned}).IsBreached(ctx, msg.NewPassword); err != nil {
		h.log.Warn("pwned check on reset failed", slog.String("error", err.Error()))
	} else if breached {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("this password appears in a known breach; please pick a different one"))
	}
	newHash, err := auth.HashPassword(msg.NewPassword)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := h.users.ConsumeResetTokens(ctx, u.ID, matched.ID); err != nil {
		if errors.Is(err, users.ErrTokenExpired) {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				errors.New("this code has expired or already been used; request a new one"))
		}
		h.log.Error("consume reset code failed", slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("reset failed"))
	}
	if err := h.users.SetPasswordHash(ctx, u.ID, newHash); err != nil {
		h.log.Error("set password hash failed", slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("reset failed"))
	}

	// A password change invalidates every existing session: a stolen
	// cookie from before the change must not survive it. Best-effort.
	if n, err := h.users.RevokeSessions(ctx, u.ID); err != nil {
		h.log.Warn("revoke sessions on reset failed", slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
	} else if n > 0 {
		h.log.Info("revoked sessions on reset", slog.Int64("user_id", u.ID), slog.Int64("count", n))
	}

	// Sign the member in on the same page: new token, fresh cookie.
	sessionToken, sessionHash, err := auth.NewToken()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("session mint failed"))
	}
	ipHash := hashIPBytes(ClientIP(req))
	if _, err := h.users.CreateSession(ctx, u.ID, sessionHash, h.cfg.SessionTTL, ipHash, req.Header().Get("User-Agent")); err != nil {
		h.log.Error("session create failed", slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("session create failed"))
	}

	resp := connect.NewResponse(&v1.ResetPasswordResponse{Me: h.buildMe(u)})
	setSessionCookie(resp.Header(), sessionToken, h.cfg.SessionTTL, h.cfg.CookieSecure)
	return resp, nil
}

// isDigits reports whether s is exactly n ASCII digits.
func isDigits(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
