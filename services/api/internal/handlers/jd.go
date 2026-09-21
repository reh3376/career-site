package handlers

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/gen/career/v1/careerv1connect"
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
			errors.New("that JD looks awfully short — need at least 100 characters to work with"))
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
			slog.String("peer", req.Peer().Addr),
			slog.Duration("retry_after", retry),
		)
		return nil, connect.NewError(connect.CodeResourceExhausted,
			errors.New("too many submissions from your address — try again shortly"))
	}

	ipHash := hashIPBytes(req.Peer().Addr)
	uaHash := shortHash(req.Header().Get("User-Agent"))

	s, err := h.users.CreateJdSubmission(ctx, users.JdSubmitInput{
		SourceKind:   source,
		JdText:       text,
		RoleHint:     strings.TrimSpace(msg.RoleHint),
		EmployerHint: strings.TrimSpace(msg.EmployerHint),
		ContactEmail: strings.ToLower(strings.TrimSpace(msg.ContactEmail)),
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

	// Fire-and-forget score in the background so the RPC returns
	// immediately. Uses a fresh context (not `ctx`, which cancels
	// as soon as the caller disconnects) bounded by the pipeline
	// timeout. Nil scorer (dev without sidecar) leaves the row in
	// RECEIVED for manual triage.
	if h.scorer != nil {
		hints := prompts.Hints{Role: s.RoleHint, Employer: s.EmployerHint}
		go func(id int64, jdText string) {
			bg, cancel := context.WithTimeout(context.Background(), h.pipelineTimeout)
			defer cancel()
			h.scorer.ScoreAndPersist(bg, id, jdText, hints)
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
	if s.ResumeMarkdown != "" {
		tok := strings.TrimSpace(req.Msg.ResultToken)
		if presented, err := hex.DecodeString(tok); err == nil && s.TokenMatches(presented) {
			out.ResumeMarkdown = s.ResumeMarkdown
			if s.GeneratedResumeURL != "" {
				out.GeneratedResumeUrl = s.GeneratedResumeURL + "?t=" + tok
			}
		}
	}
	return connect.NewResponse(out), nil
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
	return "Got it, the JD is stored. Scoring runs now: the posting is broken into requirements, each is checked against Roger's career corpus, and the match score is computed from those checks. Keep this page open, or come back with your reference number."
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
