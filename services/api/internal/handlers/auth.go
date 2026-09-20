package handlers

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/gen/career/v1/careerv1connect"
	"github.com/reh3376/career-site/services/api/internal/auth"
	"github.com/reh3376/career-site/services/api/internal/email"
	"github.com/reh3376/career-site/services/api/internal/ratelimit"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Auth implements careerv1connect.AuthServiceHandler.
//
// Phase 1 wires Register and Verify. Every other RPC still returns
// UNIMPLEMENTED from the embedded default; they light up in the sign-in,
// admin-approval, and password-reset tasks that follow.
type Auth struct {
	careerv1connect.UnimplementedAuthServiceHandler

	log   *slog.Logger
	users *users.Repo
	email email.Provider
	pwned auth.PwnedChecker
	cfg   AuthConfig
	// loginLimiter: per-(ip, email) bucket that slows credential-
	// stuffing without needing an external cache. Sized for 5
	// attempts per 15 minutes (single-instance API only; a
	// horizontally-scaled deployment would need Redis).
	loginLimiter *ratelimit.Limiter
}

type AuthConfig struct {
	// Public base URL of the web app (no trailing slash). Verification link:
	//   {WebBaseURL}/verify?token=...
	WebBaseURL string
	// Address the admin approval email is delivered to.
	OwnerContactEmail string
	// From: address of every outbound message unless overridden.
	MailFrom string
	// TTL for the initial email-verify token; FR-AUTH-03 caps at 24 h.
	VerifyTTL time.Duration
	// TTL for admin one-click Accept/Decline URLs (FR-AUTH-15).
	DecisionTokenTTL time.Duration
	// HMAC signing key for one-click Accept/Decline tokens.
	DecisionTokenSecret []byte
	// If true, the current consent version required at registration. Empty
	// means "accept any nonzero version" — safe for dev; production sets it.
	ConsentVersion string

	// Sessions
	SessionTTL time.Duration // FR-AUTH-08: 30-day absolute lifetime.
	// CookieSecure toggles the Secure attribute on the session cookie. Off
	// in local dev (http://localhost/), on in production behind TLS.
	CookieSecure bool
}

func NewAuth(
	log *slog.Logger,
	repo *users.Repo,
	mailer email.Provider,
	pwned auth.PwnedChecker,
	cfg AuthConfig,
) *Auth {
	if cfg.VerifyTTL == 0 {
		cfg.VerifyTTL = 24 * time.Hour
	}
	if cfg.DecisionTokenTTL == 0 {
		cfg.DecisionTokenTTL = 7 * 24 * time.Hour
	}
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = 30 * 24 * time.Hour
	}
	return &Auth{
		log:          log,
		users:        repo,
		email:        mailer,
		pwned:        pwned,
		cfg:          cfg,
		loginLimiter: ratelimit.New(5, 5.0/(15*60)), // 5 attempts, refills to 5 over 15 min
	}
}

