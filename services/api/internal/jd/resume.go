package jd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/reh3376/career-site/services/api/internal/llm"
	"github.com/reh3376/career-site/services/api/internal/prompts"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// ResumeTopK is how many JD-retrieved chunks join the résumé evidence
// on top of the assessment's cited chunks and the master résumé.
const ResumeTopK = 12

// Sourced is one résumé line with the chunk ids it was drawn from.
type Sourced struct {
	Text    string  `json:"text"`
	Sources []int64 `json:"sources"`
}

// Experience is one role.
type Experience struct {
	Role         string    `json:"role"`
	Organisation string    `json:"organisation"`
	Dates        string    `json:"dates"`
	Bullets      []Sourced `json:"bullets"`
}

// Resume is the verified structured résumé. Every Sourced item cites at
// least one chunk that was offered to the model, and carries no
// quantity or named organisation that those chunks do not contain.
type Resume struct {
	Headline     string       `json:"headline"`
	Summary      string       `json:"summary"`
	Competencies []Sourced    `json:"competencies"`
	Experience   []Experience `json:"experience"`
	Education    []Sourced    `json:"education"`
	// Verification stats, kept for the admin view.
	Dropped int `json:"dropped"`
	// Unsupported counts lines dropped because they asserted a number or
	// an organisation their own sources do not contain. Separate from
	// Dropped, which counts lines with no usable citation at all: one is
	// a model citing nothing, the other is a model citing something that
	// does not say what it claims, and they argue for different fixes.
	Unsupported int `json:"unsupported"`
	// UnsupportedClaims is what those lines asserted, for the admin view.
	// A résumé that drops three lines is worth looking at; a count alone
	// does not say what was wrong with them.
	UnsupportedClaims []string `json:"unsupported_claims,omitempty"`
	Model             string   `json:"model"`
	PromptID          string   `json:"prompt_id"`
	PromptVers        int      `json:"prompt_version"`
}

// Renderer produces the locked PDF from verified résumé JSON. The
// sidecar implements it with Typst + pypdf.
type Renderer interface {
	RenderResume(ctx context.Context, resumeJSON, ownerPassword, traceID string) (pdf []byte, pages int32, err error)
}

// ResumeWriter turns an above-threshold assessment into a grounded
// résumé via the LLM gateway, verifies the sources in code, and
// renders markdown from the verified structure.
type ResumeWriter struct {
	log        *slog.Logger
	users      *users.Repo
	llm        llm.Client
	monthlyCap int64
	// PDF rendering is optional: nil renderer or empty password leaves
	// the markdown as the deliverable and logs why.
	renderer      Renderer
	ownerPassword string
	// numCtx bounds the résumé prompt: evidence is added in priority
	// order until the budget is spent (see Write).
	numCtx int
}

// resumeReserve is the context kept free for the system prompt and the
// JSON résumé output when trimming evidence.
const resumeReserve = 2600

// jdHeadRunes is how much of the posting the writer keeps when the
// posting and the full master résumé cannot both fit the window.
const jdHeadRunes = 1500

// NewResumeWriter wires the deps. renderer may be nil; numCtx <= 0 falls
// back to 16384.
func NewResumeWriter(log *slog.Logger, repo *users.Repo, client llm.Client, monthlyCap int64, renderer Renderer, ownerPassword string, numCtx int) *ResumeWriter {
	if numCtx <= 0 {
		numCtx = 16384
	}
	return &ResumeWriter{log: log, users: repo, llm: client, monthlyCap: monthlyCap, renderer: renderer, ownerPassword: ownerPassword, numCtx: numCtx}
}

// RenderPDF renders the verified résumé to a locked PDF. Returns nil,
// 0, nil when rendering is not configured so callers can treat the
// PDF as optional.
func (w *ResumeWriter) RenderPDF(ctx context.Context, submissionID int64, resumeJSON []byte) ([]byte, int32, error) {
	if w.renderer == nil {
		return nil, 0, nil
	}
	if w.ownerPassword == "" {
		w.log.Warn("resume pdf skipped: RESUME_PDF_OWNER_PASSWORD is not set", slog.Int64("jd_id", submissionID))
		return nil, 0, nil
	}
	return w.renderer.RenderResume(ctx, string(resumeJSON), w.ownerPassword, fmt.Sprintf("jd:%d", submissionID))
}

