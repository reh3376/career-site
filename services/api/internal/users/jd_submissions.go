package users

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
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
	ApplyURL           string
	Status             string
	MatchScore         *float64
	GeneratedResumeURL string
	Error              string
	CreatedAt          time.Time
	CompletedAt        *time.Time
	// ResultToken is issued once at submit; the public poll must present
	// it to read ResumeMarkdown.
	ResultToken    []byte
	ResumeMarkdown string
	LLMModel       string
	PromptID       string
	PromptVersion  int32
	// RetrievalScore is the cheap cosine pre-score; MatchScore is the
	// requirement-weighted gate. Assessment is the raw jsonb derivation.
	RetrievalScore *float64
	Assessment     []byte
	// UserID is the submitting member (0 for pre-gate rows);
	// SubmitterEmail is joined for the admin views.
	UserID         int64
	SubmitterEmail string
}

// TokenMatches reports whether presented equals the stored token,
// in constant time. A row without a token never matches.
func (s *JdSubmission) TokenMatches(presented []byte) bool {
	return len(s.ResultToken) > 0 && subtle.ConstantTimeCompare(s.ResultToken, presented) == 1
}

// JdSubmitInput is what the handler passes to CreateJdSubmission.
// jd_text is authoritative; the repo computes text_head + jd_hash.
type JdSubmitInput struct {
	SourceKind   string
	JdText       string
	RoleHint     string
	EmployerHint string
	ContactEmail string
	ApplyURL     string
	IPHash       []byte
	UAHash       []byte
	UserID       int64 // submitting member; required
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
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return nil, fmt.Errorf("result token: %w", err)
	}

	const q = `
    INSERT INTO jd_submissions (
      ip_hash, ua_hash, source_kind, jd_text, jd_hash, text_head,
      role_hint, employer_hint, contact_email, result_token, user_id, apply_url
    ) VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''), NULLIF($8,''), NULLIF($9,''), $10, NULLIF($11, 0), NULLIF($12,''))
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
		ApplyURL:     in.ApplyURL,
		ResultToken:  token,
		UserID:       in.UserID,
	}
	err := r.pool.QueryRow(ctx, q,
		in.IPHash, in.UAHash, in.SourceKind, in.JdText, hash, head,
		in.RoleHint, in.EmployerHint, in.ContactEmail, token, in.UserID, in.ApplyURL,
	).Scan(&s.ID, &s.CreatedAt, &s.Status)
	if err != nil {
		return nil, fmt.Errorf("insert jd submission: %w", err)
	}
	return s, nil
}

const jdCols = `
    s.id, s.source_kind, s.jd_text, s.jd_hash, s.text_head,
    COALESCE(s.role_hint, ''), COALESCE(s.employer_hint, ''), COALESCE(s.contact_email, ''),
    COALESCE(s.apply_url, ''),
    s.status, s.match_score, COALESCE(s.generated_resume_url, ''), COALESCE(s.error, ''),
    s.created_at, s.completed_at,
    s.result_token, COALESCE(s.resume_markdown, ''), COALESCE(s.llm_model, ''),
    COALESCE(s.prompt_id, ''), COALESCE(s.prompt_version, 0),
    s.retrieval_score, COALESCE(s.assessment::text, ''),
    COALESCE(s.user_id, 0), COALESCE(u.email, '')