// Register accepts a new registration, hashes the password, creates the
// user in `unverified`, and sends the verification email. Response text is
// intentionally the same for a duplicate email so the endpoint does not
// leak account existence (FR-AUTH-01).
func (h *Auth) Register(
	ctx context.Context,
	req *connect.Request[v1.RegisterRequest],
) (*connect.Response[v1.RegisterResponse], error) {
	msg := req.Msg
	if !msg.ConsentAccepted {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("consent_accepted must be true"))
	}
	if h.cfg.ConsentVersion != "" && msg.ConsentVersion != h.cfg.ConsentVersion {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("consent_version out of date; refresh the page"))
	}
	addr := strings.ToLower(strings.TrimSpace(msg.Email))
	name := strings.TrimSpace(msg.Name)
	if addr == "" || name == "" || len(msg.Password) < auth.MinPasswordLen {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("name, email, and password (>= %d chars) are required", auth.MinPasswordLen))
	}

	// HaveIBeenPwned: refuse breached passwords. Transport error is logged
	// and treated as "not breached" so an outage does not block signup.
	if breached, err := (auth.FailOpen{Inner: h.pwned}).IsBreached(ctx, msg.Password); err != nil {
		h.log.Warn("pwned check failed", slog.String("error", err.Error()))
	} else if breached {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("that password appears in a known breach corpus; pick a different one"))
	}

	hash, err := auth.HashPassword(msg.Password)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Attempt the insert; a duplicate email returns the same generic
	// message the happy path returns.
	_, createErr := h.users.Create(ctx, users.CreateInput{
		Email:          addr,
		PasswordHash:   hash,
		Name:           name,
		Organization:   strings.TrimSpace(msg.Organization),
		StatedRole:     strings.TrimSpace(msg.StatedRole),
		ConsentVersion: msg.ConsentVersion,
	})
	if createErr != nil && !errors.Is(createErr, users.ErrEmailInUse) {
		h.log.Error("user create failed", slog.String("error", createErr.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("registration failed"))
	}

	// Look up the user (either just-created or the pre-existing row) so the
	// verify token attaches to the right user id.
	u, err := h.users.GetByEmail(ctx, addr)
	if err != nil {
		h.log.Error("post-create lookup failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("registration failed"))
	}

	// Only send verify email if the account is still in a state where
	// verification is meaningful. An `active` user re-registering silently
	// gets the same generic response.
	if u.Status == users.StatusUnverified {
		if err := h.sendVerifyEmail(ctx, u); err != nil {
			h.log.Error("send verify email failed",
				slog.Int64("user_id", u.ID),
				slog.String("error", err.Error()),
			)
			return nil, connect.NewError(connect.CodeInternal, errors.New("registration failed"))
		}
	}

	return connect.NewResponse(&v1.RegisterResponse{
		Message: "Check your email for a verification link.",
	}), nil
}

func (h *Auth) sendVerifyEmail(ctx context.Context, u *users.User) error {
	token, tokenHash, err := auth.NewToken()
	if err != nil {
		return err
	}
	code, codeHash, err := auth.NewCode()
	if err != nil {
		return err
	}
	if _, err := h.users.CreateToken(ctx, u.ID, users.PurposeVerify, tokenHash, codeHash, h.cfg.VerifyTTL); err != nil {
		return err
	}

	verifyURL := h.cfg.WebBaseURL + "/verify?token=" + token

	text, htmlBody, err := email.VerifyTemplate.Render(map[string]any{
		"Name":      u.Name,
		"VerifyURL": verifyURL,
		"Code":      code,
	})
	if err != nil {
		return err
	}
	return h.email.Send(ctx, email.Message{
		To:       u.Email,
		From:     h.cfg.MailFrom,
		Subject:  "Verify your email for career-site",
		TextBody: text,
		HTMLBody: htmlBody,
	})
}

// Verify consumes a verify token or (email + code) pair, moves the user
// from unverified to pending_approval, and triggers the admin approval
// email (Task 3 wires the Accept/Decline handlers those URLs point to;
// today the email carries placeholder URLs and any admin can copy the
// reference id into the review surface once it exists).
func (h *Auth) Verify(
	ctx context.Context,
	req *connect.Request[v1.VerifyRequest],
) (*connect.Response[v1.VerifyResponse], error) {
	tokenHash, err := h.resolveVerifyCredential(ctx, req.Msg)
	if err != nil {
		return nil, err
	}

	userID, err := h.users.ConsumeToken(ctx, tokenHash, users.PurposeVerify)
	if err != nil {
		if errors.Is(err, users.ErrTokenExpired) {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				errors.New("this verification link has expired or already been used"))
		}
		return nil, connect.NewError(connect.CodeInternal, errors.New("verify failed"))
	}

	// Look up the user, then decide whether to auto-approve (whitelist hit)
	// or drop them into pending_approval.
	u, err := h.users.GetByID(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("verify failed"))
	}

	grant, err := h.users.GetActiveGrant(ctx, u.Email)
	switch {
	case err == nil:
		// Whitelist hit — grant active, set expiry, record decision, welcome.
		return h.applyWhitelistAutoApprove(ctx, u, grant)
	case errors.Is(err, users.ErrNotFound):
		// No whitelist entry — standard pending_approval flow.
		return h.applyPendingApproval(ctx, u, req.Peer().Addr, req.Header().Get("User-Agent"))
	default:
		h.log.Error("whitelist lookup failed",
			slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("verify failed"))
	}
}

