package jd

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/reh3376/career-site/services/api/internal/email"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// OutcomeNotifier is told once per submission when the pipeline
// reaches a terminal state (ready, below_threshold, failed). The
// owner-facing implementation mails the score, the verdict summary,
// the application link and the admin links; the /jd-upload page has
// promised "the full JD lands in Roger's inbox with a match score"
// since it shipped, and this is what keeps that promise.
type OutcomeNotifier interface {
	NotifyJdOutcome(ctx context.Context, s *users.JdSubmission, a *Assessment, bands Bands) error
}

// SetNotifier installs the outcome notifier. Nil disables it.
func (s *Scorer) SetNotifier(n OutcomeNotifier) { s.notifier = n }

// notifyOutcome runs after ScoreAndPersist, whatever path it took. It
// reads the row back so the mail reflects exactly what was stored.
func (s *Scorer) notifyOutcome(ctx context.Context, submissionID int64) {
	if s.notifier == nil {
		return
	}
	row, err := s.users.GetJdSubmission(ctx, submissionID)
	if err != nil {
		s.log.Warn("jd: outcome mail skipped, row unreadable", slog.Int64("id", submissionID), slog.String("error", err.Error()))
		return
	}
	switch row.Status {
	case "ready", "below_threshold", "failed":
	default:
		return
	}
	var a *Assessment
	if len(row.Assessment) > 0 {
		var parsed Assessment
		if json.Unmarshal(row.Assessment, &parsed) == nil {
			a = &parsed
		}
	}
	if err := s.notifier.NotifyJdOutcome(ctx, row, a, s.bands.Get(ctx)); err != nil {
		s.log.Warn("jd: outcome mail failed", slog.Int64("id", submissionID), slog.String("error", err.Error()))
	}
}

// OwnerMailer mails the owner and the submitter about each finished
// submission.
type OwnerMailer struct {
	email   email.Provider
	users   *users.Repo
	from    string
	to      string
	webBase string
	log     *slog.Logger
}

// NewOwnerMailer wires the mailer. to is the owner's address; repo is
// used to fetch the résumé PDF for attachment.
func NewOwnerMailer(provider email.Provider, repo *users.Repo, from, to, webBase string, log *slog.Logger) *OwnerMailer {
	return &OwnerMailer{email: provider, users: repo, from: from, to: to, webBase: strings.TrimRight(webBase, "/"), log: log}
}

// jdOutcomeData is the template payload.
type jdOutcomeData struct {
	ID             string
	StatusLabel    string
	Headline       string
	Score          string
	Threshold      string
	RetrievalScore string
	Role           string
	Employer       string
	SubmitterEmail string
	ContactEmail   string
	ApplyURL       string
	Requirements   int
	Met            int
	Partial        int
	Unmet          int
	UnmetList      []string
	Verdicts       []jdOutcomeVerdict
	Error          string
	TextHead       string
	SubmittedAt    string
	AdminURL       string
	DecisionsURL   string
	PDFURL         string
	// ReviewURL is the submitter's own page for this review.
	ReviewURL string
	// Fit category and its wording (very strong, strong, possible,
	// weak, very weak); empty when no score was computed.
	Category      string
	CategoryLabel string
	// GoodFit is true for very strong and strong: the résumé is attached.
	GoodFit bool
	// WillReview is true for possible and weak: Roger reads it himself.
	WillReview bool
}

type jdOutcomeVerdict struct {
	Verdict   string
	Category  string
	Text      string
	Rationale string
}