// Write assembles evidence, calls the model once, verifies, and
// returns the résumé plus its markdown rendering.
func (w *ResumeWriter) Write(
	ctx context.Context, submissionID int64, jdText string, hints prompts.Hints,
	assessment *Assessment, jdHits []users.CorpusHit,
) (*Resume, string, error) {
	if err := w.checkCap(ctx, 1); err != nil {
		return nil, "", err
	}

	// Evidence in priority order: master résumé first (roles and
	// dates), then the chunks the judge cited, then the JD retrieval.
	// De-duplicated by id and cut off once the rendered prompt would
	// exceed the context budget, so the call fits the box.
	var verdicts []prompts.Verdict
	var cited []users.CorpusHit
	if assessment != nil {
		var ids []int64
		for _, j := range assessment.Judgments {
			ids = append(ids, j.EvidenceIDs...)
		}
		if hits, err := w.users.GetCorpusChunks(ctx, ids); err == nil {
			cited = hits
		}
		reqText := map[string]string{}
		for _, r := range assessment.Requirements {
			reqText[r.ID] = r.Text
		}
		for _, j := range assessment.Judgments {
			verdicts = append(verdicts, prompts.Verdict{RequirementID: j.RequirementID, Text: reqText[j.RequirementID], Verdict: j.Verdict})
		}
	}
	resumeChunks, err := w.users.ListChunksByKind(ctx, "resume", 40)
	if err != nil {
		w.log.Warn("resume chunks unavailable", slog.String("error", err.Error()))
	}
	if len(jdHits) > ResumeTopK {
		jdHits = jdHits[:ResumeTopK]
	}

	p := prompts.ResumeTailor
	budget := w.numCtx - resumeReserve - prompts.EstimateTokens(p.System)

	// The master résumé outranks the posting body. The writer already
	// holds every requirement with its verdict, so when a long posting
	// and the full résumé cannot share the window, the posting is cut
	// to its head rather than dropping roles, dates or education.
	if prompts.EstimateTokens(prompts.RenderResumeUser(jdText, hints, verdicts, resumeChunks)) > budget {
		if r := []rune(jdText); len(r) > jdHeadRunes {
			jdText = string(r[:jdHeadRunes]) + "\n[posting truncated to fit the context window; the verdicts list every requirement]"
			w.log.Info("resume prompt: posting truncated to keep the full master résumé",
				slog.Int64("jd_id", submissionID), slog.Int("kept_runes", jdHeadRunes), slog.Int("num_ctx", w.numCtx))
		}
	}

	var evidence []users.CorpusHit
	seen := map[int64]bool{}
	dropped := 0
	for _, group := range [][]users.CorpusHit{resumeChunks, cited, jdHits} {
		for _, h := range group {
			if seen[h.Chunk.ID] {
				continue
			}
			trial := append(append([]users.CorpusHit{}, evidence...), h)
			if len(evidence) > 0 && prompts.EstimateTokens(prompts.RenderResumeUser(jdText, hints, verdicts, trial)) > budget {
				dropped++
				continue
			}
			seen[h.Chunk.ID] = true
			evidence = trial
		}
	}
	if len(evidence) == 0 {
		return nil, "", errors.New("resume: no evidence available")
	}
	if dropped > 0 {
		w.log.Info("resume evidence trimmed to context budget",
			slog.Int64("jd_id", submissionID), slog.Int("kept", len(evidence)), slog.Int("dropped", dropped), slog.Int("num_ctx", w.numCtx))
	}

	started := time.Now()
	resp, err := w.llm.Generate(ctx, llm.Request{
		System:      p.System,
		User:        prompts.RenderResumeUser(jdText, hints, verdicts, evidence),
		MaxTokens:   2200,
		Temperature: 0.2,
		JSONSchema:  p.Schema,
		TraceID:     fmt.Sprintf("jd:%d:%s", submissionID, p.ID),
	})
	usage := users.LLMUsage{
		Kind: "jd_resume", RefID: submissionID,
		PromptID: p.ID, PromptVersion: p.Version,
		LatencyMs: time.Since(started).Milliseconds(), Model: "unknown",
	}
	if err == nil {
		usage.OK = true
		usage.Model = resp.Model
		usage.PromptTokens = resp.PromptTokens
		usage.CompletionTokens = resp.CompletionTokens
		if resp.LatencyMs > 0 {
			usage.LatencyMs = resp.LatencyMs
		}
	} else {
		usage.Error = truncErr(err.Error())
	}
	if rErr := w.users.RecordLLMUsage(context.WithoutCancel(ctx), usage); rErr != nil {
		w.log.Warn("llm usage record failed", slog.Int64("jd_id", submissionID), slog.String("error", rErr.Error()))
	}
	if err != nil {
		return nil, "", err
	}

	var raw struct {
		Headline     string `json:"headline"`
		Summary      string `json:"summary"`
		Competencies []struct {
			Text    string   `json:"text"`
			Sources []string `json:"sources"`
		} `json:"competencies"`
		Experience []struct {
			Role         string `json:"role"`
			Organisation string `json:"organisation"`
			Dates        string `json:"dates"`
			Bullets      []struct {
				Text    string   `json:"text"`
				Sources []string `json:"sources"`
			} `json:"bullets"`
		} `json:"experience"`
		Education []struct {
			Text    string   `json:"text"`
			Sources []string `json:"sources"`
		} `json:"education"`
	}
	if err := json.Unmarshal([]byte(resp.Text), &raw); err != nil {
		return nil, "", fmt.Errorf("decode resume: %w", err)
	}

	out := &Resume{
		Headline:   tidy(raw.Headline),
		Summary:    tidy(raw.Summary),
		Model:      resp.Model,
		PromptID:   p.ID,
		PromptVers: p.Version,
	}
	// Text of every chunk offered, so a line can be checked against what
	// it actually cites rather than against the fact that it cited.
	chunkText := map[int64]string{}
	for _, h := range evidence {
		chunkText[h.Chunk.ID] = h.Chunk.Text
	}
	verify := func(text string, sources []string) (Sourced, bool) {
		var ids []int64
		for _, s := range sources {
			id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
			if err == nil && seen[id] {
				ids = append(ids, id)
			}
		}
		text = tidy(text)
		if text == "" || len(ids) == 0 {
			out.Dropped++
			return Sourced{}, false
		}
		// Does what it cites actually carry the specifics it asserts?
		// Only the checkable parts: a quantity or a named organisation
		// that appears in the line and in none of its sources is either
		// invented or mis-cited, and both are reasons not to print it on
		// a résumé that goes to an employer.
		var srcText strings.Builder
		for _, id := range ids {
			srcText.WriteString(chunkText[id])
			srcText.WriteByte('\n')
		}
		if u := checkSupport(text, srcText.String()); u.any() {
			out.Unsupported++
			out.UnsupportedClaims = append(out.UnsupportedClaims,
				describeUnsupported(text, u))
			w.log.Warn("resume: line dropped, its sources do not carry the claim",
				slog.Int64("jd_id", submissionID),
				slog.String("numbers", strings.Join(u.Numbers, ",")),
				slog.String("parties", strings.Join(u.Parties, ",")))
			return Sourced{}, false
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		return Sourced{Text: text, Sources: ids}, true
	}
	for _, c := range raw.Competencies {
		if s, ok := verify(c.Text, c.Sources); ok {
			out.Competencies = append(out.Competencies, s)
		}
	}
	for _, e := range raw.Experience {
		exp := Experience{Role: tidy(e.Role), Organisation: tidy(e.Organisation), Dates: tidy(e.Dates)}
		for _, b := range e.Bullets {
			if s, ok := verify(b.Text, b.Sources); ok {
				exp.Bullets = append(exp.Bullets, s)
			}
		}
		if exp.Role != "" && len(exp.Bullets) > 0 {
			out.Experience = append(out.Experience, exp)
		} else {
			out.Dropped++
		}
	}
	for _, e := range raw.Education {
		if s, ok := verify(e.Text, e.Sources); ok {
			out.Education = append(out.Education, s)
		}
	}
	if out.Headline == "" || len(out.Experience) == 0 {
		return nil, "", errors.New("resume: verification left no usable experience")
	}
	if out.Unsupported > 0 {
		w.log.Info("resume: unsupported lines removed",
			slog.Int64("jd_id", submissionID), slog.Int("count", out.Unsupported))
	}
	return out, RenderMarkdown(out), nil
}

// DownloadPath is the api route that streams a submission's PDF. The
// caller appends the result token as `?t=`; the path alone is what is
// stored, so a token never lands in the database twice.
func (w *ResumeWriter) DownloadPath(submissionID int64) string {
	return fmt.Sprintf("/api/jd/resume/%d.pdf", submissionID)
}

// RenderMarkdown is the deterministic markdown view of a verified
// résumé. Sources are not printed; they stay in the JSON for audit and
// for the PDF renderer.
func RenderMarkdown(r *Resume) string {
	var b strings.Builder
	b.WriteString("# Roger E. Henley II\n\n")
	if r.Headline != "" {
		b.WriteString(r.Headline + "\n\n")
	}
	if r.Summary != "" {
		b.WriteString("## Summary\n\n" + r.Summary + "\n\n")
	}
	if len(r.Competencies) > 0 {
		b.WriteString("## Core competencies\n\n")
		for _, c := range r.Competencies {
			b.WriteString("- " + c.Text + "\n")
		}
		b.WriteString("\n")
	}
	if len(r.Experience) > 0 {
		b.WriteString("## Selected experience\n\n")
		for _, e := range r.Experience {
			line := "### " + e.Role
			if e.Organisation != "" {
				line += ", " + e.Organisation
			}
			if e.Dates != "" {
				line += " (" + e.Dates + ")"
			}
			b.WriteString(line + "\n\n")
			for _, bl := range e.Bullets {
				b.WriteString("- " + bl.Text + "\n")
			}
			b.WriteString("\n")
		}
	}
	if len(r.Education) > 0 {
		b.WriteString("## Education and credentials\n\n")
		for _, e := range r.Education {
			b.WriteString("- " + e.Text + "\n")
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String()) + "\n"
}

// tidy trims whitespace and enforces the site's no-em-dash rule on
// generated text.
func tidy(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " — ", ", ")
	s = strings.ReplaceAll(s, "—", ", ")
	s = strings.ReplaceAll(s, "&mdash;", ", ")
	return s
}

func (w *ResumeWriter) checkCap(ctx context.Context, calls int64) error {
	if w.monthlyCap <= 0 {
		return nil
	}
	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	n, err := w.users.CountLLMCallsSince(ctx, monthStart)
	if err != nil {
		return err
	}
	if n+calls > w.monthlyCap {
		return ErrMonthlyCap
	}
	return nil
}