// applyWhitelistAutoApprove activates a whitelisted user, sets expires_at
// from the grant's TTL, records the decision, and dispatches the welcome
// email in a background goroutine.
func (h *Auth) applyWhitelistAutoApprove(
	ctx context.Context,
	u *users.User,
	grant *users.AccessGrant,
) (*connect.Response[v1.VerifyResponse], error) {
	var expiresAt *time.Time
	var grantedTTL *time.Duration
	if !grant.DefaultTTL.IsPermanent() {
		d := grant.DefaultTTL.Duration()
		t := time.Now().UTC().Add(d)
		expiresAt = &t
		grantedTTL = &d
	}

	if err := h.users.Activate(ctx, u.ID, expiresAt); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("verify failed"))
	}
	if err := h.users.RecordApprovalDecision(ctx, users.ApprovalDecision{
		UserID:     u.ID,
		Decision:   "approve",
		DecidedVia: "whitelist_auto",
		GrantedTTL: grantedTTL,
	}); err != nil {
		h.log.Warn("record whitelist decision failed",
			slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
	}
	u.Status = users.StatusActive
	u.ExpiresAt = expiresAt

	go func() {
		emailCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := h.sendWhitelistWelcomeEmail(emailCtx, u); err != nil {
			h.log.Error("send whitelist-welcome email failed",
				slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
		}
	}()

	return connect.NewResponse(&v1.VerifyResponse{
		Me: &v1.Me{
			Id:     fmt.Sprintf("%d", u.ID),
			Name:   u.Name,
			Email:  u.Email,
			Status: v1.MemberStatus_MEMBER_STATUS_ACTIVE,
		},
	}), nil
}

// applyPendingApproval is the pre-whitelist behaviour: park the user and
// email Roger.
func (h *Auth) applyPendingApproval(
	ctx context.Context,
	u *users.User,
	remoteAddr, userAgent string,
) (*connect.Response[v1.VerifyResponse], error) {
	if err := h.users.SetStatus(ctx, u.ID, users.StatusPendingApproval); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("verify failed"))
	}
	u.Status = users.StatusPendingApproval

	go func() {
		emailCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := h.sendApprovalRequestEmail(emailCtx, u, remoteAddr, userAgent); err != nil {
			h.log.Error("send approval-request email failed",
				slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
		}
	}()

	return connect.NewResponse(&v1.VerifyResponse{
		Me: &v1.Me{
			Id:     fmt.Sprintf("%d", u.ID),
			Name:   u.Name,
			Email:  u.Email,
			Status: v1.MemberStatus_MEMBER_STATUS_PENDING_APPROVAL,
		},
	}), nil
}

func (h *Auth) sendWhitelistWelcomeEmail(ctx context.Context, u *users.User) error {
	summary := "permanent"
	if u.ExpiresAt != nil {
		summary = fmt.Sprintf("through %s UTC", u.ExpiresAt.UTC().Format("Mon, 02 Jan 2006 15:04"))
	}
	text, html, err := email.WelcomeWhitelistTemplate.Render(map[string]any{
		"Name":          u.Name,
		"SignInURL":     h.cfg.WebBaseURL + "/login",
		"AccessSummary": summary,
		"OwnerEmail":    h.cfg.OwnerContactEmail,
	})
	if err != nil {
		return err
	}
	return h.email.Send(ctx, email.Message{
		To:       u.Email,
		From:     h.cfg.MailFrom,
		Subject:  "Your career-site access is ready",
		TextBody: text,
		HTMLBody: html,
	})
}

