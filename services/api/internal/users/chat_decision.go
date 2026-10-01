package users

import (
	"encoding/json"
	"fmt"
)

// The shape of a `chat_answer` row in decision_log.
//
// decision_log types `input` and `output` as jsonb with the shape
// deliberately left per kind, which is right for a table serving four
// different decisions and wrong as an excuse not to decide. These
// structs are that decision for the assistant, and they exist because
// of what the rows are for: the owner grades them in the console and
// the graded ones become training data. A field that is sometimes
// present and sometimes not, or named one thing this month and another
// the next, survives review and quietly ruins an export six months
// later when nothing can be joined on it.
//
// Three things are captured that the JD rows never needed.
//
//   - The path. An answer can come from the Q&A bank, from the model,
//     from the bank because the model was down, or from neither because
//     the question was out of scope or nothing was retrieved. Those are
//     four different systems working or failing, and they render almost
//     identically to a reader. Without this, "the assistant got worse"
//     is unanswerable.
//   - The retrieval set, with similarities, and which of it was shown.
//     Grounding is a property of an answer *given what it was shown*,
//     and a reviewer cannot grade it from the answer alone.
//   - Citation markers written against markers offered. This makes
//     citation validity (FR-CHAT-04, acceptance at 95 %) a computed
//     number rather than something a human has to mark, which matters
//     because a human will not mark several hundred answers and a
//     query will.

// Chat answer paths. One of these is always set on a logged answer.
const (
	// ChatPathQABank is an owner-written answer served verbatim: no
	// model call, and the fast path on this hardware.
	ChatPathQABank = "qa_bank"
	// ChatPathModel is the ordinary retrieval-augmented answer.
	ChatPathModel = "model"
	// ChatPathDegraded is the bank answering because the model was
	// unavailable or the budget cap was reached (FR-CHAT-17/12). The
	// answer may be identical to a qa_bank one; the situation is not,
	// and grading them together would hide an outage.
	ChatPathDegraded = "degraded"
	// ChatPathOutOfScope is a refusal under persona rule 6 or 7.
	ChatPathOutOfScope = "out_of_scope"
	// ChatPathNoSupport is "I don't have anything in my records about
	// that": retrieval returned nothing usable (FR-CHAT-03).
	ChatPathNoSupport = "no_support"
	// ChatPathError is a call that produced no answer at all. Logged
	// like the others, because the behaviour near a failure is the
	// behaviour least understood.
	ChatPathError = "error"
)

// ChatDecisionInput is what an answer was produced from.
//
// Carries no member identity. The row already joins to the message and
// through it to the member, so repeating it here would only put a
// person's id into a file whose whole purpose is to be exported and fed
// to a trainer.
type ChatDecisionInput struct {
	Question string `json:"question"`
	// Path taken, from the constants above.
	Path string `json:"path"`
	// PersonaFingerprint is prompts.AskRogerPersona.Fingerprint(), so a
	// change of persona is visible in the row rather than inferred from
	// its date.
	PersonaFingerprint string `json:"persona_fingerprint"`
	// HistoryTurns is how many prior exchanges were in the prompt. An
	// answer that reads oddly on its own often reads correctly as a
	// follow-up, and a grader needs to know which it was looking at.
	HistoryTurns int `json:"history_turns"`
	// CorpusScope is "public" or "private", the access level retrieval
	// ran at (FR-CHAT-18).
	CorpusScope string `json:"corpus_scope"`
	// Retrieved is everything retrieval returned, in rank order,
	// including what was not shown to the model. The chunks that were
	// dropped are how a retrieval problem is told from a generation
	// one: an answer that missed an obvious fact sitting at rank 4,
	// below the cut, is a tuning problem and not a model problem.
	Retrieved []ChatRetrievedChunk `json:"retrieved"`
	// QA records the bank lookup that every question goes through,
	// whether or not it matched. The near misses are what calibrate
	// QAMatchThreshold, and they only exist if the misses are logged.
	QA ChatQALookup `json:"qa"`
	// Timings in milliseconds, per stage.
	Timings ChatTimings `json:"timings"`
}

// ChatRetrievedChunk is one retrieval hit as it was considered.
type ChatRetrievedChunk struct {
	ChunkID    int64   `json:"chunk_id"`
	ContentID  string  `json:"content_id,omitempty"`
	Title      string  `json:"title,omitempty"`
	Similarity float64 `json:"similarity"`
	// Citable is false for private-corpus material, which may inform an
	// answer and must never be named (FR-CHAT-04).
	Citable bool `json:"citable"`
	// Shown is false for a hit retrieval found and the prompt budget
	// left out.
	Shown bool `json:"shown"`
	// Marker is the number this chunk was offered to the model as, or 0
	// if it was not offered one.
	Marker int `json:"marker,omitempty"`
}

