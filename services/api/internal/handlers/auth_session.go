package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/auth"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// SessionCookieName is the browser-facing cookie name. Prefixed to reduce
// the chance of collision with third-party cookies on the same origin.
const SessionCookieName = "career_site_session"

// Login validates email + password, creates a session, sets the cookie,
// and returns Me. Failed logins return a generic message (no enumeration).
func (h *Auth) Login(
	ctx context.Context,
	req *connect.Request[v1.LoginRequest],
) (*connect.Response[v1.LoginResponse], error) {
	msg := req.Msg
	addr := strings.ToLower(strings.TrimSpace(msg.Email))
	if addr == "" || msg.Password == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("email and password are required"))
	}

	// Rate limit before any DB or argon2 work. Key on (ip, email) so
	// a botnet can't spread attempts across many source IPs to bypass
	// per-IP, and a single admin who's mistyped their password a few
	// times doesn't lock out every other user behind the same NAT.
	if h.loginLimiter != nil {
		key := "login:" + req.Peer().Addr + "|" + addr
		if ok, retry := h.loginLimiter.Allow(key); !ok {
			h.log.Warn("login rate limited", slog.String("email", addr), slog.Duration("retry_after", retry))
			return nil, connect.NewError(connect.CodeResourceExhausted,
				errors.New("too many attempts; try again shortly"))
		}
	}

	u, err := h.users.GetByEmail(ctx, addr)
	if err != nil {
		// Same response for "no such account" and "wrong password" so the
		// endpoint does not distinguish. Constant-time argon2 still runs so
		// there is no timing side-channel either.
		_ = auth.VerifyPassword(msg.Password, dummyArgonHash)
		return nil, badCredentials
	}

	if hashErr := auth.VerifyPassword(msg.Password, currentPasswordHash(ctx, h.users, u.ID)); hashErr != nil {
		return nil, badCredentials
	}

	if err := statusToLoginError(u.Status); err != nil {
		return nil, err
	}

	// Session-fixation defence: if the browser is presenting an
	// existing session cookie at login time, revoke it before minting
	// the fresh one. An attacker who planted a cookie on the victim
	// (via a link or cross-site inject) can't have it survive the
	// authenticated login and become a legitimate session. Best-effort
	// — a miss here just means one dead cookie stays dead in the DB
	// until its TTL, which is harmless.
	if oldToken := sessionTokenFromRequest(req); oldToken != "" {
		if err := h.users.RevokeSession(ctx, auth.HashToken(oldToken)); err != nil {
			h.log.Warn("pre-login session revoke failed",
				slog.Int64("user_id", u.ID),
				slog.String("error", err.Error()),
			)
		}
	}

	// Mint the session.
	token, tokenHash, err := auth.NewToken()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("session mint failed"))
	}
	ipHash := hashIPBytes(req.Peer().Addr)
	_, err = h.users.CreateSession(ctx, u.ID, tokenHash, h.cfg.SessionTTL, ipHash, req.Header().Get("User-Agent"))
	if err != nil {
		h.log.Error("session create failed", slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("session create failed"))
	}
	// Log the login as an activity event so the admin console has
	// something to render on member detail pages. Best-effort — a
	// failure here shouldn't fail the login.
	if _, err := h.users.RecordActivity(ctx, users.ActivityEvent{
		UserID: u.ID,
		Kind:   "login",
	}); err != nil {
		h.log.Warn("record login activity failed",
			slog.Int64("user_id", u.ID), slog.String("error", err.Error()),
		)
	}

	resp := connect.NewResponse(&v1.LoginResponse{
		Me:          h.buildMe(u),
		MfaRequired: false, // Task 5 flips true for admins pre-MFA.
	})
	setSessionCookie(resp.Header(), token, h.cfg.SessionTTL, h.cfg.CookieSecure)
	return resp, nil
}

// Logout consumes the caller's session cookie (revoking that one session)
// and clears the browser cookie.
func (h *Auth) Logout(
	ctx context.Context,
	req *connect.Request[v1.LogoutRequest],
) (*connect.Response[v1.LogoutResponse], error) {
	if token := sessionTokenFromRequest(req); token != "" {
		_ = h.users.RevokeSession(ctx, auth.HashToken(token))
	}
	resp := connect.NewResponse(&v1.LogoutResponse{})
	clearSessionCookie(resp.Header(), h.cfg.CookieSecure)
	return resp, nil
}

// LookupSessionUser is the helper other handlers call to resolve the
// caller's session cookie to a *users.User. Returns nil, nil for
// "no session" and nil, err for a hard failure the caller should surface.
func (h *Auth) LookupSessionUser(ctx context.Context, req connect.AnyRequest) (*users.User, error) {
	return h.lookupSessionToken(ctx, sessionTokenFromRequest(req))
}

// LookupSessionUserHTTP is LookupSessionUser for plain net/http
// handlers (file downloads) that are not Connect RPCs.
func (h *Auth) LookupSessionUserHTTP(ctx context.Context, r *http.Request) (*users.User, error) {
	c, err := r.Cookie(SessionCookieName)
	if err != nil {
		return nil, nil
	}
	return h.lookupSessionToken(ctx, c.Value)
}

