// Package chat is Ask Roger's answer pipeline: one question in, one
// grounded answer with sources out, and a record of how it was reached.
//
// The shape of this package is set by one measurement. On the
// production box, prompt evaluation runs at roughly 32 tokens a second
// and generation at 6.5, with no GPU. Every passage shown to the model
// is about five seconds before the reader sees a word. So the pipeline
// is arranged to answer without the model wherever it honestly can, and
// to show the model as little as it can get away with when it cannot.
//
// The order of the stages is that principle, not convenience:
//
//	embed once  ->  Q&A bank  ->  restricted topics  ->  retrieval
//	            ->  nothing found  ->  the model
//
// Four of those six stages can produce a complete answer with no model
// call at all.
//
// Streaming is not here yet. The sidecar gateway is single-shot
// (Generate returns a whole response), and streaming needs a new RPC
// through it. The seam is Generator below; when the streaming RPC
// lands, the model stage changes and nothing else does.
package chat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/reh3376/career-site/services/api/internal/corpusscope"
	"github.com/reh3376/career-site/services/api/internal/ingest"
	"github.com/reh3376/career-site/services/api/internal/llm"
	"github.com/reh3376/career-site/services/api/internal/prompts"
	"github.com/reh3376/career-site/services/api/internal/runid"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Embedder turns the question into a vector. One call per question,
// reused by the bank lookup and by retrieval: they are the same
// question and embedding it twice would be two seconds wasted on a box
// that has none to spare.
type Embedder interface {
	Embed(ctx context.Context, texts []string, purpose ingest.EmbedPurpose) ([][]float32, string, error)
}

// Generator is the model. Named rather than taking llm.Client directly
// so the streaming implementation can replace it without touching the
// pipeline.
type Generator interface {
	Generate(ctx context.Context, req llm.Request) (*llm.Response, error)
}

// Store is the persistence this pipeline needs.
type Store interface {
	MatchQA(ctx context.Context, embedding []float32, threshold float64) (users.QAMatch, bool, error)
	SearchCorpusForChat(ctx context.Context, embedding []float32, topK int) ([]users.CorpusHit, error)
	AppendMessage(ctx context.Context, m users.ChatMessage) (int64, error)
	InsertDecisions(ctx context.Context, rows []users.Decision) error
	RecordLLMUsage(ctx context.Context, u users.LLMUsage) error
}

// Service answers questions.
type Service struct {
	Embed Embedder
	Model Generator
	Store Store
	Log   *slog.Logger

	// Show is how many passages reach the model. Three by default, and
	// this is the single most expensive number in the package: the
	// measured cost is about five seconds to first token per passage.
	Show int
	// Consider is how many retrieval returns before the cut. Larger
	// than Show costs nothing (a vector search over a few thousand
	// chunks is milliseconds) and buys the decision log a record of
	// what was found and not shown, which is how a retrieval problem is
	// told from a generation one.
	Consider int
	// QAThreshold overrides users.QAMatchThreshold. Zero uses it.
	QAThreshold float64
	// NumCtx is the context window the sidecar is configured with. The
	// API does not set it per call, but a decision row that cannot say
	// how much context the model had cannot be compared with one from a
	// different configuration.
	NumCtx int

	// Now is injectable for tests.
	Now func() time.Time
}

// Defaults, applied on first use so a zero Service is usable.
const (
	defaultShow     = 3
	defaultConsider = 12
)

// Request is one question in a conversation.
type Request struct {
	ConversationID int64
	UserID         int64
	Question       string
	// History is the conversation so far, oldest first. Rendering caps
	// it; the caller does not have to.
	History []prompts.AskTurn
	// AllowPrivate widens retrieval to the private corpus. Private
	// material may inform an answer and may never be cited, which is
	// enforced at the citation boundary rather than here.
	AllowPrivate bool
}

// Answer is what the member sees, plus what the surface needs to
// explain it.
type Answer struct {
	MessageID int64
	Text      string
	Citations []users.ChatCitation

	// Path is one of the users.ChatPath* constants: which of the six
	// ways this answer was produced.
	Path string

	OutOfScope bool
	NoSupport  bool
	Degraded   bool
	QAMatch    bool

	// FirstTokenMs and TotalMs are what the reader actually waited.
	// Until streaming lands FirstTokenMs is the whole model call, and
	// is recorded that way rather than left at zero, because a zero
	// would silently read as "instant" in the metrics.
	FirstTokenMs int64
	TotalMs      int64
}

// ErrNoQuestion is an empty question, which is a caller bug.
var ErrNoQuestion = errors.New("chat: empty question")

