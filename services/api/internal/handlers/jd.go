package handlers

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/gen/career/v1/careerv1connect"
	"github.com/reh3376/career-site/services/api/internal/events"
	"github.com/reh3376/career-site/services/api/internal/jd"
	"github.com/reh3376/career-site/services/api/internal/prompts"
	"github.com/reh3376/career-site/services/api/internal/ratelimit"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Jd implements careerv1connect.JdServiceHandler for the public
// JD-upload flow: persist, then score + assess (and later generate)
// out of band while the caller polls GetJdResult.
type Jd struct {
	careerv1connect.UnimplementedJdServiceHandler

	log     *slog.Logger
	users   *users.Repo
	auth    *Auth // session lookup; JD upload is members-only
	limiter *ratelimit.Limiter
	scorer  *jd.Scorer // nil in dev without a sidecar; SubmitJd skips scoring.
	// pipelineTimeout bounds one submission's background run. CPU
	// inference can take minutes per LLM call.
	pipelineTimeout time.Duration
	// events is the product event stream; nil is silent.
	events *events.Writer
}

func NewJd(log *slog.Logger, repo *users.Repo, auth *Auth, scorer *jd.Scorer, pipelineTimeout time.Duration) *Jd {
	if pipelineTimeout <= 0 {
		pipelineTimeout = 15 * time.Minute
	}
	return &Jd{
		log:   log,
		users: repo,
		auth:  auth,
		// 5-burst per (ip, jd_hash), refill to 5 over 15 min. Blocks
		// a paster hammering "Submit" and a botnet trying to fuzz.
		limiter:         ratelimit.New(5, 5.0/(15*60)),
		scorer:          scorer,
		pipelineTimeout: pipelineTimeout,
	}
}

// SubmitJd accepts a job description and persists it. Returns the
// submission id + a fixed fallback message the /jd-upload page renders
// back to the caller. The real work (score + generate) runs out of
// band and updates the row's status; the caller polls GetJdResult
// for progress.
func (h *Jd) SubmitJd(
	ctx context.Context,
	req *connect.Request[v1.SubmitJdRequest],
) (*connect.Response[v1.SubmitJdResponse], error) {
	member, err := h.requireMember(ctx, req)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	text := strings.TrimSpace(msg.JdText)
	if text == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("paste the JD text (byte-body upload lands in a follow-up)"))
	}
	if len(text) < 100 {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("that JD looks awfully short; we need at least 100 characters to work with"))
	}

	applyURL, err := cleanApplyURL(msg.ApplyUrl)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	source := jdSourceProtoToRepo(msg.Source)
	if source == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("source is required"))
	}

	// Rate-limit on (member, jd_hash) so a submitter cannot hammer,
	// and the same JD can't be re-submitted many times to game
	// future scoring.
	hash := users.NormaliseJdHash(text)
	key := "jd:" + strconv.FormatInt(member.ID, 10) + "|" + string(hash)
	if ok, retry := h.limiter.Allow(key); !ok {
		h.log.Warn("jd submit rate limited",
			slog.String("peer", ClientIP(req)),
			slog.Duration("retry_after", retry),
		)
		return nil, connect.NewError(connect.CodeResourceExhausted,
			errors.New("too many submissions from your address; try again shortly"))
	}

	ipHash := hashIPBytes(ClientIP(req))
	uaHash := shortHash(req.Header().Get("User-Agent"))

	s, err := h.users.CreateJdSubmission(ctx, users.JdSubmitInput{
		SourceKind:   source,
		JdText:       text,
		RoleHint:     strings.TrimSpace(msg.RoleHint),
		EmployerHint: strings.TrimSpace(msg.EmployerHint),
		ContactEmail: strings.ToLower(strings.TrimSpace(msg.ContactEmail)),
		ApplyURL:     applyURL,
		IPHash:       ipHash,
		UAHash:       uaHash,
		UserID:       member.ID,
	})
	if err != nil {
		h.log.Error("CreateJdSubmission failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("submission failed"))
	}

	h.log.Info("jd submitted",
		slog.Int64("id", s.ID),
		slog.Int64("user_id", member.ID),
		slog.String("source", source),
		slog.Int("chars", len(text)),
		slog.String("role_hint", s.RoleHint),
	)
	h.events.Emit(ctx, requestEvent(req, "jd.submitted", member.ID,
		map[string]any{"submission_id": s.ID, "chars": len(text), "has_apply_url": s.ApplyURL != ""}))

	// Fire-and-forget score in the background so the RPC returns
	// immediately. Uses a fresh context (not `ctx`, which cancels
	// as soon as the caller disconnects) bounded by the pipeline
	// timeout. Nil scorer (dev without sidecar) leaves the row in
	// RECEIVED for manual triage.
	if h.scorer != nil {
		hints := prompts.Hints{Role: s.RoleHint, Employer: s.EmployerHint}
		// No deadline here: the scorer caps the queue wait and applies the
		// pipeline timeout itself once the submission holds its slot.
		go func(id int64, jdText string) {
			h.scorer.ScoreAndPersist(context.Background(), id, jdText, hints)
		}(s.ID, text)
	}

	return connect.NewResponse(&v1.SubmitJdResponse{
		SubmissionId: strconv.FormatInt(s.ID, 10),
		Status:       jdStatusRepoToProto(s.Status),
		Message:      fixedSubmitAckMessage(),
		ResultToken:  hex.EncodeToString(s.ResultToken),
	}), nil
}

