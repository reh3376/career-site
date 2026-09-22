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
	NotifyJdOutcome(ctx context.Context, s *users.JdSubmission, a *Assessment, threshold float64) error
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
	if err := s.notifier.NotifyJdOutcome(ctx, row, a, s.threshold); err != nil {
		s.log.Warn("jd: outcome mail failed", slog.Int64("id", submissionID), slog.String("error", err.Error()))
	}
}

// OwnerMailer mails the owner about each finished submission.
type OwnerMailer struct {
	email   email.Provider
	from    string
	to      string
	webBase string
	log     *slog.Logger
}

// NewOwnerMailer wires the mailer. to is the owner's address.
func NewOwnerMailer(provider email.Provider, from, to, webBase string, log *slog.Logger) *OwnerMailer {
	return &OwnerMailer{email: provider, from: from, to: to, webBase: strings.TrimRight(webBase, "/"), log: log}
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
}

type jdOutcomeVerdict struct {
	Verdict   string
	Category  string
	Text      string
	Rationale string
}

// NotifyJdOutcome implements OutcomeNotifier.
func (m *OwnerMailer) NotifyJdOutcome(ctx context.Context, s *users.JdSubmission, a *Assessment, threshold float64) error {
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
		Threshold:      fmt.Sprintf("%.2f", threshold),
	}
	if s.MatchScore != nil {
		d.Score = fmt.Sprintf("%.3f", *s.MatchScore)
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
	return m.email.Send(ctx, email.Message{
		From:     m.from,
		To:       m.to,
		Subject:  subject,
		TextBody: text,
		HTMLBody: html,
		Kind:     "jd_outcome",
		UserID:   s.UserID,
	})
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
