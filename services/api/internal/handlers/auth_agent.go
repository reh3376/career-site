package handlers

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/auth"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// The agent account (FSD FR-AUTH-20).
//
// An agent doing a live pass or a review signs in with a fixed username
// and a password the owner created minutes earlier from the admin
// console. The password is good for one sign-in and for fifteen minutes
// from creation; creating a new one revokes the old, so there is never
// more than one that works. The account has no standing password hash,
// so the ordinary branch of Login can never sign it in, and every
// attempt still pays the sign-in rate limiter and the Turnstile
// challenge like any other.
//
// The owner's instruction was access to the entire site, so the row is
// an admin. What makes that acceptable is everything above plus the
// record: every password, who made it and why, when it was used and
// from where, and the session it minted, which the console can end.

// isAgentUsername reports whether a sign-in names the agent account.
// Compared lower-cased, as addresses are.
func (h *Auth) isAgentUsername(addr string) bool {
	return h.cfg.AgentUsername != "" && addr == strings.ToLower(h.cfg.AgentUsername)
}

// agentLogin is Login's branch for the agent account. The caller has
// already applied the rate limiter and the Turnstile challenge.
func (h *Auth) agentLogin(
	ctx context.Context,
	req *connect.Request[v1.LoginRequest],
	username string,
) (*connect.Response[v1.LoginResponse], error) {
	challengeKey := ClientIP(req)

	u, err := h.users.GetByEmail(ctx, username)
	if err != nil {
		// The row is created at boot, so a miss is a configuration
		// fault rather than a guess. It still gets the guess's answer,
		// and the same argon2 cost, so nothing about the account is
		// learnable from here.
		_ = auth.VerifyPassword(req.Msg.Password, dummyArgonHash)
		h.loginFailures.record(challengeKey)
		h.events.Emit(ctx, requestEvent(req, "login.failed", 0, map[string]any{"reason": "agent_no_account"}))
		h.log.Error("agent account missing; EnsureAgentUser did not run", slog.String("username", username))
		return nil, badCredentials
	}

	live, err := h.users.LiveAgentPasswords(ctx)
	if err != nil {
		h.log.Error("live agent passwords lookup failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("sign-in failed"))
	}
	var matched *users.AgentPassword
	for i := range live {
		if auth.VerifyPassword(req.Msg.Password, live[i].PasswordHash) == nil {
			matched = &live[i]
			break
		}
	}
	if matched == nil {
		if len(live) == 0 {
			// No live password: pay the same argon2 cost a comparison
			// would, so "nothing to compare against" and "wrong
			// password" take the same time.
			_ = auth.VerifyPassword(req.Msg.Password, dummyArgonHash)
		}
		h.loginFailures.record(challengeKey)
		h.events.Emit(ctx, requestEvent(req, "login.failed", u.ID, map[string]any{"reason": "agent_bad_password"}))
		return nil, badCredentials
	}
	h.loginFailures.clear(challengeKey)

	if err := statusToLoginError(u.Status); err != nil {
		h.events.Emit(ctx, requestEvent(req, "login.failed", u.ID, map[string]any{"reason": string(u.Status)}))
		return nil, err
	}

	// Session-fixation defence, as in Login.
	if oldToken := sessionTokenFromRequest(req); oldToken != "" {
		if err := h.users.RevokeSession(ctx, auth.HashToken(oldToken)); err != nil {
			h.log.Warn("pre-login session revoke failed", slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
		}
	}

	token, tokenHash, err := auth.NewToken()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("session mint failed"))
	}
	ipHash := hashIPBytes(ClientIP(req))
	sess, err := h.users.CreateSession(ctx, u.ID, tokenHash, h.cfg.AgentSessionTTL, ipHash, req.Header().Get("User-Agent"))
	if err != nil {
		h.log.Error("agent session create failed", slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("session create failed"))
	}

	// Consume the password for this session. Two sign-ins racing on the
	// same password both reach here; the one that loses the UPDATE
	// gives its session back and gets the wrong-password answer.
	if err := h.users.UseAgentPassword(ctx, matched.ID, sess.ID, ipHash); err != nil {
		_ = h.users.RevokeSession(ctx, tokenHash)
		h.events.Emit(ctx, requestEvent(req, "login.failed", u.ID, map[string]any{"reason": "agent_password_raced"}))
		return nil, badCredentials
	}

	if _, err := h.users.RecordActivity(ctx, users.ActivityEvent{UserID: u.ID, Kind: "login"}); err != nil {
		h.log.Warn("record agent login activity failed", slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
	}
	h.events.Emit(ctx, requestEvent(req, "login.success", u.ID, map[string]any{"user_id": u.ID}))
	h.events.Emit(ctx, requestEvent(req, "agent.login", u.ID, map[string]any{"user_id": u.ID, "password_id": matched.ID}))
	h.log.Info("agent signed in",
		slog.Int64("password_id", matched.ID), slog.Int64("session_id", sess.ID))

	resp := connect.NewResponse(&v1.LoginResponse{Me: h.buildMe(u)})
	setSessionCookie(resp.Header(), token, h.cfg.AgentSessionTTL, h.cfg.CookieSecure)
	return resp, nil
}