// GetJdResult polls a submission. Today just reflects the stored
// row — no async worker exists yet, so status stays at "received"
// until scoring lands. The endpoint is wired now so the frontend
// doesn't need a rewrite when the pipeline goes live.
func (h *Jd) GetJdResult(
	ctx context.Context,
	req *connect.Request[v1.GetJdResultRequest],
) (*connect.Response[v1.GetJdResultResponse], error) {
	member, err := h.requireMember(ctx, req)
	if err != nil {
		return nil, err
	}
	id, err := strconv.ParseInt(req.Msg.SubmissionId, 10, 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid submission_id"))
	}
	s, err := h.users.GetJdSubmission(ctx, id)
	if err != nil {
		// Any DB error, including no-rows, comes back as NotFound
		// so the endpoint doesn't leak internal detail.
		return nil, connect.NewError(connect.CodeNotFound, errors.New("submission not found"))
	}
	// A member sees only their own submissions; admins see all.
	if s.UserID != member.ID && member.Role != users.RoleAdmin {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("submission not found"))
	}
	out := &v1.GetJdResultResponse{
		Status:             jdStatusRepoToProto(s.Status),
		GeneratedResumeUrl: s.GeneratedResumeURL,
		ErrorMessage:       s.Error,
		CreatedAt:          timestamppb.New(s.CreatedAt),
	}
	if s.MatchScore != nil {
		out.MatchScore = s.MatchScore
	}
	if s.CompletedAt != nil {
		out.CompletedAt = timestamppb.New(*s.CompletedAt)
	}
	// The résumé (and its PDF link) is only released to a caller holding
	// the token that was issued at submit time; the numeric id alone
	// reveals nothing beyond status and score.
	out.GeneratedResumeUrl = ""
	tok := strings.TrimSpace(req.Msg.ResultToken)
	presented, tokErr := hex.DecodeString(tok)
	holdsToken := tokErr == nil && tok != "" && s.TokenMatches(presented)
	// The member who submitted it may reopen the review from a fresh
	// session (the token only ever lived in the tab that submitted).
	// Ownership releases the same things the token does, and the PDF
	// link is built with the stored token so the download route,
	// which still checks it, accepts it.
	if s.UserID != 0 && s.UserID == member.ID {
		holdsToken = true
		tok = hex.EncodeToString(s.ResultToken)
	}
	if s.ResumeMarkdown != "" && holdsToken {
		out.ResumeMarkdown = s.ResumeMarkdown
		if s.GeneratedResumeURL != "" {
			out.GeneratedResumeUrl = s.GeneratedResumeURL + "?t=" + tok
		}
	}
	// The verdict breakdown is released with the token whatever the
	// outcome: a below-threshold result should say what was and was not
	// evidenced, not just a number. Rationales never quote private
	// chunks (judge prompt rule 4), so they are safe to show the
	// person who submitted the posting.
	if holdsToken && len(s.Assessment) > 0 {
		out.Verdicts, out.MetCount, out.PartialCount, out.UnmetCount = verdictBreakdown(s.Assessment)
	}
	if h.scorer != nil {
		out.MatchThreshold = h.scorer.Threshold()
		if s.MatchScore != nil {
			out.FitCategory = h.scorer.Bands(ctx).Category(*s.MatchScore)
		}
	}
	out.ProgressPct = s.ProgressPct
	out.ProgressStage = s.ProgressStage
	return connect.NewResponse(out), nil
}