// Answer produces one reply and records how it was reached.
//
// The decision-log write is best-effort and deliberately last: the
// member's answer has already been persisted by then, and losing the
// training record is bad while failing the question over it would be
// worse.
func (s *Service) Answer(ctx context.Context, req Request) (Answer, error) {
	started := s.now()
	question := strings.TrimSpace(req.Question)
	if question == "" {
		return Answer{}, ErrNoQuestion
	}

	// One id for this answer, carried on the context so the decision
	// row and the usage ledger row are written with the same value and
	// can be joined. The JD pipeline uses it per run; here the unit is
	// one answer, which is the thing anyone would ask a question about.
	ctx = runid.With(ctx, runid.New())

	in := users.ChatDecisionInput{
		Question:           question,
		PersonaFingerprint: prompts.AskRogerPersona.Fingerprint(),
		HistoryTurns:       min(len(req.History), prompts.AskHistoryTurns),
		CorpusScope:        "public",
	}
	if req.AllowPrivate {
		in.CorpusScope = "private"
		ctx = corpusscope.With(ctx, corpusscope.All)
	}

	// 1. Embed once.
	embedStart := s.now()
	vecs, _, err := s.Embed.Embed(ctx, []string{question}, ingest.PurposeQuery)
	in.Timings.EmbedMs = s.since(embedStart)
	if err != nil || len(vecs) == 0 || len(vecs[0]) == 0 {
		// Without an embedding there is no bank lookup and no
		// retrieval, so there is nothing to ground an answer in and the
		// honest reply is the no-support one rather than an ungrounded
		// guess.
		return s.finish(ctx, req, in, started, users.ChatPathError,
			noSupportText, nil, "", "", "", nil, 0, 0, 0, errString(err, "embedding unavailable"))
	}
	embedding := vecs[0]

	// 2. The Q&A bank: the owner's own words, served verbatim, no model
	// call. On this hardware this is the path a common question should
	// take.
	qaStart := s.now()
	match, hit, qaErr := s.Store.MatchQA(ctx, embedding, s.qaThreshold())
	in.Timings.QAMatchMs = s.since(qaStart)
	if qaErr != nil {
		// A bank failure is not fatal: the model path still works, and
		// a question answered slowly beats a question not answered.
		s.log().Warn("qa bank lookup failed", slog.String("error", qaErr.Error()))
	}
	in.QA = users.ChatQALookup{
		Matched:        hit,
		Threshold:      s.qaThreshold(),
		BestSimilarity: match.Similarity,
	}
	if hit {
		in.QA.EntryID = match.Entry.ID
		in.QA.Phrasing = match.Phrasing
		out := users.ChatDecisionOutput{Text: match.Entry.Answer, QAMatch: true}
		out.Citations = qaCitations(match.Entry.Sources)
		return s.finish(ctx, req, in, started, users.ChatPathQABank,
			match.Entry.Answer, out.Citations, "", "", "", &out, 0, 0, 0, "")
	}

	// 3. Restricted topics (FR-CHAT-06). Compensation, references,
	// employer-confidential matters and personal life are refused
	// unless the owner has written an approved bank entry, and step 2
	// is where such an entry would have answered. Reaching here means
	// there is none.
	//
	// Checked in code as well as in persona rule 7 because it is the
	// one category where being talked into an answer does real damage,
	// and because refusing here costs nothing while asking the model to
	// refuse costs twenty seconds.
	if topic := restrictedTopic(question); topic != "" {
		text := restrictedReply(topic)
		out := users.ChatDecisionOutput{Text: text, OutOfScope: true}
		return s.finish(ctx, req, in, started, users.ChatPathOutOfScope,
			text, nil, "", "", "", &out, 0, 0, 0, "")
	}

	// 4. Retrieval. Gated on chatbot_include and on visibility in the
	// query, never in the prompt (FR-CHAT-18).
	retStart := s.now()
	hits, err := s.Store.SearchCorpusForChat(ctx, embedding, s.consider())
	in.Timings.RetrieveMs = s.since(retStart)
	if err != nil {
		return s.finish(ctx, req, in, started, users.ChatPathError,
			noSupportText, nil, "", "", "", nil, 0, 0, 0, fmt.Sprintf("retrieval failed: %v", err))
	}

	shown := hits
	if len(shown) > s.show() {
		shown = shown[:s.show()]
	}
	in.Retrieved = describeRetrieval(hits, shown)

	// 5. Nothing retrieved. The grounding rule (FR-CHAT-03) says an
	// answer with no support takes the "I don't know" path, and that
	// decision does not need a model to make it.
	if len(shown) == 0 {
		out := users.ChatDecisionOutput{Text: noSupportText, NoSupport: true}
		return s.finish(ctx, req, in, started, users.ChatPathNoSupport,
			noSupportText, nil, "", "", "", &out, 0, 0, 0, "")
	}

	// 6. The model.
	userTurn, offered := prompts.RenderAskUser(question, req.History, shown)
	markCitationOffers(in.Retrieved, offered)

	callStart := s.now()
	resp, err := s.Model.Generate(ctx, llm.Request{
		System:      prompts.AskRogerPersona.System,
		User:        userTurn,
		MaxTokens:   prompts.AskRogerAnswerMaxTokens,
		Temperature: 0,
	})
	callMs := s.since(callStart)

	// The usage ledger, written for a failed call as well as a good
	// one. It is where the monthly budget cap reads from (FR-CHAT-12)
	// and where token cost across surfaces is totted up, and a ledger
	// that silently omits the calls that went wrong reports a system
	// cheaper and healthier than it is.
	usage := users.LLMUsage{
		Kind:          "chat",
		RefID:         req.ConversationID,
		PromptID:      prompts.AskRogerPersona.ID,
		PromptVersion: prompts.AskRogerPersona.Version,
		LatencyMs:     callMs,
		OK:            err == nil && resp != nil && strings.TrimSpace(resp.Text) != "",
	}
	if resp != nil {
		usage.Model = resp.Model
		usage.PromptTokens = resp.PromptTokens
		usage.CompletionTokens = resp.CompletionTokens
	}
	if err != nil {
		usage.Error = err.Error()
	}
	if uerr := s.Store.RecordLLMUsage(ctx, usage); uerr != nil {
		s.log().Warn("chat usage not recorded", slog.String("error", uerr.Error()))
	}

	if err != nil || resp == nil || strings.TrimSpace(resp.Text) == "" {
		// FR-CHAT-17: degrade rather than fail. The bank has already
		// been consulted and did not match, so there is nothing to fall
		// back to except saying so and offering a way through.
		text := degradedText
		out := users.ChatDecisionOutput{Text: text, Degraded: true}
		return s.finish(ctx, req, in, started, users.ChatPathDegraded,
			text, nil, "", prompts.AskRogerPersona.System+"\n\n"+userTurn, "", &out,
			0, 0, callMs, errString(err, "the model returned nothing"))
	}

	text, cites, written, dropped := validateCitations(resp.Text, offered)
	out := users.ChatDecisionOutput{
		Text:           text,
		Citations:      cites,
		MarkersOffered: len(offered),
		MarkersWritten: written,
		MarkersDropped: dropped,
		FinishReason:   resp.FinishReason,
		Truncated:      resp.FinishReason == "length",
	}
	// The provider's own split, rather than the whole call recorded
	// twice. prompt_eval is what a reader waits through before the
	// first word, so it is the honest first-token figure until the
	// gateway streams; generation is what follows. A provider that
	// reports neither falls back to the whole call, which at least does
	// not claim to be something it is not.
	in.Timings.PromptEvalMs = resp.PromptEvalMs
	in.Timings.GenerateMs = resp.EvalMs
	in.Timings.FirstTokenMs = resp.PromptEvalMs
	if in.Timings.FirstTokenMs == 0 {
		in.Timings.FirstTokenMs = callMs
	}
	return s.finish(ctx, req, in, started, users.ChatPathModel,
		text, cites, resp.Model, prompts.AskRogerPersona.System+"\n\n"+userTurn, resp.Text,
		&out, resp.PromptTokens, resp.CompletionTokens, in.Timings.FirstTokenMs, "")
}

