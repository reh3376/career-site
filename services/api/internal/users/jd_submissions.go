package users

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"
)

// JdSubmission is one row of jd_submissions.
type JdSubmission struct {
	ID                 int64
	SourceKind         string // "paste" | "pdf" | "text_upload"
	JdText             string
	JdHash             []byte
	TextHead           string
	RoleHint           string
	EmployerHint       string
	ContactEmail       string
	Status             string
	MatchScore         *float64
	GeneratedResumeURL string
	Error              string
	CreatedAt          time.Time
	CompletedAt        *time.Time
}

// JdSubmitInput is what the handler passes to CreateJdSubmission.
// jd_text is authoritative; the repo computes text_head + jd_hash.
type JdSubmitInput struct {
	SourceKind   string
	JdText       string
	RoleHint     string
	EmployerHint string
	ContactEmail string
	IPHash       []byte
	UAHash       []byte
}

// NormaliseJdHash returns the sha256 of the whitespace-collapsed,
// lower-cased jd text. Same JD submitted twice by different pasters
// (spacing, capitalisation) dedupes to the same hash — matches the
// per-JD rate-limit and dedup query.
func NormaliseJdHash(jdText string) []byte {
	// Collapse all runs of whitespace to a single space; strip
	// leading/trailing; lowercase. Cheap; good enough for a dedup
	// signal that's not trying to defeat a determined adversary.
	var b strings.Builder
	prevSpace := true
	for _, r := range strings.ToLower(jdText) {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	normalised := strings.TrimSpace(b.String())
	sum := sha256.Sum256([]byte(normalised))
	return sum[:]
}

// CreateJdSubmission stores one row and returns it with server fields
// (id, created_at) populated.
func (r *Repo) CreateJdSubmission(
	ctx context.Context, in JdSubmitInput,
) (*JdSubmission, error) {
	if in.SourceKind == "" || in.JdText == "" {
		return nil, fmt.Errorf("source_kind and jd_text required")
	}
	head := in.JdText
	if len(head) > 400 {
		head = head[:400]
	}
	hash := NormaliseJdHash(in.JdText)

	const q = `
    INSERT INTO jd_submissions (
      ip_hash, ua_hash, source_kind, jd_text, jd_hash, text_head,
      role_hint, employer_hint, contact_email
    ) VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''), NULLIF($8,''), NULLIF($9,''))
    RETURNING id, created_at, status
  `
	s := &JdSubmission{
		SourceKind:   in.SourceKind,
		JdText:       in.JdText,
		JdHash:       hash,
		TextHead:     head,
		RoleHint:     in.RoleHint,
		EmployerHint: in.EmployerHint,
		ContactEmail: in.ContactEmail,
	}
	err := r.pool.QueryRow(ctx, q,
		in.IPHash, in.UAHash, in.SourceKind, in.JdText, hash, head,
		in.RoleHint, in.EmployerHint, in.ContactEmail,
	).Scan(&s.ID, &s.CreatedAt, &s.Status)
	if err != nil {
		return nil, fmt.Errorf("insert jd submission: %w", err)
	}
	return s, nil
}

const jdCols = `
    id, source_kind, jd_text, jd_hash, text_head,
    COALESCE(role_hint, ''), COALESCE(employer_hint, ''), COALESCE(contact_email, ''),
    status, match_score, COALESCE(generated_resume_url, ''), COALESCE(error, ''),
    created_at, completed_at
`

// UpdateJdScoring records the outcome of the scoring pass. status
// should be one of the JdSubmission state values; error stays
// empty on a happy path. completed_at is stamped only for terminal
// states (below_threshold / ready / failed) so the "still working"
// case leaves it NULL.
func (r *Repo) UpdateJdScoring(
	ctx context.Context, id int64, status string, score *float64, errText string,
) error {
	terminal := status == "below_threshold" || status == "ready" || status == "failed"
	const q = `
    UPDATE jd_submissions
    SET status       = $2,
        match_score  = $3,
        error        = NULLIF($4, ''),
        completed_at = CASE WHEN $5::boolean THEN now() ELSE completed_at END
    WHERE id = $1
  `
	tag, err := r.pool.Exec(ctx, q, id, status, score, errText, terminal)
	if err != nil {
		return fmt.Errorf("update jd scoring: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListJdSubmissions returns every submission newest-first with the
// full JD text omitted (use text_head for a preview; the full body
// loads on the detail view). Capped at 500 rows.
func (r *Repo) ListJdSubmissions(ctx context.Context) ([]JdSubmission, error) {
	const q = `
    SELECT id, source_kind, ''::text /* jd_text elided */, jd_hash, text_head,
           COALESCE(role_hint, ''), COALESCE(employer_hint, ''),
           COALESCE(contact_email, ''),
           status, match_score, COALESCE(generated_resume_url, ''),
           COALESCE(error, ''),
           created_at, completed_at
    FROM jd_submissions
    ORDER BY created_at DESC
    LIMIT 500
  `
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list jd submissions: %w", err)
	}
	defer rows.Close()
	var out []JdSubmission
	for rows.Next() {
		var s JdSubmission
		if err := rows.Scan(
			&s.ID, &s.SourceKind, &s.JdText, &s.JdHash, &s.TextHead,
			&s.RoleHint, &s.EmployerHint, &s.ContactEmail,
			&s.Status, &s.MatchScore, &s.GeneratedResumeURL, &s.Error,
			&s.CreatedAt, &s.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan jd row: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetJdSubmission looks up one row by id.
func (r *Repo) GetJdSubmission(ctx context.Context, id int64) (*JdSubmission, error) {
	s := &JdSubmission{}
	err := r.pool.QueryRow(ctx, `SELECT `+jdCols+` FROM jd_submissions WHERE id = $1`, id).Scan(
		&s.ID, &s.SourceKind, &s.JdText, &s.JdHash, &s.TextHead,
		&s.RoleHint, &s.EmployerHint, &s.ContactEmail,
		&s.Status, &s.MatchScore, &s.GeneratedResumeURL, &s.Error,
		&s.CreatedAt, &s.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get jd submission: %w", err)
	}
	return s, nil
}