func (h *Auth) resolveVerifyCredential(_ context.Context, msg *v1.VerifyRequest) ([]byte, error) {
	switch cred := msg.Credential.(type) {
	case *v1.VerifyRequest_Token:
		if cred.Token == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("token is empty"))
		}
		return auth.HashToken(cred.Token), nil
	case *v1.VerifyRequest_Code:
		if cred.Code == "" || msg.Email == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				errors.New("email and code are required for code-based verify"))
		}
		// Codes are keyed by user; hash the plaintext code and hand the
		// caller a "code hash" that the repo compares. Because we don't
		// know the user_id yet we look up the code-hash column directly
		// via a helper (below) — not yet implemented; token-based verify is
		// the primary path in Phase 1.
		return nil, connect.NewError(connect.CodeUnimplemented,
			errors.New("code-based verify lands in Task 4; use the emailed link for now"))
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("token or code required"))
	}
}

func (h *Auth) sendApprovalRequestEmail(ctx context.Context, u *users.User, remoteAddr, userAgent string) error {
	ipHash := hashIP(remoteAddr)
	reference := referenceID(u)

	acceptTok, err := auth.SignDecision(h.cfg.DecisionTokenSecret, u.ID, auth.DecisionApprove, h.cfg.DecisionTokenTTL)
	if err != nil {
		return fmt.Errorf("sign accept token: %w", err)
	}
	declineTok, err := auth.SignDecision(h.cfg.DecisionTokenSecret, u.ID, auth.DecisionDecline, h.cfg.DecisionTokenTTL)
	if err != nil {
		return fmt.Errorf("sign decline token: %w", err)
	}
	acceptURL := h.cfg.WebBaseURL + "/admin/decision?token=" + acceptTok
	declineURL := h.cfg.WebBaseURL + "/admin/decision?token=" + declineTok
	reviewURL := h.cfg.WebBaseURL + "/admin/pending"

	text, htmlBody, err := email.ApprovalRequestTemplate.Render(map[string]any{
		"Name":         u.Name,
		"Email":        u.Email,
		"Organization": nonEmpty(u.Organization, "(not provided)"),
		"StatedRole":   nonEmpty(u.StatedRole, "(not provided)"),
		"SubmittedAt":  time.Now().UTC().Format(time.RFC1123),
		"ReferenceID":  reference,
		"IPHash":       ipHash,
		"UserAgent":    nonEmpty(userAgent, "(unknown)"),
		"AcceptURL":    acceptURL,
		"DeclineURL":   declineURL,
		"ReviewURL":    reviewURL,
	})
	if err != nil {
		return err
	}
	return h.email.Send(ctx, email.Message{
		To:       h.cfg.OwnerContactEmail,
		From:     h.cfg.MailFrom,
		Subject:  fmt.Sprintf("[career-site] Access request from %s", u.Name),
		TextBody: text,
		HTMLBody: htmlBody,
	})
}

func nonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func hashIP(addr string) string {
	// Strip port, then SHA-256 truncated to 12 hex chars — enough to
	// distinguish addresses in the admin queue without storing raw IPs.
	if host, _, err := splitHostPort(addr); err == nil {
		addr = host
	}
	h := sha256.Sum256([]byte(addr))
	return fmt.Sprintf("%x", h[:6])
}

func splitHostPort(hostport string) (string, string, error) {
	i := strings.LastIndex(hostport, ":")
	if i < 0 {
		return hostport, "", nil
	}
	return hostport[:i], hostport[i+1:], nil
}

// Compile-time assertion that Auth satisfies the generated handler
// interface even as the interface grows.
var _ careerv1connect.AuthServiceHandler = (*Auth)(nil)

// ClientIP extracts a best-effort client IP from a Connect request for
// logging. Kept here in case future handlers need it.
func ClientIP(req connect.AnyRequest) string {
	if xff := req.Header().Get("X-Forwarded-For"); xff != "" {
		if i := strings.Index(xff, ","); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xri := req.Header().Get("X-Real-IP"); xri != "" {
		return xri
	}
	return req.Peer().Addr
}
