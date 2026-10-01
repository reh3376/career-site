package chat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/reh3376/career-site/services/api/internal/ingest"
	"github.com/reh3376/career-site/services/api/internal/llm"
	"github.com/reh3376/career-site/services/api/internal/prompts"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Fakes rather than a database, because what is under test here is the
// order of the stages and what each one records, not SQL. The store
// layer has its own tests against a real Postgres.

type fakeEmbed struct {
	err error
}

func (f fakeEmbed) Embed(_ context.Context, texts []string, _ ingest.EmbedPurpose) ([][]float32, string, error) {
	if f.err != nil {
		return nil, "", f.err
	}
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{0.1, 0.2, 0.3}
	}
	return out, "nomic-embed-text", nil
}

type fakeModel struct {
	text    string
	finish  string
	err     error
	calls   int
	noSplit bool
}

func (f *fakeModel) Generate(_ context.Context, _ llm.Request) (*llm.Response, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &llm.Response{
		Text: f.text, Model: "ollama:qwen3:4b-q8_0", FinishReason: f.finish,
		PromptTokens: 1019, CompletionTokens: 187,
		PromptEvalMs: splitOr(f.noSplit, 31000), EvalMs: splitOr(f.noSplit, 14000),
	}, nil
}

type fakeStore struct {
	qa       users.QAMatch
	qaHit    bool
	hits     []users.CorpusHit
	messages []users.ChatMessage
	logged   []users.Decision
	usage    []users.LLMUsage
	nextID   int64
}

func (f *fakeStore) MatchQA(context.Context, []float32, float64) (users.QAMatch, bool, error) {
	return f.qa, f.qaHit, nil
}
func (f *fakeStore) SearchCorpusForChat(context.Context, []float32, int) ([]users.CorpusHit, error) {
	return f.hits, nil
}
func (f *fakeStore) AppendMessage(_ context.Context, m users.ChatMessage) (int64, error) {
	f.nextID++
	m.ID = f.nextID
	f.messages = append(f.messages, m)
	return m.ID, nil
}
func (f *fakeStore) InsertDecisions(_ context.Context, rows []users.Decision) error {
	f.logged = append(f.logged, rows...)
	return nil
}
func (f *fakeStore) RecordLLMUsage(_ context.Context, u users.LLMUsage) error {
	f.usage = append(f.usage, u)
	return nil
}

func (f *fakeStore) decisionInput(t *testing.T) users.ChatDecisionInput {
	t.Helper()
	if len(f.logged) != 1 {
		t.Fatalf("logged %d decisions, want exactly 1", len(f.logged))
	}
	var in users.ChatDecisionInput
	if err := json.Unmarshal(f.logged[0].Input, &in); err != nil {
		t.Fatalf("decode decision input: %v", err)
	}
	return in
}

func (f *fakeStore) decisionOutput(t *testing.T) users.ChatDecisionOutput {
	t.Helper()
	var out users.ChatDecisionOutput
	if err := json.Unmarshal(f.logged[0].Output, &out); err != nil {
		t.Fatalf("decode decision output: %v", err)
	}
	return out
}

func hit(id int64, title, path, text string, sim float32) users.CorpusHit {
	h := users.CorpusHit{Title: title, SourcePath: path, Visibility: "public", Similarity: sim}
	h.Chunk.ID = id
	h.Chunk.Text = text
	return h
}

func svc(store *fakeStore, model *fakeModel) *Service {
	return &Service{Embed: fakeEmbed{}, Model: model, Store: store}
}