// finish persists the answer, logs the decision, and returns it.
//
// One place, so that every path (bank, refusal, no support, degraded,
// model, error) is persisted and logged the same way. The paths that
// never called a model are logged too: "did the bank fire when it
// should not have" is one of the most valuable labels the owner can
// give, and it cannot be asked about a row that was never written.
func (s *Service) finish(
	ctx context.Context, req Request, in users.ChatDecisionInput, started time.Time,
	path, text string, cites []users.ChatCitation,
	model, promptText, responseText string,
	out *users.ChatDecisionOutput,
	promptTokens, completionTokens int32, firstTokenMs int64, callErr string,
) (Answer, error) {
	in.Path = path
	if firstTokenMs > 0 {
		in.Timings.FirstTokenMs = firstTokenMs
	}
	in.Timings.TotalMs = s.since(started)

	ans := Answer{
		Text:         text,
		Citations:    cites,
		Path:         path,
		OutOfScope:   path == users.ChatPathOutOfScope,
		NoSupport:    path == users.ChatPathNoSupport || path == users.ChatPathError,
		Degraded:     path == users.ChatPathDegraded,
		QAMatch:      path == users.ChatPathQABank,
		FirstTokenMs: in.Timings.FirstTokenMs,
		TotalMs:      in.Timings.TotalMs,
	}

	id, err := s.Store.AppendMessage(ctx, users.ChatMessage{
		ConversationID: req.ConversationID,
		Role:           "assistant",
		Text:           text,
		OutOfScope:     ans.OutOfScope,
		NoSupport:      ans.NoSupport,
		Degraded:       ans.Degraded,
		QAMatch:        ans.QAMatch,
		PersonaVersion: in.PersonaFingerprint,
		Citations:      cites,
	})
	if err != nil {
		// The member gets nothing if this fails, so it is the one error
		// here that is fatal.
		return Answer{}, fmt.Errorf("persist answer: %w", err)
	}
	ans.MessageID = id

	if out == nil {
		out = &users.ChatDecisionOutput{Text: text, Citations: cites}
	}
	out.OutOfScope, out.NoSupport = ans.OutOfScope, ans.NoSupport
	out.Degraded, out.QAMatch = ans.Degraded, ans.QAMatch

	// Best-effort, and last. The answer is already persisted and about
	// to be shown; losing the training record is bad, and failing the
	// member's question to protect it would be worse.
	d, derr := users.NewChatDecision(id, model, promptText, responseText,
		in, *out, promptTokens, completionTokens,
		prompts.AskRogerPersona.Version, s.NumCtx, callErr)
	if derr != nil {
		s.log().Warn("chat decision not encoded", slog.String("error", derr.Error()))
		return ans, nil
	}
	if derr := s.Store.InsertDecisions(ctx, []users.Decision{d}); derr != nil {
		s.log().Warn("chat decision not logged",
			slog.Int64("message_id", id), slog.String("error", derr.Error()))
	}
	return ans, nil
}

