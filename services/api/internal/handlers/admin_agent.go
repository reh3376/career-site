package handlers

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/auth"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// The agent-access surface, /admin/agent (FSD FR-ADM-24; the login half
// is FR-AUTH-20 in auth_agent.go).

// AgentPasswordTTL is how long a one-time agent password stays usable
// after the owner creates it. The owner's figure (2026-11-08): fifteen
// minutes, restarted by creating a new one.
const AgentPasswordTTL = 15 * time.Minute

// SetAgentAccess names the agent account. Empty disables the surface;
// the server configures it from AGENT_LOGIN_USERNAME.
func (a *Admin) SetAgentAccess(username string) {
	a.agentUsername = strings.ToLower(strings.TrimSpace(username))
}

func (a *Admin) agentConfigured() error {
	if a.agentUsername == "" {
		return connect.NewError(connect.CodeFailedPrecondition,
			errors.New("agent login is not configured on this server"))
	}
	return nil
}

// CreateAgentPassword mints a one-time password, revoking any live one,
// and returns the plaintext once.
func (a *Admin) CreateAgentPassword(
	ctx context.Context,
	req *connect.Request[v1.CreateAgentPasswordRequest],
) (*connect.Response[v1.CreateAgentPasswordResponse], error) {
	caller, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	if err := a.agentConfigured(); err != nil {
		return nil, err
	}
	plain, err := auth.NewAgentPassword()
	if err != nil {
		a.log.Error("agent password generation failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not create a password"))
	}
	hash, err := auth.HashPassword(plain)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not create a password"))
	}
	p, err := a.users.CreateAgentPassword(ctx, hash, strings.TrimSpace(req.Msg.Reason), caller.ID, AgentPasswordTTL)
	if err != nil {
		a.log.Error("CreateAgentPassword failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not create a password"))
	}
	a.events.Emit(ctx, requestEvent(req, "agent.password_created", caller.ID, map[string]any{"password_id": p.ID}))
	// The plaintext is in the response and nowhere else: not in this
	// log line, not in the audit row (the interceptor records the
	// request, which has no password in it).
	a.log.Info("agent password created", slog.Int64("password_id", p.ID), slog.Int64("by", caller.ID))
	return connect.NewResponse(&v1.CreateAgentPasswordResponse{
		Password:  agentPasswordToProto(p, time.Now()),
		Plaintext: plain,
		Username:  a.agentUsername,
	}), nil
}

// ListAgentPasswords returns recent passwords and the live session count.
func (a *Admin) ListAgentPasswords(
	ctx context.Context,
	_ *connect.Request[v1.ListAgentPasswordsRequest],
) (*connect.Response[v1.ListAgentPasswordsResponse], error) {
	if err := a.agentConfigured(); err != nil {
		return nil, err
	}
	rows, err := a.users.ListAgentPasswords(ctx, 50)
	if err != nil {
		a.log.Error("ListAgentPasswords failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("list failed"))
	}
	now := time.Now()
	out := &v1.ListAgentPasswordsResponse{Username: a.agentUsername}
	for i := range rows {
		out.Passwords = append(out.Passwords, agentPasswordToProto(&rows[i], now))
	}
	if u, err := a.users.GetByEmail(ctx, a.agentUsername); err == nil {
		if n, err := a.users.CountLiveSessions(ctx, u.ID); err == nil {
			out.LiveSessions = int32(n)
		}
	}
	return connect.NewResponse(out), nil
}

// RevokeAgentPassword ends a live password by hand.
func (a *Admin) RevokeAgentPassword(
	ctx context.Context,
	req *connect.Request[v1.RevokeAgentPasswordRequest],
) (*connect.Response[v1.RevokeAgentPasswordResponse], error) {
	caller, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := strconv.ParseInt(req.Msg.Id, 10, 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id"))
	}
	p, err := a.users.RevokeAgentPassword(ctx, id, caller.ID)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("no such password"))
		}
		a.log.Error("RevokeAgentPassword failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("revoke failed"))
	}
	a.events.Emit(ctx, requestEvent(req, "agent.password_revoked", caller.ID, map[string]any{"password_id": p.ID}))
	return connect.NewResponse(&v1.RevokeAgentPasswordResponse{Password: agentPasswordToProto(p, time.Now())}), nil
}

// RevokeAgentSessions ends every live session of the agent account.
func (a *Admin) RevokeAgentSessions(
	ctx context.Context,
	req *connect.Request[v1.RevokeAgentSessionsRequest],
) (*connect.Response[v1.RevokeAgentSessionsResponse], error) {
	caller, err := RequireCaller(ctx)
	if err != nil {
		return nil, err
	}
	if err := a.agentConfigured(); err != nil {
		return nil, err
	}
	u, err := a.users.GetByEmail(ctx, a.agentUsername)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("the agent account does not exist yet"))
	}
	n, err := a.users.RevokeSessions(ctx, u.ID)
	if err != nil {
		a.log.Error("RevokeAgentSessions failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("revoke failed"))
	}
	a.events.Emit(ctx, requestEvent(req, "agent.sessions_revoked", caller.ID, map[string]any{"count": n}))
	return connect.NewResponse(&v1.RevokeAgentSessionsResponse{Revoked: int32(n)}), nil
}

func agentPasswordToProto(p *users.AgentPassword, now time.Time) *v1.AgentPassword {
	out := &v1.AgentPassword{
		Id:          strconv.FormatInt(p.ID, 10),
		CreatedAt:   timestamppb.New(p.CreatedAt),
		ExpiresAt:   timestamppb.New(p.ExpiresAt),
		Reason:      p.Reason,
		State:       p.State(now),
		SessionLive: p.SessionLive,
	}
	if p.UsedAt != nil {
		out.UsedAt = timestamppb.New(*p.UsedAt)
	}
	if p.RevokedAt != nil {
		out.RevokedAt = timestamppb.New(*p.RevokedAt)
	}
	if p.CreatedBy != nil {
		out.CreatedBy = strconv.FormatInt(*p.CreatedBy, 10)
	}
	return out
}