// The bank is the fast path on this hardware, and "fast" means the
// model is never called. If a bank hit still paid for generation the
// whole reason the bank exists would be gone, and nothing about the
// answer a member sees would reveal it.
func TestBankHitAnswersVerbatimWithoutTheModel(t *testing.T) {
	store := &fakeStore{
		qaHit: true,
		qa: users.QAMatch{
			Similarity: 0.93,
			Phrasing:   "would you move?",
			Entry: users.QAEntry{
				ID:      7,
				Answer:  "I am, for the right role.",
				Sources: []users.QASource{{Title: "About", Path: "/about"}},
			},
		},
	}
	model := &fakeModel{text: "should never be used"}
	ans, err := svc(store, model).Answer(context.Background(),
		Request{ConversationID: 1, Question: "are you open to relocating?"})
	if err != nil {
		t.Fatalf("answer: %v", err)
	}

	if model.calls != 0 {
		t.Errorf("the model was called %d times on a bank hit", model.calls)
	}
	if ans.Text != "I am, for the right role." {
		t.Errorf("the bank answer was not served verbatim: %q", ans.Text)
	}
	if !ans.QAMatch || ans.Path != users.ChatPathQABank {
		t.Errorf("path = %q, qa_match = %v", ans.Path, ans.QAMatch)
	}
	if len(ans.Citations) != 1 || ans.Citations[0].Path != "/about" {
		t.Errorf("bank sources did not become citations: %+v", ans.Citations)
	}
	in := store.decisionInput(t)
	if !in.QA.Matched || in.QA.EntryID != 7 || in.QA.BestSimilarity != 0.93 {
		t.Errorf("the bank lookup was not recorded: %+v", in.QA)
	}
	if store.logged[0].Model != "code" {
		t.Errorf("model = %q, want code for a path that called no model", store.logged[0].Model)
	}
}

// A miss is worth as much as a hit and only exists if it is written
// down: the near misses are the only thing that can calibrate
// QAMatchThreshold.
func TestBankMissIsRecordedWithItsBestSimilarity(t *testing.T) {
	store := &fakeStore{
		qaHit: false,
		qa:    users.QAMatch{Similarity: 0.61},
		hits:  []users.CorpusHit{hit(1, "Roles", "/cv", "Thirty years of controls work.", 0.8)},
	}
	model := &fakeModel{text: "Thirty years of it [1]."}
	if _, err := svc(store, model).Answer(context.Background(),
		Request{ConversationID: 1, Question: "how long have you done this?"}); err != nil {
		t.Fatalf("answer: %v", err)
	}
	in := store.decisionInput(t)
	if in.QA.Matched {
		t.Error("recorded as matched")
	}
	if in.QA.BestSimilarity != 0.61 {
		t.Errorf("best similarity = %v, want the near miss to be kept", in.QA.BestSimilarity)
	}
	if in.QA.Threshold != users.QAMatchThreshold {
		t.Errorf("threshold = %v, want the one actually applied", in.QA.Threshold)
	}
}

// FR-CHAT-06. The bank is consulted first, so reaching the refusal
// means the owner has written no approved statement. Refusing in code
// costs nothing; asking the model to refuse costs twenty seconds and
// can be argued with.
func TestRestrictedTopicsAreRefusedWithoutTheModel(t *testing.T) {
	for _, q := range []string{
		"what are your salary expectations?",
		"What Are Your Compensation Requirements",
		"can you provide references?",
		"tell me something confidential about your employer",
		"are you married?",
	} {
		store := &fakeStore{hits: []users.CorpusHit{hit(1, "t", "/p", "x", 0.9)}}
		model := &fakeModel{text: "should never be used"}
		ans, err := svc(store, model).Answer(context.Background(),
			Request{ConversationID: 1, Question: q})
		if err != nil {
			t.Fatalf("%q: %v", q, err)
		}
		if model.calls != 0 {
			t.Errorf("%q reached the model", q)
		}
		if !ans.OutOfScope || ans.Path != users.ChatPathOutOfScope {
			t.Errorf("%q: path = %q, out_of_scope = %v", q, ans.Path, ans.OutOfScope)
		}
		if ans.Text == "" {
			t.Errorf("%q: refused with no reply", q)
		}
	}
}

// The refusal is a backstop for wordings that must never get through,
// so it has to stay narrow. A false positive silently refuses a fair
// question, which is the expensive mistake here.
func TestOrdinaryQuestionsAreNotMistakenForRestrictedOnes(t *testing.T) {
	for _, q := range []string{
		"what pay-per-view systems have you integrated?",
		"how do you reference a tag in Ignition?",
		"what is your approach to confidential data in a historian?",
		"how old is the MDEMG framework?",
	} {
		store := &fakeStore{hits: []users.CorpusHit{hit(1, "t", "/p", "x", 0.9)}}
		model := &fakeModel{text: "An answer [1]."}
		ans, err := svc(store, model).Answer(context.Background(),
			Request{ConversationID: 1, Question: q})
		if err != nil {
			t.Fatalf("%q: %v", q, err)
		}
		if ans.OutOfScope {
			t.Errorf("%q was refused as a restricted topic", q)
		}
	}
}

