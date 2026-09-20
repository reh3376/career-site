package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/reh3376/career-site/services/api/internal/auth"
	"github.com/reh3376/career-site/services/api/internal/email"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// AdminDecision serves the plain-HTTP endpoint the one-click Accept /
// Decline URLs in the approval email point at. The token IS the auth —
// no admin session required. A successful decision is single-use because
// the DB update only fires when the user is still in pending_approval;
// after that state changes, the same token bounces with a friendly error.
type AdminDecision struct {
	log       *slog.Logger
	users     *users.Repo
	email     email.Provider
	secret    []byte
	from      string
	ownerAddr string
	webBase   string
	// DefaultTTL applied to an Accept from the one-click email (FR-NOTF-05.a).
	// Other TTLs go through the console (Task 5).
	AcceptDefaultTTL users.GrantTTL
}

func NewAdminDecision(
	log *slog.Logger,
	repo *users.Repo,
	mailer email.Provider,
	secret []byte,
	from, ownerAddr, webBase string,
) *AdminDecision {
	return &AdminDecision{
		log:              log,
		users:            repo,
		email:            mailer,
		secret:           secret,
		from:             from,
		ownerAddr:        ownerAddr,
		webBase:          webBase,
		AcceptDefaultTTL: users.GrantTTL7d,
	}
}

type decisionRequest struct {
	Token string `json:"token"`
}