// GetJdReviewConfig returns the fit bands in force.
func (h *Jd) GetJdReviewConfig(
	ctx context.Context,
	req *connect.Request[v1.GetJdReviewConfigRequest],
) (*connect.Response[v1.GetJdReviewConfigResponse], error) {
	if _, err := h.requireMember(ctx, req); err != nil {
		return nil, err
	}
	b := jd.DefaultBands(0)
	if h.scorer != nil {
		b = h.scorer.Bands(ctx)
	}
	return connect.NewResponse(&v1.GetJdReviewConfigResponse{Bands: bandsToProto(b)}), nil
}

func bandsToProto(b jd.Bands) *v1.JdFitBands {
	return &v1.JdFitBands{VeryStrong: b.VeryStrong, Strong: b.Strong, Possible: b.Possible, Weak: b.Weak}
}

// ListMySubmissions returns the caller's own submissions, newest first.
func (h *Jd) ListMySubmissions(
	ctx context.Context,
	req *connect.Request[v1.ListMySubmissionsRequest],
) (*connect.Response[v1.ListMySubmissionsResponse], error) {
	member, err := h.requireMember(ctx, req)
	if err != nil {
		return nil, err
	}
	rows, err := h.users.ListJdSubmissionsByUser(ctx, member.ID, 100)
	if err != nil {
		h.log.Error("ListMySubmissions failed", slog.String("error", err.Error()))
		return nil, connect.NewError(connect.CodeInternal, errors.New("list failed"))
	}
	out := &v1.ListMySubmissionsResponse{Submissions: make([]*v1.MySubmission, 0, len(rows))}
	for i := range rows {
		r := &rows[i]
		m := &v1.MySubmission{
			Id:           strconv.FormatInt(r.ID, 10),
			Status:       jdStatusRepoToProto(r.Status),
			RoleHint:     r.RoleHint,
			EmployerHint: r.EmployerHint,
			CreatedAt:    timestamppb.New(r.CreatedAt),
			HasResume:    r.ResumeMarkdown != "",
		}
		if r.MatchScore != nil {
			score := *r.MatchScore
			m.MatchScore = &score
			if h.scorer != nil {
				m.FitCategory = h.scorer.Bands(ctx).Category(score)
			}
		}
		m.ProgressPct = r.ProgressPct
		if r.CompletedAt != nil {
			m.CompletedAt = timestamppb.New(*r.CompletedAt)
		}
		out.Submissions = append(out.Submissions, m)
	}
	return connect.NewResponse(out), nil
}

// verdictBreakdown flattens the stored assessment into per-requirement
// verdicts in requirement order, with counts. An assessment that
// recorded an error (retrieval fallback) yields nothing.
func verdictBreakdown(raw []byte) (out []*v1.RequirementVerdict, met, partial, unmet int32) {
	var a jd.Assessment
	if err := json.Unmarshal(raw, &a); err != nil || a.Error != "" {
		return nil, 0, 0, 0
	}
	byReq := map[string]jd.Judgment{}
	for _, j := range a.Judgments {
		byReq[j.RequirementID] = j
	}
	for _, r := range a.Requirements {
		j, ok := byReq[r.ID]
		if !ok {
			j = jd.Judgment{RequirementID: r.ID, Verdict: "unmet"}
		}
		switch j.Verdict {
		case "met":
			met++
		case "partial":
			partial++
		default:
			unmet++
		}
		out = append(out, &v1.RequirementVerdict{
			Id: r.ID, Text: r.Text, Category: r.Category, Weight: int32(r.Weight),
			Verdict: j.Verdict, Rationale: j.Rationale,
		})
	}
	return out, met, partial, unmet
}