// NotifyJdOutcome implements OutcomeNotifier.
func (m *OwnerMailer) NotifyJdOutcome(ctx context.Context, s *users.JdSubmission, a *Assessment, bands Bands) error {
	if m == nil || m.to == "" {
		return nil
	}
	id := fmt.Sprintf("%d", s.ID)
	d := jdOutcomeData{
		ID:             id,
		Role:           s.RoleHint,
		Employer:       s.EmployerHint,
		SubmitterEmail: s.SubmitterEmail,
		ContactEmail:   s.ContactEmail,
		ApplyURL:       s.ApplyURL,
		Error:          s.Error,
		TextHead:       s.TextHead,
		SubmittedAt:    s.CreatedAt.UTC().Format(time.RFC1123),
		AdminURL:       m.webBase + "/admin/jd/" + id,
		DecisionsURL:   m.webBase + "/admin/decisions?ref=" + id,
		ReviewURL:      m.webBase + "/jd-upload/" + id,
		Threshold:      fmt.Sprintf("%.2f", bands.Strong),
	}
	if s.MatchScore != nil {
		d.Score = fmt.Sprintf("%.3f", *s.MatchScore)
		d.Category = bands.Category(*s.MatchScore)
		d.CategoryLabel = FitLabel(d.Category)
		d.GoodFit = d.Category == FitVeryStrong || d.Category == FitStrong
		d.WillReview = d.Category == FitPossible || d.Category == FitWeak
	}
	if s.RetrievalScore != nil {
		d.RetrievalScore = fmt.Sprintf("%.3f", *s.RetrievalScore)
	}
	if s.GeneratedResumeURL != "" && len(s.ResultToken) > 0 {
		d.PDFURL = m.webBase + s.GeneratedResumeURL + "?t=" + hex.EncodeToString(s.ResultToken)
	}
	if a != nil && a.Error == "" {
		byReq := map[string]Judgment{}
		for _, j := range a.Judgments {
			byReq[j.RequirementID] = j
		}
		d.Requirements = len(a.Requirements)
		for _, r := range a.Requirements {
			j := byReq[r.ID]
			v := j.Verdict
			if v == "" {
				v = "unmet"
			}
			switch v {
			case "met":
				d.Met++
			case "partial":
				d.Partial++
			default:
				d.Unmet++
				d.UnmetList = append(d.UnmetList, r.Text)
			}
			d.Verdicts = append(d.Verdicts, jdOutcomeVerdict{Verdict: v, Category: r.Category, Text: r.Text, Rationale: j.Rationale})
		}
	}
	switch s.Status {
	case "ready":
		d.StatusLabel = "résumé ready"
	case "below_threshold":
		d.StatusLabel = "below the gate"
	default:
		d.StatusLabel = "failed"
	}
	what := strings.TrimSpace(strings.Join(nonEmpty(s.RoleHint, s.EmployerHint), " at "))
	if what == "" {
		what = "untitled posting"
	}
	d.Headline = what
	subject := fmt.Sprintf("[JD #%s] %s: %s", id, what, d.StatusLabel)
	if d.Score != "" {
		subject = fmt.Sprintf("[JD #%s] %s: %s, %s", id, what, d.Score, d.StatusLabel)
	}

	text, html, err := email.JdOutcomeTemplate.Render(d)
	if err != nil {
		return fmt.Errorf("render jd outcome: %w", err)
	}
	ownerErr := m.email.Send(ctx, email.Message{
		From:     m.from,
		To:       m.to,
		Subject:  subject,
		TextBody: text,
		HTMLBody: html,
		Kind:     "jd_outcome",
		UserID:   s.UserID,
	})

	// The submitter hears too: their account address, plus the contact
	// address from the form when it differs. The mail carries the fit
	// category, the score, the summary and the link to their own review
	// page (which works from any signed-in session), never the admin
	// links. For a strong or very strong fit the tailored résumé PDF is
	// attached (owner's spec, 2026-09-22).
	var attachments []email.Attachment
	if d.GoodFit && s.GeneratedResumeURL != "" {
		if pdf, _, _, err := m.users.GetJdResumePDF(ctx, s.ID); err == nil && len(pdf) > 0 {
			attachments = append(attachments, email.Attachment{
				Filename:    fmt.Sprintf("roger-henley-resume-jd-%s.pdf", id),
				ContentType: "application/pdf",
				Data:        pdf,
			})
		} else if err != nil {
			m.log.Warn("jd: résumé pdf not attached", slog.Int64("jd_id", s.ID), slog.String("error", err.Error()))
		}
	}
	subjectFor := func() string {
		if d.CategoryLabel == "" {
			return fmt.Sprintf("Your JD review is finished: %s", what)
		}
		return fmt.Sprintf("Your JD review is finished: %s (%s fit)", what, d.CategoryLabel)
	}
	for _, to := range submitterAddresses(s) {
		rt, rh, err := email.JdResultTemplate.Render(d)
		if err != nil {
			return fmt.Errorf("render jd result: %w", err)
		}
		if err := m.email.Send(ctx, email.Message{
			From:        m.from,
			To:          to,
			Subject:     subjectFor(),
			TextBody:    rt,
			HTMLBody:    rh,
			Kind:        "jd_result",
			UserID:      s.UserID,
			Attachments: attachments,
		}); err != nil {
			m.log.Warn("jd: submitter mail failed", slog.Int64("jd_id", s.ID), slog.String("to", to), slog.String("error", err.Error()))
		}
	}
	return ownerErr
}

// submitterAddresses returns the distinct addresses a finished review
// is sent to: the member's account email, then the form's contact
// email when it differs.
func submitterAddresses(s *users.JdSubmission) []string {
	var out []string
	seen := map[string]bool{}
	for _, a := range []string{s.SubmitterEmail, s.ContactEmail} {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" || seen[a] || !strings.Contains(a, "@") {
			continue
		}
		seen[a] = true
		out = append(out, a)
	}
	return out
}

func nonEmpty(parts ...string) []string {
	var out []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, strings.TrimSpace(p))
		}
	}
	return out
}
