package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/gen/career/v1/careerv1connect"
	"github.com/reh3376/career-site/services/api/internal/email"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Contact implements careerv1connect.ContactServiceHandler.
// Only SubmitContact is wired today; GetContactOptions still returns
// UNIMPLEMENTED from the embedded default until the /contact page needs it.
type Contact struct {
	careerv1connect.UnimplementedContactServiceHandler

	log       *slog.Logger
	users     *users.Repo
	auth      *Auth // for session-cookie lookup
	email     email.Provider
	from      string
	ownerAddr string
	webBase   string
}

func NewContact(
	log *slog.Logger,
	repo *users.Repo,
	auth *Auth,
	mailer email.Provider,
	from, ownerAddr, webBase string,
) *Contact {
	return &Contact{
		log:       log,
		users:     repo,
		auth:      auth,
		email:     mailer,
		from:      from,
		ownerAddr: ownerAddr,
		webBase:   webBase,
	}
}

// SubmitContact accepts a message from either a signed-in member (identity
// from cookie) or an anonymous visitor (name + email on the form). Every
// submission is persisted to support_messages and delivered to
// OWNER_CONTACT_EMAIL. Returns a ticket_id the sender can quote in
// follow-ups.
func (h *Contact) SubmitContact(
	ctx context.Context,
	req *connect.Request[v1.SubmitContactRequest],
) (*connect.Response[v1.SubmitContactResponse], error) {
	msg := req.Msg

	subject := strings.TrimSpace(msg.Subject)
	body := strings.TrimSpace(msg.Message)
	if subject == "" || body == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("subject and message are required"))
	}
	if msg.Category == v1.SupportCategory_SUPPORT_CATEGORY_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("category is required"))
	}

	// Identity: prefer the session cookie; fall back to the form fields.
	var (
		userID      *int64
		senderName  string
		senderEmail string
		authNote    string
	)
	if u, err := h.auth.LookupSessionUser(ctx, req); err == nil && u != nil {
		id := u.ID
		userID = &id
		senderName = u.Name
		senderEmail = u.Email
		authNote = fmt.Sprintf("user id %d", u.ID)
	} else {
		// Anonymous: name + email required + valid.
		name := strings.TrimSpace(msg.Name)
		addr := strings.ToLower(strings.TrimSpace(msg.Email))
		if name == "" || addr == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				errors.New("name and email are required when not signed in"))
		}
		if _, err := mail.ParseAddress(addr); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				errors.New("email does not look valid"))
		}
		// Turnstile verification would go here (D-09 rate-limit follow-up);
		// dev mode accepts anything. Prod flip lands with FR-AUTH-09.
		if msg.TurnstileToken == "" && h.turnstileEnforced() {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("turnstile token is required for anonymous submissions"))
		}
		senderName = name
		senderEmail = addr
	}

	replyChannel := "email"
	if msg.ReplyChannel == v1.SubmitContactRequest_REPLY_CHANNEL_LINKEDIN {
		replyChannel = "linkedin"
	}

	// Hiring-inquiry extras. Persisted only when the category is
	// HIRING_INQUIRY; a submitter with a different category who
	// happens to send these fields sees them dropped silently.
	var hiringRole, hiringJDURL, hiringTargetStart string
	if msg.Category == v1.SupportCategory_SUPPORT_CATEGORY_HIRING_INQUIRY {
		hiringRole = strings.TrimSpace(msg.HiringRole)
		hiringJDURL = strings.TrimSpace(msg.HiringJdUrl)
		hiringTargetStart = strings.TrimSpace(msg.HiringTargetStart)
	}

	stored, err := h.users.CreateSupport(ctx, users.CreateSupportInput{
		Category:          categoryFromProto(msg.Category),
		Subject:           subject,
		Body:              body,
		UserID:            userID,
		SenderName:        senderName,
		SenderEmail:       senderEmail,
		ReplyChannel:      replyChannel,
		IPHash:            hashIPBytes(req.Peer().Addr),
		UserAgent:         req.Header().Get("User-Agent"),
		HiringRole:        hiringRole,
		HiringJDURL:       hiringJDURL,
		HiringTargetStart: hiringTargetStart,
	})
	if err != nil {
		h.log.Error("support insert failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("submit failed"))
	}

	go func() {
		emailCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := h.sendSupportNotification(emailCtx, stored, authNote); err != nil {
			h.log.Error("support-notification email failed",
				slog.String("ticket", stored.TicketID),
				slog.String("error", err.Error()))
		}
	}()

	return connect.NewResponse(&v1.SubmitContactResponse{
		TicketId: stored.TicketID,
	}), nil
}