// cleanApplyURL accepts an empty value or an absolute http(s) URL with
// a host. Anything else is rejected so the admin view never renders a
// javascript: or relative link from a submitter.
func cleanApplyURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if len(raw) > 2048 {
		return "", errors.New("application link is too long")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errors.New("application link must be a full http(s) URL")
	}
	return u.String(), nil
}

// requireMember resolves the caller's session or fails the RPC. JD
// upload is members-only; the proto declares AUTH_LEVEL_MEMBER but
// enforcement lives here until the auth interceptor lands.
func (h *Jd) requireMember(ctx context.Context, req connect.AnyRequest) (*users.User, error) {
	if h.auth == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("auth not wired"))
	}
	u, err := h.auth.LookupSessionUser(ctx, req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("session lookup failed"))
	}
	if u == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("sign in to use the JD upload"))
	}
	if u.Status != users.StatusActive {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("account is not active"))
	}
	return u, nil
}

// ServeResumePDF streams a submission's locked PDF. Plain HTTP (a
// browser download link), gated by a signed-in session that owns the
// submission (or an admin) AND the result token in `t`.
// Route: GET /api/jd/resume/{file} where file is "<id>.pdf".
func (h *Jd) ServeResumePDF(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	idText, ok := strings.CutSuffix(file, ".pdf")
	if !ok {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	member, err := h.auth.LookupSessionUserHTTP(r.Context(), r)
	if err != nil || member == nil || member.Status != users.StatusActive {
		http.Error(w, "sign in to download", http.StatusUnauthorized)
		return
	}
	presented, err := hex.DecodeString(strings.TrimSpace(r.URL.Query().Get("t")))
	if err != nil || len(presented) == 0 {
		http.Error(w, "missing result token", http.StatusForbidden)
		return
	}
	pdf, token, ownerID, err := h.users.GetJdResumePDF(r.Context(), id)
	if err != nil || pdf == nil {
		http.NotFound(w, r)
		return
	}
	if ownerID != member.ID && member.Role != users.RoleAdmin {
		http.NotFound(w, r)
		return
	}
	if len(token) == 0 || subtle.ConstantTimeCompare(token, presented) != 1 {
		http.Error(w, "invalid result token", http.StatusForbidden)
		return
	}
	h.events.Emit(r.Context(), httpEvent(r, "jd.pdf_downloaded", member.ID, map[string]any{"submission_id": id}))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"roger-henley-resume-%d.pdf\"", id))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(pdf)
}

// fixedSubmitAckMessage is the copy rendered back after every
// successful submission. Kept server-side so an update lands on
// every caller without a frontend rebuild.
func fixedSubmitAckMessage() string {
	return "Got it, the JD is stored. Scoring runs now: the posting is broken into requirements, each is checked against Roger's records, and the match score is computed from those checks. It usually takes 15 to 30 minutes. You can close this page: the review stays under your submissions on this page, and you get an email when it finishes."
}

func jdSourceProtoToRepo(s v1.JdSource) string {
	switch s {
	case v1.JdSource_JD_SOURCE_PASTE:
		return "paste"
	case v1.JdSource_JD_SOURCE_PDF:
		return "pdf"
	case v1.JdSource_JD_SOURCE_TEXT_UPLOAD:
		return "text_upload"
	default:
		return ""
	}
}

func jdStatusRepoToProto(status string) v1.JdStatus {
	switch status {
	case "received":
		return v1.JdStatus_JD_STATUS_RECEIVED
	case "scoring":
		return v1.JdStatus_JD_STATUS_SCORING
	case "below_threshold":
		return v1.JdStatus_JD_STATUS_BELOW_THRESHOLD
	case "generating":
		return v1.JdStatus_JD_STATUS_GENERATING
	case "ready":
		return v1.JdStatus_JD_STATUS_READY
	case "failed":
		return v1.JdStatus_JD_STATUS_FAILED
	case "not_a_posting":
		return v1.JdStatus_JD_STATUS_NOT_A_POSTING
	default:
		return v1.JdStatus_JD_STATUS_UNSPECIFIED
	}
}

// shortHash returns the first 16 bytes of SHA-256(s) for storing a
// user-agent audit fingerprint without keeping the raw string.
func shortHash(s string) []byte {
	if s == "" {
		return nil
	}
	h := sha256.Sum256([]byte(s))
	return h[:16]
}
