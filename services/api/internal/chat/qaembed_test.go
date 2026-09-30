package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/reh3376/career-site/services/api/internal/ingest"
	"github.com/reh3376/career-site/services/api/internal/users"
)

type fakeQAStore struct {
	pending []users.QAPhrasing
	stored  map[int64][]float32
	failOn  int64
}

func (f *fakeQAStore) QAPhrasingsNeedingEmbedding(context.Context, int) ([]users.QAPhrasing, error) {
	return f.pending, nil
}
func (f *fakeQAStore) SetQAPhrasingEmbedding(_ context.Context, id int64, v []float32) error {
	if id == f.failOn {
		return errors.New("write failed")
	}
	if f.stored == nil {
		f.stored = map[int64][]float32{}
	}
	f.stored[id] = v
	return nil
}

type recordingEmbed struct {
	purpose ingest.EmbedPurpose
	texts   []string
	calls   int
	short   bool
}

func (r *recordingEmbed) Embed(_ context.Context, texts []string, p ingest.EmbedPurpose) ([][]float32, string, error) {
	r.calls++
	r.purpose = p
	r.texts = texts
	n := len(texts)
	if r.short {
		n--
	}
	out := make([][]float32, n)
	for i := range out {
		out[i] = []float32{0.1, 0.2}
	}
	return out, "nomic-embed-text", nil
}

// These vectors are compared against an embedded question, so they have
// to live in the same space as one. nomic-embed-text prefixes queries
// and documents differently, so embedding a phrasing as a document
// would put every one of them slightly off from the thing it exists to
// match, and the symptom would be a bank that almost never fires with
// no error anywhere.
func TestPhrasingsAreEmbeddedAsQueriesNotDocuments(t *testing.T) {
	store := &fakeQAStore{pending: []users.QAPhrasing{{ID: 1, Text: "would you move?"}}}
	em := &recordingEmbed{}
	if err := (&QAEmbedder{Embed: em, Store: store}).Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if em.purpose != ingest.PurposeQuery {
		t.Errorf("purpose = %q, want %q", em.purpose, ingest.PurposeQuery)
	}
}

// One call for the batch, not one per phrasing: on this box the
// per-call overhead dominates a short question.
func TestTheBatchIsOneEmbedCall(t *testing.T) {
	store := &fakeQAStore{pending: []users.QAPhrasing{
		{ID: 1, Text: "a"}, {ID: 2, Text: "b"}, {ID: 3, Text: "c"},
	}}
	em := &recordingEmbed{}
	if err := (&QAEmbedder{Embed: em, Store: store}).Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if em.calls != 1 {
		t.Errorf("embed calls = %d, want 1 for the whole batch", em.calls)
	}
	if len(store.stored) != 3 {
		t.Errorf("stored %d vectors, want 3", len(store.stored))
	}
}

// A mismatched count means the vectors cannot be trusted to line up
// with the phrasings, and storing them anyway would attach the wrong
// vector to the wrong wording: the bank would then match confidently
// and wrongly, which is the failure this whole design tries hardest to
// avoid.
func TestAMismatchedVectorCountStoresNothing(t *testing.T) {
	store := &fakeQAStore{pending: []users.QAPhrasing{{ID: 1, Text: "a"}, {ID: 2, Text: "b"}}}
	em := &recordingEmbed{short: true}
	err := (&QAEmbedder{Embed: em, Store: store}).Run(context.Background())
	if err == nil {
		t.Fatal("a short vector list was accepted")
	}
	if len(store.stored) != 0 {
		t.Errorf("stored %d vectors despite the mismatch", len(store.stored))
	}
}

// One bad row must not strand the batch; the next tick finds it again.
func TestOneFailedWriteDoesNotStrandTheBatch(t *testing.T) {
	store := &fakeQAStore{
		pending: []users.QAPhrasing{{ID: 1, Text: "a"}, {ID: 2, Text: "b"}, {ID: 3, Text: "c"}},
		failOn:  2,
	}
	if err := (&QAEmbedder{Embed: &recordingEmbed{}, Store: store}).Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(store.stored) != 2 {
		t.Errorf("stored %d, want the two that could be written", len(store.stored))
	}
	if _, ok := store.stored[2]; ok {
		t.Error("the failing row was recorded as stored")
	}
}

func TestNothingPendingIsNotAnError(t *testing.T) {
	em := &recordingEmbed{}
	if err := (&QAEmbedder{Embed: em, Store: &fakeQAStore{}}).Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if em.calls != 0 {
		t.Error("an empty batch still called the embedder")
	}
}