func (h *Contact) turnstileEnforced() bool {
	// Turnstile hookup ships with the FR-AUTH-09 rate-limit pass.
	// Anonymous submissions are still rate-limited by IP at the repo layer
	// once that lands; for now the form is unprotected in dev.
	return false
}

func (h *Contact) sendSupportNotification(ctx context.Context, m *users.SupportMessage, authNote string) error {
	text, html, err := email.SupportNotificationTemplate.Render(map[string]any{
		"Category":    categoryDisplay(m.Category),
		"Subject":     m.Subject,
		"Body":        m.Body,
		"SenderName":  m.SenderName,
		"SenderEmail": m.SenderEmail,
		"Auth":        authNote,
		"TicketID":    m.TicketID,
		"SubmittedAt": m.CreatedAt.UTC().Format(time.RFC1123),
		"ReviewURL":   h.webBase + "/admin/support",
	})
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	return h.email.Send(ctx, email.Message{
		To:       h.ownerAddr,
		From:     h.from,
		Subject:  fmt.Sprintf("[%s] %s", categoryShort(m.Category), m.Subject),
		TextBody: text,
		HTMLBody: html,
	})
}

func categoryFromProto(c v1.SupportCategory) users.SupportCategory {
	switch c {
	case v1.SupportCategory_SUPPORT_CATEGORY_GENERAL_QUESTION:
		return users.SupportCategoryGeneralQuestion
	case v1.SupportCategory_SUPPORT_CATEGORY_BUG_REPORT:
		return users.SupportCategoryBugReport
	case v1.SupportCategory_SUPPORT_CATEGORY_FEATURE_REQUEST:
		return users.SupportCategoryFeatureRequest
	case v1.SupportCategory_SUPPORT_CATEGORY_CONTRIBUTOR_ACCESS:
		return users.SupportCategoryContributorAccess
	case v1.SupportCategory_SUPPORT_CATEGORY_PRESS_INQUIRY:
		return users.SupportCategoryPressInquiry
	case v1.SupportCategory_SUPPORT_CATEGORY_HIRING_INQUIRY:
		return users.SupportCategoryHiringInquiry
	default:
		return users.SupportCategoryOther
	}
}

// categoryDisplay is the human-readable label used inside the email body.
func categoryDisplay(c users.SupportCategory) string {
	switch c {
	case users.SupportCategoryGeneralQuestion:
		return "General question"
	case users.SupportCategoryBugReport:
		return "Bug report"
	case users.SupportCategoryFeatureRequest:
		return "Feature request"
	case users.SupportCategoryContributorAccess:
		return "Contributor access request"
	case users.SupportCategoryHiringInquiry:
		return "Hiring inquiry"
	case users.SupportCategoryPressInquiry:
		return "Press / interview inquiry"
	default:
		return "Other"
	}
}

// categoryShort is the tag inside the email subject line.
func categoryShort(c users.SupportCategory) string {
	switch c {
	case users.SupportCategoryBugReport:
		return "bug"
	case users.SupportCategoryFeatureRequest:
		return "feature"
	case users.SupportCategoryContributorAccess:
		return "contributor"
	case users.SupportCategoryPressInquiry:
		return "press"
	case users.SupportCategoryGeneralQuestion:
		return "question"
	default:
		return "support"
	}
}