func (h *Auth) lookupSessionToken(ctx context.Context, token string) (*users.User, error) {
	if token == "" {
		return nil, nil
	}
	u, err := h.users.LookupSessionUser(ctx, auth.HashToken(token))
	if errors.Is(err, users.ErrNotFound) {
		return nil, nil
	}
	return u, err
}

// buildMe maps a repo user to the proto Me carried in responses.
func (h *Auth) buildMe(u *users.User) *v1.Me {
	me := &v1.Me{
		Id:           fmt.Sprintf("%d", u.ID),
		Name:         u.Name,
		Email:        u.Email,
		Organization: u.Organization,
		StatedRole:   u.StatedRole,
		Status:       statusToProto(u.Status),
		Role:         roleToProto(u.Role),
	}
	if !u.CreatedAt.IsZero() {
		me.CreatedAt = timestamppb.New(u.CreatedAt)
	}
	if u.ExpiresAt != nil {
		me.ExpiresAt = timestamppb.New(*u.ExpiresAt)
	}
	return me
}

func statusToProto(s users.Status) v1.MemberStatus {
	switch s {
	case users.StatusUnverified:
		return v1.MemberStatus_MEMBER_STATUS_UNVERIFIED
	case users.StatusPendingApproval:
		return v1.MemberStatus_MEMBER_STATUS_PENDING_APPROVAL
	case users.StatusActive:
		return v1.MemberStatus_MEMBER_STATUS_ACTIVE
	case users.StatusDeclined:
		return v1.MemberStatus_MEMBER_STATUS_DECLINED
	case users.StatusExpired:
		return v1.MemberStatus_MEMBER_STATUS_EXPIRED
	case users.StatusDisabled:
		return v1.MemberStatus_MEMBER_STATUS_DISABLED
	default:
		return v1.MemberStatus_MEMBER_STATUS_UNSPECIFIED
	}
}

func roleToProto(r users.Role) v1.MemberRole {
	if r == users.RoleAdmin {
		return v1.MemberRole_MEMBER_ROLE_ADMIN
	}
	return v1.MemberRole_MEMBER_ROLE_MEMBER
}

// statusToLoginError maps non-active states to actionable Connect errors so
// the frontend can render the right message.
func statusToLoginError(s users.Status) error {
	switch s {
	case users.StatusActive:
		return nil
	case users.StatusUnverified:
		return connect.NewError(connect.CodeFailedPrecondition,
			errors.New("please verify your email address — check your inbox for the link"))
	case users.StatusPendingApproval:
		return connect.NewError(connect.CodeFailedPrecondition,
			errors.New("your request is with Roger — you'll receive an email when he decides"))
	case users.StatusDeclined:
		return connect.NewError(connect.CodePermissionDenied,
			errors.New("this account is not permitted to sign in"))
	case users.StatusExpired:
		return connect.NewError(connect.CodePermissionDenied,
			errors.New("your access has expired — reach out to Roger to renew"))
	case users.StatusDisabled:
		return connect.NewError(connect.CodePermissionDenied,
			errors.New("this account is disabled — reach out to Roger for help"))
	default:
		return badCredentials
	}
}

// currentPasswordHash reads the current PHC-encoded hash for a user id.
// Isolated so a future test can swap it. Repo doesn't expose it because
// the field is only ever needed at Login time.
func currentPasswordHash(ctx context.Context, r *users.Repo, id int64) string {
	return r.PasswordHash(ctx, id)
}

// dummyArgonHash is a fixed Argon2 hash used to keep the login path a
// constant-time compare when the email is unknown. The password never
// matches this hash, but the compare cost keeps the timing profile
// indistinguishable from a real check.
const dummyArgonHash = "$argon2id$v=19$m=65536,t=2,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA0"

var badCredentials = connect.NewError(connect.CodeUnauthenticated, errors.New("email or password is incorrect"))

// sessionTokenFromRequest extracts the session cookie value from a Connect
// request. Returns "" when the cookie is missing.
func sessionTokenFromRequest(req connect.AnyRequest) string {
	raw := req.Header().Values("Cookie")
	for _, line := range raw {
		for _, part := range strings.Split(line, ";") {
			part = strings.TrimSpace(part)
			name, value, ok := strings.Cut(part, "=")
			if ok && name == SessionCookieName {
				return value
			}
		}
	}
	return ""
}

// setSessionCookie writes a Set-Cookie header on the Connect response for
// the given plaintext token. Secure flag is off in dev (http) and on in
// production (https).
func setSessionCookie(h http.Header, token string, ttl time.Duration, secure bool) {
	c := &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	}
	h.Add("Set-Cookie", c.String())
}

func clearSessionCookie(h http.Header, secure bool) {
	c := &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
	h.Add("Set-Cookie", c.String())
}

// hashIPBytes is a byte-slice version of hashIP for the sessions.ip_hash
// bytea column. Truncates to 16 bytes.
func hashIPBytes(addr string) []byte {
	if host, _, err := splitHostPort(addr); err == nil {
		addr = host
	}
	full := auth.HashToken(addr) // reuse the SHA-256 helper
	return full[:16]
}