// FR-CHAT-03: an answer with no support takes the "I don't know" path,
// and that decision needs no model.
func TestNothingRetrievedTakesTheNoSupportPath(t *testing.T) {
	store := &fakeStore{}
	model := &fakeModel{text: "should never be used"}
	ans, err := svc(store, model).Answer(context.Background(),
		Request{ConversationID: 1, Question: "what is your favourite compiler?"})
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if model.calls != 0 {
		t.Error("the model was asked to answer with nothing retrieved")
	}
	if !ans.NoSupport || ans.Path != users.ChatPathNoSupport {
		t.Errorf("path = %q, no_support = %v", ans.Path, ans.NoSupport)
	}
}

// FR-CHAT-17. The bank has already been consulted by this point, so
// there is nothing to fall back to except saying so honestly.
func TestModelFailureDegradesRatherThanFails(t *testing.T) {
	store := &fakeStore{hits: []users.CorpusHit{hit(1, "t", "/p", "x", 0.9)}}
	model := &fakeModel{err: errors.New("connection refused")}
	ans, err := svc(store, model).Answer(context.Background(),
		Request{ConversationID: 1, Question: "tell me about your work"})
	if err != nil {
		t.Fatalf("a model outage must not fail the question: %v", err)
	}
	if !ans.Degraded || ans.Path != users.ChatPathDegraded {
		t.Errorf("path = %q, degraded = %v", ans.Path, ans.Degraded)
	}
	if store.logged[0].Error == "" {
		t.Error("the outage was not recorded on the decision row")
	}
}

// Only what retrieval returned may be cited. A dangling marker is an
// answer claiming support it does not have, in the one place a
// sceptical reader will check.
func TestInventedCitationMarkersAreDropped(t *testing.T) {
	store := &fakeStore{hits: []users.CorpusHit{
		hit(11, "Roles", "/cv", "Controls work.", 0.9),
		hit(22, "Writing", "/writing/uns", "Unified namespace.", 0.8),
	}}
	model := &fakeModel{text: "I did that at Joy Global [1]. It is written up too [2]. And here [7]."}
	ans, err := svc(store, model).Answer(context.Background(),
		Request{ConversationID: 1, Question: "tell me about it"})
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if strings.Contains(ans.Text, "[7]") {
		t.Errorf("an invented marker survived into the answer: %q", ans.Text)
	}
	if !strings.Contains(ans.Text, "[1]") || !strings.Contains(ans.Text, "[2]") {
		t.Errorf("valid markers were removed: %q", ans.Text)
	}
	if len(ans.Citations) != 2 {
		t.Fatalf("citations = %d, want 2: %+v", len(ans.Citations), ans.Citations)
	}
	if ans.Citations[0].ChunkID != 11 || ans.Citations[1].ChunkID != 22 {
		t.Errorf("citations point at the wrong chunks: %+v", ans.Citations)
	}
	out := store.decisionOutput(t)
	if out.MarkersOffered != 2 || out.MarkersWritten != 3 || out.MarkersDropped != 1 {
		t.Errorf("offered/written/dropped = %d/%d/%d, want 2/3/1",
			out.MarkersOffered, out.MarkersWritten, out.MarkersDropped)
	}
}

// Removing a marker must not leave the seam visible.
func TestRemovingAMarkerLeavesCleanText(t *testing.T) {
	store := &fakeStore{hits: []users.CorpusHit{hit(11, "Roles", "/cv", "x", 0.9)}}
	model := &fakeModel{text: "I built that [9]. Then this [4] , and more."}
	ans, _ := svc(store, model).Answer(context.Background(),
		Request{ConversationID: 1, Question: "q"})
	for _, bad := range []string{"  ", " .", " ,"} {
		if strings.Contains(ans.Text, bad) {
			t.Errorf("stripping markers left %q in: %q", bad, ans.Text)
		}
	}
}

