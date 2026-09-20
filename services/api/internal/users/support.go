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
	Status       string // "open" | "resolved"
	TicketID     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ResolvedAt   *time.Time
}

// ListSupportFilter narrows a ListSupport call. Zero values mean "no
// filter" so callers can pass in a partly-populated struct.
type ListSupportFilter struct {
	Query    string // case-insensitive substring in subject / body / sender
	Category SupportCategory // "" = any
	Status   string          // "" = any, "open" | "resolved"
	Limit    int32           // 1..200 (clamped)
	Offset   int32           // >= 0
}

// ListSupportResult is a page of messages plus counts for the caller
// to render a summary header. Total is the count matching the filter
// (without Limit/Offset). OpenCount / ResolvedCount ignore the status
// filter — they always report both totals so the header UI can show
// them side-by-side.
type ListSupportResult struct {
	Messages      []SupportMessage
	Total         int32
	OpenCount     int32
	ResolvedCount int32
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

// ListSupport returns a page of support messages plus aggregate
// counts. The query builds up dynamically so unused filters don't
// affect the plan; parameters are bound (never interpolated) so
// there's no SQL injection surface.
func (r *Repo) ListSupport(ctx context.Context, f ListSupportFilter) (*ListSupportResult, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 25
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	// Aggregate counts (unfiltered by status so the header can show
	// open vs resolved simultaneously).
	var open, resolved int32
	if err := r.pool.QueryRow(ctx, `
    SELECT
      COUNT(*) FILTER (WHERE status = 'open'),
      COUNT(*) FILTER (WHERE status = 'resolved')
    FROM support_messages
  `).Scan(&open, &resolved); err != nil {
		return nil, fmt.Errorf("count support: %w", err)
	}

	// Filter clauses shared by count(matches) and the paged select.
	where := "WHERE 1=1"
	args := []any{}
	next := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	if f.Query != "" {
		p := next("%" + f.Query + "%")
		where += " AND (subject ILIKE " + p + " OR body ILIKE " + p +
			" OR sender_name ILIKE " + p + " OR sender_email ILIKE " + p + ")"
	}
	if f.Category != "" {
		where += " AND category = " + next(string(f.Category)) + "::support_category"
	}
	if f.Status != "" {
		where += " AND status = " + next(f.Status) + "::support_status"
	}

	// Total (matching filter, not paged).
	var total int32
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM support_messages "+where, args...,
	).Scan(&total); err != nil {
		return nil, fmt.Errorf("count support (filter): %w", err)
	}

	// Paged select. Newest first.
	limArg := next(f.Limit)
	offArg := next(f.Offset)
	q := `
    SELECT id, category::text, status::text, subject, body, user_id,
           sender_name, sender_email, reply_channel, ticket_id,
           created_at, updated_at, resolved_at
    FROM support_messages
  ` + where + " ORDER BY created_at DESC LIMIT " + limArg + " OFFSET " + offArg

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("select support: %w", err)
	}
	defer rows.Close()

	messages := []SupportMessage{}
	for rows.Next() {
		var m SupportMessage
		var cat, status string
		if err := rows.Scan(
			&m.ID, &cat, &status, &m.Subject, &m.Body, &m.UserID,
			&m.SenderName, &m.SenderEmail, &m.ReplyChannel, &m.TicketID,
			&m.CreatedAt, &m.UpdatedAt, &m.ResolvedAt,
		); err != nil {
			return nil, fmt.Errorf("scan support row: %w", err)
		}
		m.Category = SupportCategory(cat)
		m.Status = status
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate support rows: %w", err)
	}

	return &ListSupportResult{
		Messages:      messages,
		Total:         total,
		OpenCount:     open,
		ResolvedCount: resolved,
	}, nil
}

// SetSupportStatus flips a message between "open" and "resolved" and
// updates updated_at / resolved_at accordingly. Returns the updated
// row.
func (r *Repo) SetSupportStatus(ctx context.Context, id int64, status string) (*SupportMessage, error) {
	if status != "open" && status != "resolved" {
		return nil, fmt.Errorf("invalid status %q", status)
	}
	// resolved_at gets set on the resolved->true transition and cleared
	// on the reverse. Doing it in one statement keeps updated_at and
	// resolved_at in agreement without a read-modify-write dance.
	const q = `
    UPDATE support_messages
    SET status      = $2::support_status,
        updated_at  = now(),
        resolved_at = CASE WHEN $2 = 'resolved' THEN now() ELSE NULL END
    WHERE id = $1
    RETURNING id, category::text, status::text, subject, body, user_id,
              sender_name, sender_email, reply_channel, ticket_id,
              created_at, updated_at, resolved_at
  `
	var m SupportMessage
	var cat, st string
	if err := r.pool.QueryRow(ctx, q, id, status).Scan(
		&m.ID, &cat, &st, &m.Subject, &m.Body, &m.UserID,
		&m.SenderName, &m.SenderEmail, &m.ReplyChannel, &m.TicketID,
		&m.CreatedAt, &m.UpdatedAt, &m.ResolvedAt,
	); err != nil {
		return nil, fmt.Errorf("set support status: %w", err)
	}
	m.Category = SupportCategory(cat)
	m.Status = st
	return &m, nil
}
