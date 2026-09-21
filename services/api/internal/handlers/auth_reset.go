package handlers

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/auth"
	"github.com/reh3376/career-site/services/api/internal/email"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// ForgotPassword issues a single-use reset token, delivers it via
// email, and returns a fixed message. The response is deliberately
// identical for known and unknown emails so this endpoint cannot be
// used to enumerate accounts — see FR-AUTH-19.
//
// Rate-limited on the shared login limiter, keyed on (ip, email), so
// a burst of bogus emails from one source can't flood the mail
// provider or the DB.
func (h *Auth) ForgotPassword(
	ctx context.Context,
	req *connect.Request[v1.ForgotPasswordRequest],
) (*connect.Response[v1.ForgotPasswordResponse], error) {
	addr := strings.ToLower(strings.TrimSpace(req.Msg.Email))
	if addr == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("email is required"))
	}

	// Reuse the login limiter: a single (ip, email) tuple gets at
	// most 5 attempts per 15 min. Combines with the existing login
	// bucket so an attacker can't split their budget between login
	// and reset flows on the same target.
	if h.loginLimiter != nil {
		key := "reset:" + req.Peer().Addr + "|" + addr
		if ok, retry := h.loginLimiter.Allow(key); !ok {
			h.log.Warn("reset rate limited", slog.String("email", addr), slog.Duration("retry_after", retry))
			// Same fixed response — do not signal to the caller
			// whether they hit the limit or the account is unknown.
			return connect.NewResponse(&v1.ForgotPasswordResponse{
				Message: fixedResetMessage(),
			}), nil
		}
	}

	u, err := h.users.GetByEmail(ctx, addr)
	if err == nil {
		// Only mint + send when the account exists. Timing is close
		// enough (a DB miss vs a DB hit + row insert + email queue)
		// that a network attacker can't cleanly enumerate; a local
		// attacker enumerating is a lost cause anyway.
		if sendErr := h.sendPasswordResetEmail(ctx, u); sendErr != nil {
			h.log.Warn("send password_reset failed",
				slog.Int64("user_id", u.ID), slog.String("error", sendErr.Error()),
			)
		}
	} else {
		h.log.Info("reset requested for unknown email", slog.String("email", addr))
	}

	return connect.NewResponse(&v1.ForgotPasswordResponse{
		Message: fixedResetMessage(),
	}), nil
}

// fixedResetMessage is the exact response the caller sees whether
// their email matched an account or not.
func fixedResetMessage() string {
	return "If an account exists for that email, we've sent a reset link. Check your inbox — the link is valid for 60 minutes."
}

func (h *Auth) sendPasswordResetEmail(ctx context.Context, u *users.User) error {
	token, tokenHash, err := auth.NewToken()
	if err != nil {
		return err
	}
	if _, err := h.users.CreateToken(ctx, u.ID, users.PurposeReset, tokenHash, nil, h.cfg.ResetTTL); err != nil {
		return err
	}
	resetURL := h.cfg.WebBaseURL + "/reset-password?token=" + token

	text, htmlBody, err := email.PasswordResetTemplate.Render(map[string]any{
		"Name":       u.Name,
		"ResetURL":   resetURL,
		"OwnerEmail": h.cfg.OwnerContactEmail,
	})
	if err != nil {
		return err
	}
	return h.email.Send(ctx, email.Message{
		To:       u.Email,
		From:     h.cfg.MailFrom,
		Subject:  "Reset your career-site password",
		TextBody: text,
		HTMLBody: htmlBody,
		Kind:     "password_reset",
		UserID:   u.ID,
	})
}

// ResetPassword consumes a single-use reset token, updates the
// password hash, revokes every existing session for the account
// (an old cookie must not survive a password change), and issues a
// fresh session so the user is signed in on the same page.
//
// The RPC deliberately does not require the old password — that's
// the whole point of a reset flow. The token itself is the auth.
func (h *Auth) ResetPassword(
	ctx context.Context,
	req *connect.Request[v1.ResetPasswordRequest],
) (*connect.Response[v1.ResetPasswordResponse], error) {
	msg := req.Msg
	if msg.Token == "" || msg.NewPassword == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("token and new_password are required"))
	}

	// Consume the token atomically (marks used, returns user_id).
	userID, err := h.users.ConsumeToken(ctx, auth.HashToken(msg.Token), users.PurposeReset)
	if err != nil {
		if errors.Is(err, users.ErrTokenExpired) {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				errors.New("this reset link has expired or already been used — request a new one"))
		}
		h.log.Error("consume reset token failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("reset failed"))
	}

	u, err := h.users.GetByID(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("reset failed"))
	}
	// A disabled or declined account should not be revivable by reset.
	if u.Status == users.StatusDisabled || u.Status == users.StatusDeclined {
		return nil, connect.NewError(connect.CodePermissionDenied,
			errors.New("this account is not permitted to sign in"))
	}

	// HIBP + argon2. Hash before touching the DB so a slow HIBP
	// response doesn't leave a half-applied state. FailOpen mirrors
	// Register — a checker outage should not block a legitimate reset.
	if breached, err := (auth.FailOpen{Inner: h.pwned}).IsBreached(ctx, msg.NewPassword); err != nil {
		h.log.Warn("pwned check on reset failed", slog.String("error", err.Error()))
	} else if breached {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("this password appears in a known breach — please pick a different one"))
	}
	newHash, err := auth.HashPassword(msg.NewPassword)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("hash failed"))
	}
	if err := h.users.SetPasswordHash(ctx, u.ID, newHash); err != nil {
		h.log.Error("set password hash failed", slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("reset failed"))
	}

	// A password change invalidates every existing session for the
	// user — otherwise a stolen cookie from before the change
	// survives, defeating the reset's purpose. Best-effort.
	if n, err := h.users.RevokeSessions(ctx, u.ID); err != nil {
		h.log.Warn("revoke sessions on reset failed",
			slog.Int64("user_id", u.ID), slog.String("error", err.Error()),
		)
	} else if n > 0 {
		h.log.Info("revoked sessions on reset",
			slog.Int64("user_id", u.ID), slog.Int64("count", n),
		)
	}

	// Sign the user in on the same page — new token, fresh cookie.
	sessionToken, sessionHash, err := auth.NewToken()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("session mint failed"))
	}
	ipHash := hashIPBytes(req.Peer().Addr)
	if _, err := h.users.CreateSession(ctx, u.ID, sessionHash, h.cfg.SessionTTL, ipHash, req.Header().Get("User-Agent")); err != nil {
		h.log.Error("session create failed", slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("session create failed"))
	}

	resp := connect.NewResponse(&v1.ResetPasswordResponse{Me: h.buildMe(u)})
	setSessionCookie(resp.Header(), sessionToken, h.cfg.SessionTTL, h.cfg.CookieSecure)
	return resp, nil
}