// Private material may inform an answer and must never be named. It is
// kept out of the decision record's titles too, because a private
// document's title is itself something the owner did not publish.
func TestPrivatePassagesAreNeitherCitableNorNamedInTheRecord(t *testing.T) {
	priv := hit(90, "Whiskey House engagement", "", "Confidential detail.", 0.95)
	priv.Visibility = users.VisibilityCorpusOnly
	store := &fakeStore{hits: []users.CorpusHit{priv, hit(11, "Roles", "/cv", "Public detail.", 0.7)}}
	model := &fakeModel{text: "Something informed by both [1]."}
	ans, err := svc(store, model).Answer(context.Background(),
		Request{ConversationID: 1, Question: "q", AllowPrivate: true})
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	for _, c := range ans.Citations {
		if c.ChunkID == 90 {
			t.Error("a private passage was cited")
		}
	}
	in := store.decisionInput(t)
	for _, r := range in.Retrieved {
		if r.ChunkID == 90 {
			if r.Citable {
				t.Error("a private passage was recorded as citable")
			}
			if r.Title != "" {
				t.Errorf("a private document's title reached the record: %q", r.Title)
			}
		}
	}
}

// What retrieval found and did not show is how a retrieval problem is
// told from a generation one.
func TestRetrievalRecordsWhatWasFoundAndWhatWasShown(t *testing.T) {
	var hits []users.CorpusHit
	for i := int64(1); i <= 6; i++ {
		hits = append(hits, hit(i, "t", "/p", "x", float32(1.0)-float32(i)/10))
	}
	store := &fakeStore{hits: hits}
	s := svc(store, &fakeModel{text: "answer [1]"})
	s.Show = 2
	if _, err := s.Answer(context.Background(), Request{ConversationID: 1, Question: "q"}); err != nil {
		t.Fatalf("answer: %v", err)
	}
	in := store.decisionInput(t)
	if len(in.Retrieved) != 6 {
		t.Fatalf("recorded %d hits, want all 6 that retrieval returned", len(in.Retrieved))
	}
	shown := 0
	for _, r := range in.Retrieved {
		if r.Shown {
			shown++
		}
	}
	if shown != 2 {
		t.Errorf("recorded %d as shown, want 2", shown)
	}
	// The two shown passages are the ones offered as [1] and [2].
	if in.Retrieved[0].Marker != 1 || in.Retrieved[1].Marker != 2 {
		t.Errorf("markers not recorded against the passages offered: %+v", in.Retrieved[:2])
	}
	if in.Retrieved[5].Marker != 0 {
		t.Errorf("an unshown passage carries marker %d", in.Retrieved[5].Marker)
	}
}

// Every path writes a message and a decision row, including the ones
// that never call a model. Without that, "did the bank fire when it
// should not have" cannot be asked.
func TestEveryPathPersistsAMessageAndADecision(t *testing.T) {
	cases := []struct {
		name  string
		store *fakeStore
		model *fakeModel
		want  string
	}{
		{"bank", &fakeStore{qaHit: true, qa: users.QAMatch{Entry: users.QAEntry{Answer: "a"}}}, &fakeModel{}, users.ChatPathQABank},
		{"no support", &fakeStore{}, &fakeModel{}, users.ChatPathNoSupport},
		{"model", &fakeStore{hits: []users.CorpusHit{hit(1, "t", "/p", "x", 0.9)}}, &fakeModel{text: "a"}, users.ChatPathModel},
		{"degraded", &fakeStore{hits: []users.CorpusHit{hit(1, "t", "/p", "x", 0.9)}}, &fakeModel{err: errors.New("down")}, users.ChatPathDegraded},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ans, err := svc(c.store, c.model).Answer(context.Background(),
				Request{ConversationID: 1, Question: "tell me about the work"})
			if err != nil {
				t.Fatalf("answer: %v", err)
			}
			if ans.Path != c.want {
				t.Errorf("path = %q, want %q", ans.Path, c.want)
			}
			if len(c.store.messages) != 1 {
				t.Errorf("messages persisted = %d, want 1", len(c.store.messages))
			}
			if len(c.store.logged) != 1 {
				t.Fatalf("decisions logged = %d, want 1", len(c.store.logged))
			}
			d := c.store.logged[0]
			if d.Kind != "chat_answer" || d.RefKind != "chat_message" || d.RefID != ans.MessageID {
				t.Errorf("decision not joined to the message: %+v", d)
			}
			in := c.store.decisionInput(t)
			if in.Path != c.want {
				t.Errorf("recorded path = %q, want %q", in.Path, c.want)
			}
			if in.PersonaFingerprint != prompts.AskRogerPersona.Fingerprint() {
				t.Error("the persona fingerprint was not recorded")
			}
		})
	}
}

