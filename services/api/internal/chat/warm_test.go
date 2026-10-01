package chat

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/reh3376/career-site/services/api/internal/llm"
	"github.com/reh3376/career-site/services/api/internal/users"
)

type fakeFactsSource struct {
	calls  int
	chunks []users.CorpusHit
	err    error
}

func (f *fakeFactsSource) ListChunksByKind(context.Context, string, int) ([]users.CorpusHit, error) {
	f.calls++
	return f.chunks, f.err
}

func factChunk(text string) users.CorpusHit {
	var h users.CorpusHit
	h.Chunk.Text = text
	h.SourceKind = "profile"
	return h
}

// The facts block must be byte-identical across calls, because it is
// the cacheable prefix. Anything unstable here is a cache miss on every
// question, which looks exactly like the warming not working.
func TestTheFactsBlockIsStableAndCached(t *testing.T) {
	src := &fakeFactsSource{chunks: []users.CorpusHit{
		factChunk("Roles and dates."), factChunk("Degrees and credentials."),
	}}
	f := &Facts{Source: src, TTL: time.Hour}

	first := f.Text(context.Background())
	second := f.Text(context.Background())
	if first != second {
		t.Errorf("the facts block changed between calls:\n%q\n%q", first, second)
	}
	if first == "" {
		t.Fatal("no facts block was produced")
	}
	if src.calls != 1 {
		t.Errorf("loaded %d times inside the TTL, want 1", src.calls)
	}
}

// A reindex has to take effect without a restart, so the cache expires.
func TestTheFactsBlockReloadsAfterItsTTL(t *testing.T) {
	src := &fakeFactsSource{chunks: []users.CorpusHit{factChunk("v1")}}
	f := &Facts{Source: src, TTL: time.Nanosecond}
	f.Text(context.Background())
	time.Sleep(2 * time.Nanosecond)
	src.chunks = []users.CorpusHit{factChunk("v2")}
	if got := f.Text(context.Background()); got != "v2" {
		t.Errorf("after the TTL the block is %q, want the reloaded v2", got)
	}
}

// A database that will not answer must not stop the assistant
// answering. Without the facts sheet it is worse, which is a long way
// from broken.
func TestAFailedFactsLoadDoesNotBreakTheAnswer(t *testing.T) {
	f := &Facts{Source: &fakeFactsSource{err: errors.New("down")}, TTL: time.Hour}
	if got := f.Text(context.Background()); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// Nil Facts is a valid Service: the assistant answers without the
// sheet rather than panicking.
func TestNilFactsIsSafe(t *testing.T) {
	var f *Facts
	if got := f.Text(context.Background()); got != "" {
		t.Errorf("got %q from a nil Facts", got)
	}
}

// The warmer asks for one token, not a full answer. Asking for an
// answer would spend fifteen seconds generating something nobody reads.
func TestTheWarmerAsksForOneTokenOnly(t *testing.T) {
	m := &recordingModel{}
	w := &Warmer{Model: m, Facts: &Facts{
		Source: &fakeFactsSource{chunks: []users.CorpusHit{factChunk("Roles.")}},
		TTL:    time.Hour,
	}}
	if err := w.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if m.calls != 1 {
		t.Fatalf("model calls = %d, want 1", m.calls)
	}
	if m.lastMaxTokens != 1 {
		t.Errorf("max tokens = %d, want 1", m.lastMaxTokens)
	}
	if m.lastSystem == "" || m.lastUser == "" {
		t.Error("the warm call sent no prefix, so nothing would be cached")
	}
}

// Nothing to warm is not a failure.
func TestTheWarmerSkipsWhenThereAreNoFacts(t *testing.T) {
	m := &recordingModel{}
	w := &Warmer{Model: m, Facts: &Facts{Source: &fakeFactsSource{}, TTL: time.Hour}}
	if err := w.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if m.calls != 0 {
		t.Errorf("warmed with no facts to warm (%d calls)", m.calls)
	}
}

// A warm call that fails must not surface as a job error: the next
// question pays full price and still answers.
func TestAFailedWarmIsNotAnError(t *testing.T) {
	w := &Warmer{
		Model: &recordingModel{err: errors.New("ollama down")},
		Facts: &Facts{
			Source: &fakeFactsSource{chunks: []users.CorpusHit{factChunk("Roles.")}},
			TTL:    time.Hour,
		},
	}
	if err := w.Run(context.Background()); err != nil {
		t.Errorf("a failed warm returned %v, want nil", err)
	}
}

type recordingModel struct {
	calls         int
	lastSystem    string
	lastUser      string
	lastMaxTokens int
	err           error
}

func (r *recordingModel) Generate(_ context.Context, req llm.Request) (*llm.Response, error) {
	r.calls++
	r.lastSystem, r.lastUser, r.lastMaxTokens = req.System, req.User, req.MaxTokens
	if r.err != nil {
		return nil, r.err
	}
	return &llm.Response{Text: "ok", Model: "test"}, nil
}