// citationMarker matches the [1] form the persona is told to use.
var citationMarker = regexp.MustCompile(`\[(\d{1,2})\]`)

// validateCitations maps the markers the model wrote back to the
// passages it was offered, and removes the ones that point at nothing.
//
// A dangling [7] is not cosmetic. FR-CHAT-04 requires citations be
// validated against the retrieved set before display, and a marker with
// no source behind it is an answer claiming support it does not have,
// rendered in the one part of the reply a sceptical reader will check.
//
// Returns the cleaned text, the citations in order of first use, and
// the counts. The counts are what makes citation validity a query
// rather than something a human marks several hundred times.
func validateCitations(text string, offered []users.CorpusHit) (string, []users.ChatCitation, int, int) {
	var cites []users.ChatCitation
	seen := map[int]int{} // marker -> rank in cites, 1-based
	written := map[int]bool{}
	dropped := 0

	cleaned := citationMarker.ReplaceAllStringFunc(text, func(m string) string {
		n, err := strconv.Atoi(strings.Trim(m, "[]"))
		if err != nil {
			return m
		}
		written[n] = true
		if n < 1 || n > len(offered) {
			dropped++
			return ""
		}
		if _, ok := seen[n]; !ok {
			h := offered[n-1]
			seen[n] = len(cites) + 1
			cites = append(cites, users.ChatCitation{
				ChunkID: h.Chunk.ID,
				Title:   h.Title,
				Path:    h.SourcePath,
				Rank:    len(cites) + 1,
			})
		}
		return m
	})

	// Removing a marker can leave a double space or a space before a
	// full stop. A reader should not be able to tell that anything was
	// taken out.
	cleaned = strings.TrimSpace(collapseSpaces(cleaned))
	return cleaned, cites, len(written), dropped
}

var multiSpace = regexp.MustCompile(`[ \t]{2,}`)
var spaceBeforePunct = regexp.MustCompile(`[ \t]+([.,;:!?])`)

func collapseSpaces(s string) string {
	s = multiSpace.ReplaceAllString(s, " ")
	return spaceBeforePunct.ReplaceAllString(s, "$1")
}

// describeRetrieval records what retrieval found and what survived the
// cut, which is the difference between diagnosing a retrieval problem
// and guessing at a model one.
func describeRetrieval(all, shown []users.CorpusHit) []users.ChatRetrievedChunk {
	isShown := make(map[int64]bool, len(shown))
	for _, h := range shown {
		isShown[h.Chunk.ID] = true
	}
	out := make([]users.ChatRetrievedChunk, 0, len(all))
	for _, h := range all {
		c := users.ChatRetrievedChunk{
			ChunkID:    h.Chunk.ID,
			Similarity: float64(h.Similarity),
			Citable:    users.Citable(h),
			Shown:      isShown[h.Chunk.ID],
		}
		// A private document's title is itself unpublished, so it is
		// kept out of the record as well as out of the prompt.
		if c.Citable {
			c.Title = h.Title
		}
		out = append(out, c)
	}
	return out
}