// An embedding failure means no bank lookup and no retrieval, so there
// is nothing to ground an answer in. Saying so beats guessing.
func TestEmbeddingFailureAnswersHonestlyAndRecordsWhy(t *testing.T) {
	store := &fakeStore{}
	s := &Service{Embed: fakeEmbed{err: errors.New("sidecar down")}, Model: &fakeModel{}, Store: store}
	ans, err := s.Answer(context.Background(), Request{ConversationID: 1, Question: "q"})
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if ans.Path != users.ChatPathError {
		t.Errorf("path = %q, want %q", ans.Path, users.ChatPathError)
	}
	if !strings.Contains(store.logged[0].Error, "sidecar down") {
		t.Errorf("the cause was not recorded: %q", store.logged[0].Error)
	}
}

func TestEmptyQuestionIsACallerBug(t *testing.T) {
	store := &fakeStore{}
	if _, err := svc(store, &fakeModel{}).Answer(context.Background(),
		Request{ConversationID: 1, Question: "   "}); !errors.Is(err, ErrNoQuestion) {
		t.Errorf("err = %v, want ErrNoQuestion", err)
	}
	if len(store.messages) != 0 {
		t.Error("an empty question still wrote a message")
	}
}

// Completion tokens have to reach the decision row.
//
// They were hardcoded to zero, which was invisible until production
// answers started taking forty-five seconds and there was no way to
// tell how much of that was generation. At roughly 6.5 tokens a second
// on this box, the completion count IS the generation time, so losing
// it means losing the only number that says whether to write shorter
// answers or retrieve fewer passages.
func TestTokenCountsReachTheDecisionRow(t *testing.T) {
	store := &fakeStore{hits: []users.CorpusHit{hit(1, "Roles", "/cv", "x", 0.9)}}
	if _, err := svc(store, &fakeModel{text: "An answer [1]."}).Answer(
		context.Background(), Request{ConversationID: 1, Question: "q"}); err != nil {
		t.Fatalf("answer: %v", err)
	}
	if len(store.logged) != 1 {
		t.Fatalf("logged %d decisions", len(store.logged))
	}
	d := store.logged[0]
	if d.PromptTokens != 1019 {
		t.Errorf("prompt tokens = %d, want 1019", d.PromptTokens)
	}
	if d.CompletionTok != 187 {
		t.Errorf("completion tokens = %d, want 187; generation time cannot be read back without it", d.CompletionTok)
	}
}

// Everything a row needs to be comparable with another row.
//
// A production row was found carrying prompt_version 0, num_ctx 0,
// completion_tokens 0 and no run id, which makes it a record of an
// answer and not a usable training or evaluation example: nothing about
// it can be compared against a row from a different persona version or
// a different context window, and it cannot be joined to the usage
// ledger.
func TestADecisionRowIsComparable(t *testing.T) {
	store := &fakeStore{hits: []users.CorpusHit{hit(1, "Roles", "/cv", "x", 0.9)}}
	s := svc(store, &fakeModel{text: "An answer [1]."})
	s.NumCtx = 8192
	if _, err := s.Answer(context.Background(), Request{ConversationID: 7, Question: "q"}); err != nil {
		t.Fatalf("answer: %v", err)
	}
	d := store.logged[0]
	if d.PromptVersion != prompts.AskRogerPersona.Version {
		t.Errorf("prompt_version = %d, want %d", d.PromptVersion, prompts.AskRogerPersona.Version)
	}
	if d.NumCtx != 8192 {
		t.Errorf("num_ctx = %d, want 8192", d.NumCtx)
	}
	if d.Model == "" {
		t.Error("model is empty")
	}
}

// The usage ledger is where the budget cap reads from (FR-CHAT-12) and
// where tokens are totted up across surfaces. The chat path wrote
// nothing to it at all, so the cap could never have fired and chat
// spend was invisible next to the JD reviewer's.
func TestTheModelCallIsRecordedInTheUsageLedger(t *testing.T) {
	store := &fakeStore{hits: []users.CorpusHit{hit(1, "t", "/p", "x", 0.9)}}
	if _, err := svc(store, &fakeModel{text: "a [1]"}).Answer(
		context.Background(), Request{ConversationID: 7, Question: "q"}); err != nil {
		t.Fatalf("answer: %v", err)
	}
	if len(store.usage) != 1 {
		t.Fatalf("usage rows = %d, want 1", len(store.usage))
	}
	u := store.usage[0]
	if u.Kind != "chat" || u.RefID != 7 || !u.OK {
		t.Errorf("usage row wrong: %+v", u)
	}
	if u.CompletionTokens != 187 || u.PromptTokens != 1019 {
		t.Errorf("usage tokens = %d/%d, want 1019/187", u.PromptTokens, u.CompletionTokens)
	}
}