// ChatQALookup is the bank lookup, matched or not.
type ChatQALookup struct {
	Matched   bool    `json:"matched"`
	EntryID   int64   `json:"entry_id,omitempty"`
	Phrasing  string  `json:"phrasing,omitempty"`
	Threshold float64 `json:"threshold"`
	// BestSimilarity is the nearest phrasing whether or not it cleared
	// the threshold. The whole point of recording a miss.
	BestSimilarity float64 `json:"best_similarity"`
}

// ChatTimings is where the reader's wait went.
type ChatTimings struct {
	EmbedMs    int64 `json:"embed_ms"`
	QAMatchMs  int64 `json:"qa_match_ms"`
	RetrieveMs int64 `json:"retrieve_ms"`
	// FirstTokenMs is the wait before the first word. Until the gateway
	// streams, this is the provider's own prompt-evaluation time, which
	// is the thing a reader actually waits through and is measured
	// rather than guessed.
	FirstTokenMs int64 `json:"first_token_ms"`
	// PromptEvalMs and GenerateMs are the provider's own split of the
	// model call. Together they say whether a slow answer was slow
	// because it read too much or because it wrote too much, which are
	// fixed in completely different ways.
	PromptEvalMs int64 `json:"prompt_eval_ms"`
	GenerateMs   int64 `json:"generate_ms"`
	TotalMs      int64 `json:"total_ms"`
}

// ChatDecisionOutput is the answer and what can be checked about it
// without a human.
type ChatDecisionOutput struct {
	Text string `json:"text"`
	// Citations that survived validation, in the order shown.
	Citations []ChatCitation `json:"citations"`
	// MarkersOffered is how many numbered passages the model was given.
	MarkersOffered int `json:"markers_offered"`
	// MarkersWritten is how many distinct markers it wrote.
	MarkersWritten int `json:"markers_written"`
	// MarkersDropped is how many of those pointed at nothing.
	//
	// Citation validity (FR-CHAT-04, acceptance at 95 %) is
	// MarkersDropped against MarkersWritten across rows, computed and
	// not marked by hand. An assistant that invents a [7] is doing
	// something specific and detectable, and detecting it should not
	// depend on the owner noticing.
	MarkersDropped int `json:"markers_dropped"`
	// ProposedAction is the action the assistant offered the member,
	// after validation against the D-25 allowlist, or empty. Recorded
	// because "what does it try to make people do" is a question about
	// the assistant that nothing else in the row answers, and because a
	// grader needs to see that an answer ended in a button before
	// judging whether it should have.
	ProposedAction string `json:"proposed_action,omitempty"`
	// Flags mirror the four columns on chat_messages.
	OutOfScope bool `json:"out_of_scope"`
	NoSupport  bool `json:"no_support"`
	Degraded   bool `json:"degraded"`
	QAMatch    bool `json:"qa_match"`
	// FinishReason as the provider reported it. "length" means the
	// answer was cut off at AskRogerAnswerMaxTokens, which a reader
	// sees as a sentence stopping mid-word and a grader should not
	// blame the model's judgement for.
	FinishReason string `json:"finish_reason,omitempty"`
	Truncated    bool   `json:"truncated"`
}

// NewChatDecision builds the decision_log row for one answer.
//
// Returned rather than written so the caller can log it on the same
// best-effort footing as the JD pipeline does: the answer has already
// been persisted and shown, and a logging failure must never be the
// reason a member's question fails.
func NewChatDecision(
	messageID int64, model, promptText, responseText string,
	in ChatDecisionInput, out ChatDecisionOutput,
	promptTokens, completionTokens int32, promptVersion, numCtx int,
	callErr string,
) (Decision, error) {
	inJSON, err := json.Marshal(in)
	if err != nil {
		return Decision{}, fmt.Errorf("encode chat decision input: %w", err)
	}
	outJSON, err := json.Marshal(out)
	if err != nil {
		return Decision{}, fmt.Errorf("encode chat decision output: %w", err)
	}
	// model is "code" for the paths that never called one: a bank hit,
	// a scope refusal, a no-support answer. Those still belong in the
	// log. "Did the bank fire when it should not have" is one of the
	// most valuable labels the owner can give, and it cannot be asked
	// about a row that was never written.
	if model == "" {
		model = "code"
	}
	return Decision{
		Kind:     "chat_answer",
		RefKind:  "chat_message",
		RefID:    messageID,
		Model:    model,
		PromptID: "ask_roger_persona",
		// Recorded rather than left at zero. A row that cannot say
		// which persona version wrote it, or how much context the model
		// was given, cannot be compared against another row, and
		// comparing rows is the entire purpose of the table.
		PromptVersion: promptVersion,
		NumCtx:        numCtx,
		Input:         inJSON,
		Output:        outJSON,
		PromptText:    promptText,
		ResponseText:  responseText,
		PromptTokens:  promptTokens,
		CompletionTok: completionTokens,
		LatencyMs:     in.Timings.TotalMs,
		FirstTokenMs:  in.Timings.FirstTokenMs,
		Error:         callErr,
	}, nil
}