type decisionResponse struct {
	Status  string `json:"status"`
	Email   string `json:"email,omitempty"`
	Message string `json:"message,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

// Handle serves POST /api/admin/decision with a JSON body {"token":"..."}.
// Callers are the Next.js /admin/decision page's server component, which
// posts server-side and then renders whatever we return.
func (h *AdminDecision) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req decisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		writeJSON(w, http.StatusBadRequest, decisionResponse{
			Status: "error", Detail: "token is required",
		})
		return
	}

	tok, err := auth.VerifyDecision(h.secret, req.Token)
	if err != nil {
		h.log.Warn("decision token verify failed", slog.String("error", err.Error()))
		writeJSON(w, http.StatusUnauthorized, decisionResponse{
			Status:  "error",
			Detail:  "this decision link is invalid or has expired",
			Message: "Try opening the review page from the admin console instead.",
		})
		return
	}

	u, err := h.users.GetByID(r.Context(), tok.UserID)
	if err != nil {
		// The HMAC verified, so this token was issued for a real user at
		// send time — the row is just gone now. Almost always means an
		// admin (or a data-retention job) deleted the applicant after the
		// approval email went out. Distinct status so the frontend can
		// render a soft "no action needed" message instead of a scary
		// "something went wrong".
		writeJSON(w, http.StatusNotFound, decisionResponse{
			Status:  "user_gone",
			Message: "This request no longer exists — the applicant's account was removed.",
		})
		return
	}

	if u.Status != users.StatusPendingApproval {
		writeJSON(w, http.StatusConflict, decisionResponse{
			Status:  "already_decided",
			Email:   u.Email,
			Message: fmt.Sprintf("This request has already been resolved (current status: %s).", u.Status),
		})
		return
	}

	switch tok.Decision {
	case auth.DecisionApprove:
		h.approve(w, r.Context(), u, req.Token)
	case auth.DecisionDecline:
		h.decline(w, r.Context(), u, req.Token, referenceID(u))
	default:
		writeJSON(w, http.StatusBadRequest, decisionResponse{
			Status: "error", Detail: "unknown decision",
		})
	}
}

// ApproveUser flips a pending user to ACTIVE with the default TTL,
// records the decision, and fires the approval email. Shared by both
// the one-click email HTTP handler and the AdminService.ApproveRegistration
// RPC — `via` distinguishes them in the audit log ("email_link" vs
// "console") and `tokenHash` is nil for console-initiated approvals.
// Mutates u.ExpiresAt so the caller can render it back.
func (h *AdminDecision) ApproveUser(ctx context.Context, u *users.User, via string, tokenHash []byte) (users.GrantTTL, error) {
	ttl := h.AcceptDefaultTTL
	d := ttl.Duration()
	expiresAt := time.Now().UTC().Add(d)

	if err := h.users.Activate(ctx, u.ID, &expiresAt); err != nil {
		h.log.Error("activate failed", slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
		return ttl, fmt.Errorf("activate: %w", err)
	}
	if err := h.users.RecordApprovalDecision(ctx, users.ApprovalDecision{
		UserID:     u.ID,
		Decision:   "approve",
		DecidedVia: via,
		GrantedTTL: &d,
		TokenHash:  tokenHash,
	}); err != nil {
		h.log.Warn("record decision failed", slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
	}
	u.ExpiresAt = &expiresAt
	u.Status = users.StatusActive

	go h.sendUserApproved(u)
	return ttl, nil
}

// DeclineUser flips a pending user to DECLINED, records the decision,
// and fires the decline email. Shared by both the one-click email
// HTTP handler and the AdminService.DeclineRegistration RPC — `via`
// distinguishes them in the audit log and `tokenHash` is nil for
// console-initiated declines.
func (h *AdminDecision) DeclineUser(ctx context.Context, u *users.User, via string, tokenHash []byte) error {
	if err := h.users.SetStatus(ctx, u.ID, users.StatusDeclined); err != nil {
		h.log.Error("decline failed", slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
		return fmt.Errorf("set status: %w", err)
	}
	if err := h.users.RecordApprovalDecision(ctx, users.ApprovalDecision{
		UserID:     u.ID,
		Decision:   "decline",
		DecidedVia: via,
		TokenHash:  tokenHash,
	}); err != nil {
		h.log.Warn("record decision failed", slog.Int64("user_id", u.ID), slog.String("error", err.Error()))
	}
	u.Status = users.StatusDeclined
	go h.sendUserDeclined(u, referenceID(u))
	return nil
}

// approve / decline are the thin HTTP wrappers used by the one-click
// email flow. Business logic lives in ApproveUser / DeclineUser so
// the admin-console RPCs can reuse it.
func (h *AdminDecision) approve(w http.ResponseWriter, ctx context.Context, u *users.User, tokenPlain string) {
	ttl, err := h.ApproveUser(ctx, u, "email_link", auth.HashToken(tokenPlain))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, decisionResponse{Status: "error", Detail: "server error"})
		return
	}
	writeJSON(w, http.StatusOK, decisionResponse{
		Status:  "approved",
		Email:   u.Email,
		Message: fmt.Sprintf("Approved %s for %s access.", u.Name, ttl),
	})
}

func (h *AdminDecision) decline(w http.ResponseWriter, ctx context.Context, u *users.User, tokenPlain string, _ref string) {
	if err := h.DeclineUser(ctx, u, "email_link", auth.HashToken(tokenPlain)); err != nil {
		writeJSON(w, http.StatusInternalServerError, decisionResponse{Status: "error", Detail: "server error"})
		return
	}
	writeJSON(w, http.StatusOK, decisionResponse{
		Status:  "declined",
		Email:   u.Email,
		Message: fmt.Sprintf("Declined %s.", u.Name),
	})
}

func (h *AdminDecision) sendUserApproved(u *users.User) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	summary := "permanent"
	if u.ExpiresAt != nil {
		summary = "through " + u.ExpiresAt.UTC().Format("Mon, 02 Jan 2006 15:04 UTC")
	}
	text, html, err := email.UserApprovedTemplate.Render(map[string]any{
		"Name":          u.Name,
		"SignInURL":     h.webBase + "/login",
		"AccessSummary": summary,
		"OwnerEmail":    h.ownerAddr,
	})
	if err != nil {
		h.log.Warn("render user_approved failed", slog.String("error", err.Error()))
		return
	}
	sendErr := h.email.Send(ctx, email.Message{
		To:       u.Email,
		From:     h.from,
		Subject:  "Your career-site access is approved",
		TextBody: text,
		HTMLBody: html,
	})
	errText := ""
	if sendErr != nil {
		h.log.Warn("send user_approved failed", slog.Int64("user_id", u.ID), slog.String("error", sendErr.Error()))
		errText = truncErr(sendErr.Error())
	}
	if recErr := h.users.RecordNotification(ctx, u.ID, "user_approved", errText); recErr != nil {
		h.log.Warn("record user_approved notification failed", slog.Int64("user_id", u.ID), slog.String("error", recErr.Error()))
	}
}

func (h *AdminDecision) sendUserDeclined(u *users.User, ref string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	text, html, err := email.UserDeclinedTemplate.Render(map[string]any{
		"Name":         u.Name,
		"Email":        u.Email,
		"Organization": nonEmpty(u.Organization, "(not provided)"),
		"StatedRole":   nonEmpty(u.StatedRole, "(not provided)"),
		"SubmittedAt":  u.CreatedAt.UTC().Format(time.RFC1123),
		"ReferenceID":  ref,
		"OwnerEmail":   h.ownerAddr,
		"ReviewURL":    h.webBase + "/admin/pending",
	})
	if err != nil {
		h.log.Warn("render user_declined failed", slog.String("error", err.Error()))
		return
	}
	sendErr := h.email.Send(ctx, email.Message{
		To:       u.Email,
		From:     h.from,
		Subject:  "Your career-site access request",
		TextBody: text,
		HTMLBody: html,
	})
	errText := ""
	if sendErr != nil {
		h.log.Warn("send user_declined failed", slog.Int64("user_id", u.ID), slog.String("error", sendErr.Error()))
		errText = truncErr(sendErr.Error())
	}
	if recErr := h.users.RecordNotification(ctx, u.ID, "user_declined", errText); recErr != nil {
		h.log.Warn("record user_declined notification failed", slog.Int64("user_id", u.ID), slog.String("error", recErr.Error()))
	}
}

// truncErr keeps the provider's error string short so a single row's
// last_notification_error column never grows unbounded. 500 chars is
// well past the useful signal (Resend / SMTP errors are short).
func truncErr(s string) string {
	const max = 500
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// SendUserAutoDeclined is called from the auto-decline scheduler.
func (h *AdminDecision) SendUserAutoDeclined(ctx context.Context, u *users.User) error {
	text, html, err := email.UserAutoDeclinedTemplate.Render(map[string]any{
		"Name":        u.Name,
		"RegisterURL": h.webBase + "/register",
		"OwnerEmail":  h.ownerAddr,
	})
	if err != nil {
		return fmt.Errorf("render user_auto_declined: %w", err)
	}
	return h.email.Send(ctx, email.Message{
		To:       u.Email,
		From:     h.from,
		Subject:  "Your career-site request timed out",
		TextBody: text,
		HTMLBody: html,
	})
}

// referenceID reproduces the tag the approval email put in the pasteable
// block; keeps the two emails traceable to the same request.
func referenceID(u *users.User) string {
	return fmt.Sprintf("REQ-%d-%s", u.ID, u.CreatedAt.UTC().Format("20060102"))
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