// A failed call costs time and sometimes tokens, and a ledger that
// omits it reports a system cheaper and healthier than it is.
func TestAFailedModelCallIsAlsoRecorded(t *testing.T) {
	store := &fakeStore{hits: []users.CorpusHit{hit(1, "t", "/p", "x", 0.9)}}
	if _, err := svc(store, &fakeModel{err: errors.New("down")}).Answer(
		context.Background(), Request{ConversationID: 7, Question: "q"}); err != nil {
		t.Fatalf("answer: %v", err)
	}
	if len(store.usage) != 1 {
		t.Fatalf("usage rows = %d, want 1 even on failure", len(store.usage))
	}
	if store.usage[0].OK || store.usage[0].Error == "" {
		t.Errorf("the failure was recorded as a success: %+v", store.usage[0])
	}
}

// Paths that never call a model must not invent a ledger row.
func TestNonModelPathsWriteNoUsage(t *testing.T) {
	store := &fakeStore{qaHit: true, qa: users.QAMatch{Entry: users.QAEntry{Answer: "a"}}}
	if _, err := svc(store, &fakeModel{}).Answer(
		context.Background(), Request{ConversationID: 7, Question: "q"}); err != nil {
		t.Fatalf("answer: %v", err)
	}
	if len(store.usage) != 0 {
		t.Errorf("a bank hit wrote %d usage rows", len(store.usage))
	}
}

// The provider's own split has to survive into the row.
//
// A production answer took 45 seconds and the row recorded
// first_token_ms as 44975 and latency_ms as 45009, which is the same
// number twice and says nothing. Ollama had reported the split all
// along in prompt_eval_duration and eval_duration; the sidecar simply
// never read them. Without it there is no way to tell an answer that
// was slow because it read too much from one that was slow because it
// wrote too much, and those are fixed in opposite directions.
func TestTheProviderTimingSplitIsRecorded(t *testing.T) {
	store := &fakeStore{hits: []users.CorpusHit{hit(1, "t", "/p", "x", 0.9)}}
	if _, err := svc(store, &fakeModel{text: "a [1]"}).Answer(
		context.Background(), Request{ConversationID: 1, Question: "q"}); err != nil {
		t.Fatalf("answer: %v", err)
	}
	in := store.decisionInput(t)
	if in.Timings.PromptEvalMs != 31000 || in.Timings.GenerateMs != 14000 {
		t.Errorf("split = %d/%d, want 31000/14000",
			in.Timings.PromptEvalMs, in.Timings.GenerateMs)
	}
	// The honest first-token figure is the prompt evaluation, not the
	// whole call.
	if in.Timings.FirstTokenMs != 31000 {
		t.Errorf("first_token_ms = %d, want the prompt-eval time 31000", in.Timings.FirstTokenMs)
	}
}

// A provider that reports no split must not leave the row claiming an
// instant first token.
func TestFirstTokenFallsBackToTheWholeCall(t *testing.T) {
	store := &fakeStore{hits: []users.CorpusHit{hit(1, "t", "/p", "x", 0.9)}}
	m := &fakeModel{text: "a [1]", noSplit: true}
	svc := svc(store, m)
	// A fake model answers instantly, so without a clock the fallback
	// is legitimately zero and the test would prove nothing. Each read
	// advances a second.
	now := time.Unix(0, 0)
	svc.Now = func() time.Time { now = now.Add(time.Second); return now }
	if _, err := svc.Answer(
		context.Background(), Request{ConversationID: 1, Question: "q"}); err != nil {
		t.Fatalf("answer: %v", err)
	}
	if store.decisionInput(t).Timings.FirstTokenMs == 0 {
		t.Error("first_token_ms is zero, which reads as instant")
	}
}

func splitOr(off bool, v int64) int64 {
	if off {
		return 0
	}
	return v
}