// markCitationOffers records which marker each passage was offered as,
// so a logged answer can be read back against the numbering the model
// actually saw.
func markCitationOffers(recorded []users.ChatRetrievedChunk, offered []users.CorpusHit) {
	marker := make(map[int64]int, len(offered))
	for i, h := range offered {
		marker[h.Chunk.ID] = i + 1
	}
	for i := range recorded {
		recorded[i].Marker = marker[recorded[i].ChunkID]
	}
}

func qaCitations(sources []users.QASource) []users.ChatCitation {
	out := make([]users.ChatCitation, 0, len(sources))
	for i, s := range sources {
		out = append(out, users.ChatCitation{Title: s.Title, Path: s.Path, Rank: i + 1})
	}
	return out
}

// restrictedPatterns are the FR-CHAT-06 topics, checked in code.
//
// Deliberately narrow. This runs before the model and refuses outright,
// so a false positive silently refuses a fair question and is the
// expensive mistake; persona rule 7 is the general case and this is the
// backstop for the wordings that must never get through. Each pattern
// is anchored on a phrase that is about the topic rather than merely
// containing its vocabulary, which is why "salary" alone is not here
// and "salary expectations" is.
var restrictedPatterns = []struct {
	topic string
	re    *regexp.Regexp
}{
	{"compensation", regexp.MustCompile(`(?i)\b(salary (expectation|requirement|range|history)|expected salary|desired salary|compensation (expectation|requirement|package|range)|how much (do|did|would) (you|he) (want|make|earn|expect)|what (do|did) (you|he) (make|earn)|pay (expectation|requirement)|rate (expectation|requirement))`)},
	{"references", regexp.MustCompile(`(?i)\b(provide|give|share|list|who are) (me |us )?(your |his )?(professional )?references\b|\breference (check|contact)s?\b`)},
	{"employer_confidential", regexp.MustCompile(`(?i)\b(confidential|proprietary|trade secret|under nda|non-disclosure)\b.*\b(employer|company|client|work|project)\b|\b(tell|show) me (something|anything) (confidential|proprietary)`)},
	{"personal", regexp.MustCompile(`(?i)\b(marital status|are you married|your (wife|husband|spouse|children|kids|family)|religio(n|us)|political (view|affiliation|party)|who did you vote|sexual orientation|health condition|disabilit(y|ies)|how old are you|your age\b)`)},
}

func restrictedTopic(q string) string {
	for _, p := range restrictedPatterns {
		if p.re.MatchString(q) {
			return p.topic
		}
	}
	return ""
}

// The refusals. One sentence, in Roger's voice, pointing somewhere
// useful, and not explaining the rule that produced them: a reader who
// asked a fair question does not need a policy lecture, and one probing
// the edges should not be handed a map.
func restrictedReply(topic string) string {
	switch topic {
	case "compensation":
		return "That is better discussed directly than through an assistant. You can [book time with me](/meetings) or [get in touch](/contact) and we can talk about it properly."
	case "references":
		return "References are something I share directly once a conversation is underway. [Get in touch](/contact) and I will sort it out."
	case "employer_confidential":
		return "I do not discuss anything confidential to an employer or a client, past or present. I am happy to talk about the work itself: what I built, how it was approached, and what came of it."
	default:
		return "I keep this to my professional background and my work. If there is something about either I can help with, ask away, or [get in touch](/contact)."
	}
}

const noSupportText = "I do not have anything in my records about that. If it is something you need to know, [get in touch](/contact) and ask me directly."

const degradedText = "I cannot reach my own records just now, so I would rather not guess. Try again in a few minutes, or [get in touch](/contact) and ask me directly."

func (s *Service) show() int {
	if s.Show > 0 {
		return s.Show
	}
	return defaultShow
}

func (s *Service) consider() int {
	if s.Consider > 0 {
		return s.Consider
	}
	return defaultConsider
}

func (s *Service) qaThreshold() float64 {
	if s.QAThreshold > 0 {
		return s.QAThreshold
	}
	return users.QAMatchThreshold
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) since(t time.Time) int64 { return s.now().Sub(t).Milliseconds() }

func (s *Service) log() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}

func errString(err error, fallback string) string {
	if err != nil {
		return err.Error()
	}
	return fallback
}