`

// SetJdAssessment stores the retrieval pre-score and the assessment
// derivation (nil clears it). Called before the status flip so a
// reader never sees a score without its explanation.
func (r *Repo) SetJdAssessment(ctx context.Context, id int64, retrievalScore float64, assessment []byte) error {
	var doc any
	if len(assessment) > 0 {
		doc = string(assessment)
	}
	const q = `
    UPDATE jd_submissions
    SET retrieval_score = $2, assessment = $3::jsonb
    WHERE id = $1
  `
	tag, err := r.pool.Exec(ctx, q, id, retrievalScore, doc)
	if err != nil {
		return fmt.Errorf("set jd assessment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetJdResumePDF stores the rendered PDF and the download URL the
// public poll hands out.
func (r *Repo) SetJdResumePDF(ctx context.Context, id int64, pdf []byte, pages int32, url string) error {
	const q = `
    UPDATE jd_submissions
    SET resume_pdf = $2, resume_pdf_pages = $3, generated_resume_url = $4
    WHERE id = $1
  `
	tag, err := r.pool.Exec(ctx, q, id, pdf, pages, url)
	if err != nil {
		return fmt.Errorf("set jd resume pdf: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetJdResumePDF returns the PDF bytes, the row's result token, and the
// submitting member id so the download handler can gate on both. pdf
// is nil when not rendered.
func (r *Repo) GetJdResumePDF(ctx context.Context, id int64) (pdf []byte, token []byte, userID int64, err error) {
	const q = `SELECT resume_pdf, result_token, COALESCE(user_id, 0) FROM jd_submissions WHERE id = $1`
	if err := r.pool.QueryRow(ctx, q, id).Scan(&pdf, &token, &userID); err != nil {
		return nil, nil, 0, fmt.Errorf("get jd resume pdf: %w", err)
	}
	return pdf, token, userID, nil
}

// SetJdResume stores the verified résumé (structured JSON plus the
// markdown rendered from it) and flips the row to ready.
func (r *Repo) SetJdResume(
	ctx context.Context, id int64, resumeJSON []byte, markdown, model, promptID string, promptVersion int,
) error {
	var doc any
	if len(resumeJSON) > 0 {
		doc = string(resumeJSON)
	}
	const q = `
    UPDATE jd_submissions
    SET status = 'ready', resume_json = $6::jsonb, resume_markdown = $2, llm_model = $3,
        prompt_id = $4, prompt_version = $5, error = NULL, completed_at = now()
    WHERE id = $1
  `
	tag, err := r.pool.Exec(ctx, q, id, markdown, model, promptID, promptVersion, doc)
	if err != nil {
		return fmt.Errorf("set jd resume: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

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
    SELECT s.id, s.source_kind, ''::text /* jd_text elided */, s.jd_hash, s.text_head,
           COALESCE(s.role_hint, ''), COALESCE(s.employer_hint, ''),
           COALESCE(s.contact_email, ''), COALESCE(s.apply_url, ''),
           s.status, s.match_score, COALESCE(s.generated_resume_url, ''),
           COALESCE(s.error, ''),
           s.created_at, s.completed_at,
           CASE WHEN s.resume_markdown IS NULL THEN '' ELSE 'y' END /* presence only */,
           COALESCE(s.llm_model, ''), COALESCE(s.prompt_id, ''), COALESCE(s.prompt_version, 0),
           s.retrieval_score, COALESCE(s.user_id, 0), COALESCE(u.email, '')
    FROM jd_submissions s
    LEFT JOIN users u ON u.id = s.user_id
    ORDER BY s.created_at DESC
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
			&s.RoleHint, &s.EmployerHint, &s.ContactEmail, &s.ApplyURL,
			&s.Status, &s.MatchScore, &s.GeneratedResumeURL, &s.Error,
			&s.CreatedAt, &s.CompletedAt,
			&s.ResumeMarkdown, &s.LLMModel, &s.PromptID, &s.PromptVersion,
			&s.RetrievalScore, &s.UserID, &s.SubmitterEmail,
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
	err := r.pool.QueryRow(ctx,
		`SELECT `+jdCols+` FROM jd_submissions s LEFT JOIN users u ON u.id = s.user_id WHERE s.id = $1`, id,
	).Scan(
		&s.ID, &s.SourceKind, &s.JdText, &s.JdHash, &s.TextHead,
		&s.RoleHint, &s.EmployerHint, &s.ContactEmail, &s.ApplyURL,
		&s.Status, &s.MatchScore, &s.GeneratedResumeURL, &s.Error,
		&s.CreatedAt, &s.CompletedAt,
		&s.ResultToken, &s.ResumeMarkdown, &s.LLMModel, &s.PromptID, &s.PromptVersion,
		&s.RetrievalScore, &s.Assessment, &s.UserID, &s.SubmitterEmail,
	)
	if err != nil {
		return nil, fmt.Errorf("get jd submission: %w", err)
	}
	return s, nil
}

// ListJdSubmissionsByUser returns one member's submissions, newest
// first, without the JD body, for the "your submissions" list.
func (r *Repo) ListJdSubmissionsByUser(ctx context.Context, userID int64, limit int) ([]JdSubmission, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	const q = `
    SELECT s.id, s.status, s.match_score,
           COALESCE(s.role_hint, ''), COALESCE(s.employer_hint, ''),
           s.created_at, s.completed_at,
           CASE WHEN s.resume_markdown IS NULL THEN '' ELSE 'y' END
    FROM jd_submissions s
    WHERE s.user_id = $1
    ORDER BY s.created_at DESC
    LIMIT $2
  `
	rows, err := r.pool.Query(ctx, q, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list jd submissions by user: %w", err)
	}
	defer rows.Close()
	var out []JdSubmission
	for rows.Next() {
		var s JdSubmission
		if err := rows.Scan(&s.ID, &s.Status, &s.MatchScore, &s.RoleHint, &s.EmployerHint,
			&s.CreatedAt, &s.CompletedAt, &s.ResumeMarkdown); err != nil {
			return nil, fmt.Errorf("scan jd row: %w", err)
		}
		s.UserID = userID
		out = append(out, s)
	}
	return out, rows.Err()
}
