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
	"github.com/reh3376/career-site/services/api/internal/events"
	"github.com/reh3376/career-site/services/api/internal/turnstile"
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
	events    *events.Writer // product event stream; nil is silent
	webBase   string
	// turnstile guards the anonymous submission path (FR-CNT-22).
	// Never nil: NewContact defaults it to turnstile.Noop.
	turnstile turnstile.Verifier
}

// SetTurnstile replaces the bot-challenge verifier. main wires the real
// Cloudflare client here when a secret is configured.
func (h *Contact) SetTurnstile(v turnstile.Verifier) {
	if v != nil {
		h.turnstile = v
	}
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
		turnstile: turnstile.Noop{},
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
		// Anonymous: name + email required + valid, with one exception.
		name := strings.TrimSpace(msg.Name)
		addr := strings.ToLower(strings.TrimSpace(msg.Email))

		// A decision-test participant may write without identifying
		// themselves. The test tells them every field is optional and
		// that the measurement does not need their name, and the page
		// this arrives from repeats it one paragraph above the box.
		// Requiring a name there would mean a participant who took
		// that at its word has to invent one before they can say the
		// test was hard, which is a worse outcome than an unsigned
		// note in the inbox.
		//
		// Turnstile below still applies, so this relaxes who may
		// write, never whether a bot may.
		anon := msg.Category == v1.SupportCategory_SUPPORT_CATEGORY_DECISION_TEST_FEEDBACK
		if !anon && (name == "" || addr == "") {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				errors.New("name and email are required when not signed in"))
		}
		// An address that is given is still checked. Feedback with a
		// typo'd address is worse than feedback with none: it reads as
		// a reply the sender is waiting for, and the reply bounces.
		if addr != "" {
			if _, err := mail.ParseAddress(addr); err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument,
					errors.New("email does not look valid"))
			}
		}
		// Turnstile (FR-CNT-22), anonymous path only: a signed-in
		// member has already cleared the challenge at registration.
		// Same policy as Register, from the same place: a token
		// Cloudflare rejects is refused, a Cloudflare we cannot reach
		// admits with a logged error.
		if verdict, err := (turnstile.FailOpen{Inner: h.turnstile}).
			Verify(ctx, msg.TurnstileToken, ClientIP(req)); err != nil {
			h.log.Error("turnstile unreachable; admitting anonymous contact unverified",
				slog.String("error", err.Error()))
		} else if !verdict.OK {
			if verdict.InvalidSecret() {
				h.log.Error("turnstile rejected our secret; anonymous contact is blocked for everyone",
					slog.Any("error_codes", verdict.ErrorCodes))
			}
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("bot check failed; reload the page and try again"))
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

	// The decision-test run this came from, kept only when it names a
	// run that actually finished.
	//
	// Verified rather than trusted, because the whole value of the
	// field to the admin console is that it distinguishes somebody who
	// took the test from somebody who says they did. An id that names
	// nothing, or names a run still in progress, is dropped and the
	// message is stored without it: feedback is worth keeping either
	// way, and silently storing an unverified claim as though it were
	// checked is the failure this is meant to avoid.
	var dtSession string
	if msg.Category == v1.SupportCategory_SUPPORT_CATEGORY_DECISION_TEST_FEEDBACK {
		if key := strings.TrimSpace(msg.DtSessionPublicId); key != "" {
			sess, err := h.users.DTSessionByKey(ctx, key)
			switch {
			case err != nil:
				h.log.Warn("decision-test feedback names a session that could not be read",
					slog.String("error", err.Error()))
			case sess.Status != "completed":
				h.log.Warn("decision-test feedback names a run that did not finish",
					slog.String("status", sess.Status))
			default:
				dtSession = sess.PublicID
			}
		}
	}

	stored, err := h.users.CreateSupport(ctx, users.CreateSupportInput{
		Category:          categoryFromProto(msg.Category),
		DTSessionPublicID: dtSession,
		Subject:           subject,
		Body:              body,
		UserID:            userID,
		SenderName:        senderName,
		SenderEmail:       senderEmail,
		ReplyChannel:      replyChannel,
		IPHash:            hashIPBytes(ClientIP(req)),
		UserAgent:         req.Header().Get("User-Agent"),
		HiringRole:        hiringRole,
		HiringJDURL:       hiringJDURL,
		HiringTargetStart: hiringTargetStart,
	})
	if err != nil {
		h.log.Error("support insert failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("submit failed"))
	}
	var eventUser int64
	if userID != nil {
		eventUser = *userID
	}
	h.events.Emit(ctx, requestEvent(req, "contact.submitted", eventUser,
		map[string]any{"category": string(stored.Category), "has_jd": hiringJDURL != ""}))

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

func (h *Contact) sendSupportNotification(ctx context.Context, m *users.SupportMessage, authNote string) error {
	// Decision-test feedback may be unsigned by design, so the template
	// is given something to print rather than two empty strings, which
	// read as a bug in the mail rather than as a deliberate choice by
	// the sender.
	senderName, senderEmail := m.SenderName, m.SenderEmail
	if senderName == "" {
		senderName = "(not given)"
	}
	if senderEmail == "" {
		senderEmail = "(not given, no reply possible)"
	}
	// The run, when there is one, so a remark can be read beside its
	// own numbers without first going to the console to find out
	// whether there is a run at all.
	if m.DTSessionPublicID != "" {
		authNote = strings.TrimSpace(authNote + " · finished decision-test run " +
			m.DTSessionPublicID)
	}

	text, html, err := email.SupportNotificationTemplate.Render(map[string]any{
		"Category":    categoryDisplay(m.Category),
		"Subject":     m.Subject,
		"Body":        m.Body,
		"SenderName":  senderName,
		"SenderEmail": senderEmail,
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
		Kind:     "contact_owner",
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
	case v1.SupportCategory_SUPPORT_CATEGORY_DECISION_TEST_FEEDBACK:
		return users.SupportCategoryDecisionTestFeedback
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
	case users.SupportCategoryDecisionTestFeedback:
		return "Decision test feedback"
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
	case users.SupportCategoryDecisionTestFeedback:
		return "decision test"
	default:
		return "support"
	}
}
