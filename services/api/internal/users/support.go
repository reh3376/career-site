package users

import (
	"context"
	"fmt"
	"time"
)

type SupportCategory string

const (
	SupportCategoryGeneralQuestion   SupportCategory = "general_question"
	SupportCategoryBugReport         SupportCategory = "bug_report"
	SupportCategoryFeatureRequest    SupportCategory = "feature_request"
	SupportCategoryContributorAccess SupportCategory = "contributor_access"
	SupportCategoryPressInquiry      SupportCategory = "press_inquiry"
	SupportCategoryOther             SupportCategory = "other"
)

type SupportMessage struct {
	ID           int64
	Category     SupportCategory
	Subject      string
	Body         string
	UserID       *int64
	SenderName   string
	SenderEmail  string
	ReplyChannel string
	Status       string
	TicketID     string
	CreatedAt    time.Time
}

type CreateSupportInput struct {
	Category     SupportCategory
	Subject      string
	Body         string
	UserID       *int64
	SenderName   string
	SenderEmail  string
	ReplyChannel string
	IPHash       []byte
	UserAgent    string
}

// CreateSupport inserts a support-message row and returns it with the
// generated ticket_id. Ticket id format: SUPP-<yyyymmdd>-<id>.
func (r *Repo) CreateSupport(ctx context.Context, in CreateSupportInput) (*SupportMessage, error) {
	const q = `
    INSERT INTO support_messages
      (category, subject, body, user_id, sender_name, sender_email,
       reply_channel, ip_hash, user_agent, ticket_id)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'pending')
    RETURNING id, created_at
  `
	m := &SupportMessage{
		Category:     in.Category,
		Subject:      in.Subject,
		Body:         in.Body,
		UserID:       in.UserID,
		SenderName:   in.SenderName,
		SenderEmail:  in.SenderEmail,
		ReplyChannel: in.ReplyChannel,
		Status:       "open",
	}
	if err := r.pool.QueryRow(ctx, q,
		string(in.Category), in.Subject, in.Body, in.UserID, in.SenderName,
		in.SenderEmail, in.ReplyChannel, in.IPHash, in.UserAgent,
	).Scan(&m.ID, &m.CreatedAt); err != nil {
		return nil, fmt.Errorf("insert support: %w", err)
	}
	// Assign a real ticket id now that we have the row id.
	m.TicketID = fmt.Sprintf("SUPP-%s-%d", m.CreatedAt.UTC().Format("20060102"), m.ID)
	if _, err := r.pool.Exec(ctx, `UPDATE support_messages SET ticket_id = $2 WHERE id = $1`, m.ID, m.TicketID); err != nil {
		return nil, fmt.Errorf("assign ticket id: %w", err)
	}
	return m, nil
}
