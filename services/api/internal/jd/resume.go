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

// resumeTokenBudget is the résumé's output cap. Production's longest
// answer in 72 calls is 1,872 tokens and its average 959; the
// development machine, with a wider context and so more evidence in
// the prompt, averaged 1,511 and reached 2,114 before it first filled
// the cap on 2026-11-10. A cap is a guess about the longest answer the
// model will ever want to give, so it is paired with one retry
// (generate) rather than raised and trusted.
const resumeTokenBudget = 2200

// resumeDoc is the model's résumé as decoded, before any line is
// checked against its sources.
type resumeDoc struct {
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

// generate makes the résumé call and decodes it, with one retry when
// the answer filled its budget.
//
// The judge has had this retry since evaluation run 13 lost a posting
// to a judgment that filled its cap (assess.go, call). The résumé
// writer did not, and on 2026-11-10 two reviews on the development
// machine ended "failed" after their requirements had been judged and
// scored, because the résumé stopped mid-string at exactly 2,200
// tokens. A review that is finished must not be lost to the length of
// the document written about it.
//
// The retry doubles the budget. The wider answer has to fit the same
// context window, so the evidence is trimmed from the tail first: it is
// in priority order (master résumé, then cited chunks, then the
// posting's retrieval), so what goes is what mattered least. Returns
// the evidence the answer was actually written from, because every
// line is checked against the chunks it cites and must not be checked
// against one the model never saw.
func (w *ResumeWriter) generate(
	ctx context.Context, submissionID int64, p prompts.Prompt,
	render func([]users.CorpusHit) string, evidence []users.CorpusHit, budget int,
) (resumeDoc, *llm.Response, []users.CorpusHit, error) {
	var doc resumeDoc
	call := func(maxTokens int, ev []users.CorpusHit) (*llm.Response, error) {
		started := time.Now()
		resp, err := w.llm.Generate(ctx, llm.Request{
			System:      p.System,
			User:        render(ev),
			MaxTokens:   maxTokens,
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
		// The repository is absent in tests that drive this with a
		// scripted model; a missing ledger row is never a reason a call
		// fails.
		if w.users != nil {
			if rErr := w.users.RecordLLMUsage(context.WithoutCancel(ctx), usage); rErr != nil {
				w.log.Warn("llm usage record failed", slog.Int64("jd_id", submissionID), slog.String("error", rErr.Error()))
			}
		}
		return resp, err
	}

	resp, err := call(resumeTokenBudget, evidence)
	if err != nil {
		return doc, nil, evidence, err
	}
	decodeErr := json.Unmarshal([]byte(resp.Text), &doc)
	if decodeErr == nil {
		return doc, resp, evidence, nil
	}
	if resp.CompletionTokens < resumeTokenBudget {
		// Malformed under budget: a model that cannot follow the schema,
		// which a wider budget would not fix.
		return doc, resp, evidence, fmt.Errorf("decode resume: %w", decodeErr)
	}

	wider := 2 * resumeTokenBudget
	room := budget - (wider - resumeTokenBudget)
	trimmed := evidence
	for len(trimmed) > 1 && prompts.EstimateTokens(render(trimmed)) > room {
		trimmed = trimmed[:len(trimmed)-1]
	}
	w.log.Warn("resume output hit its token budget, retrying once at double",
		slog.Int64("jd_id", submissionID), slog.Int("budget", resumeTokenBudget),
		slog.Int("evidence_kept", len(trimmed)), slog.Int("evidence_dropped", len(evidence)-len(trimmed)))
	resp2, err2 := call(wider, trimmed)
	if err2 != nil {
		return doc, resp, evidence, fmt.Errorf(
			"decode resume: the model filled its %d-token budget and the retry at %d failed: %w",
			resumeTokenBudget, wider, err2)
	}
	doc = resumeDoc{}
	if err3 := json.Unmarshal([]byte(resp2.Text), &doc); err3 != nil {
		return doc, resp2, trimmed, fmt.Errorf(
			"decode resume: the model filled its %d-token budget, and the retry at %d did not decode either (%d tokens): %w",
			resumeTokenBudget, wider, resp2.CompletionTokens, err3)
	}
	w.log.Info("resume retry at double budget decoded",
		slog.Int64("jd_id", submissionID), slog.Int("completion_tokens", int(resp2.CompletionTokens)))
	return doc, resp2, trimmed, nil
}

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
	// Like the judge's facts sheet, this ignores internal/corpusscope on
	// purpose (owner, 2026-09-24): the master résumé is corpus_only, it
	// is about the candidate rather than a client, it is the same for
	// every posting, and it is fetched by kind rather than retrieved, so
	// a submitted posting cannot steer it. Without it the writer has no
	// roles or dates to work from.
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

	render := func(ev []users.CorpusHit) string {
		return prompts.RenderResumeUser(jdText, hints, verdicts, ev)
	}
	raw, resp, evidence, err := w.generate(ctx, submissionID, p, render, evidence, budget)
	if err != nil {
		return nil, "", err
	}
	// The ids a line may cite are the ones the answer was written from,
	// which after a retry is the trimmed list.
	seen = map[int64]bool{}
	for _, h := range evidence {
		seen[h.Chunk.ID] = true
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
		// Two different faults produce an empty résumé and they need
		// opposite responses, so the message has to tell them apart. A
		// model that cited nothing is a prompt or retrieval problem. The
		// entailment check emptying it means the heuristic is too
		// aggressive on this posting, and the claims it rejected are the
		// evidence for deciding which.
		if out.Unsupported > 0 {
			return nil, "", fmt.Errorf(
				"resume: the support check removed %d line(s) and left no usable experience; rejected: %s",
				out.Unsupported, strings.Join(out.UnsupportedClaims, " | "))
		}
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
