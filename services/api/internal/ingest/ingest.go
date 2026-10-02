package ingest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/reh3376/career-site/services/api/internal/users"
)

// Ingester turns raw source text into corpus rows + embeddings. Its
// contract is deliberately simple so a follow-up filesystem walker,
// admin form, and sidecar CLI can all reuse the same code path:
//
//	IngestText(ctx, in) → summary
//
// The insert-first / embed-later pattern is honoured: the document
// and chunks are written before the sidecar Embed call, so a
// mid-ingest failure leaves a partially-embedded row rather than
// losing the text entirely. A follow-up embed-sweep RPC (not built
// yet) can fill in missing embeddings.
type Ingester struct {
	log     *slog.Logger
	users   *users.Repo
	sidecar EmbedClient
	chunker Chunker
}

// EmbedClient is the shape Ingester needs from the sidecar client:
// batch a list of texts into vectors plus a model identifier. Kept
// deliberately narrow so tests can pass a fake without booting the
// gRPC stack, and so the ingest package doesn't depend on the
// generated proto types.
type EmbedClient interface {
	Embed(ctx context.Context, texts []string, purpose EmbedPurpose) (vectors [][]float32, model string, err error)
}

// EmbedPurpose tells the embedder which side of retrieval a text is
// on. Asymmetric models (nomic-embed-text) prefix documents and
// queries differently; embedding both sides the same way flattens the
// similarity distribution and ruins threshold calibration.
type EmbedPurpose string

const (
	PurposeDocument EmbedPurpose = "document"
	PurposeQuery    EmbedPurpose = "query"
)

// NewIngester wires the dependencies. Passing nil for chunker uses
// the v1 ParagraphChunker with default options.
func NewIngester(log *slog.Logger, repo *users.Repo, sc EmbedClient, chunker Chunker) *Ingester {
	if chunker == nil {
		chunker = NewParagraphChunker(Options{})
	}
	return &Ingester{log: log, users: repo, sidecar: sc, chunker: chunker}
}

// IngestInput is one document to ingest. SourceKind is a slug like
// "article" / "resume" / "career_note". SourcePath is a stable key
// per source_kind — for pasted text, callers pass a synthesised key
// like `paste/<slug>`; for a filesystem source, the relative path.
type IngestInput struct {
	SourceKind string
	SourcePath string
	Title      string
	Visibility string // users.VisibilityPublic (default) | users.VisibilityCorpusOnly
	Body       string // raw text (post-front-matter for markdown)
	Meta       []byte // jsonb, optional
}

// KnownKinds is the allow-list of source_kind slugs. The private
// corpus walker maps each top-level directory to a kind and refuses
// anything not listed, so a typo in a sync manifest can't create a
// stray kind. Adding a kind is a one-line edit here plus a UI label.
var KnownKinds = []string{
	"article", "talk", "speaker_notes", "readme", "worksheet",
	"post_mortem", "strategy_doc", "interview_prep", "career_note",
	"resume", "adr", "other",
	// profile: the owner-maintained career facts sheet (roles with
	// dates, degrees, certifications, compliance ownership). Every JD
	// requirement is judged with it on the table, because embedding
	// retrieval is unreliable for tenure / title / credential facts.
	// Keep it short: it is added to every judge call.
	"profile",
}

// IsKnownKind reports whether k is in KnownKinds.
func IsKnownKind(k string) bool {
	return slices.Contains(KnownKinds, k)
}

// IngestResult reports what the ingester did with one document.
// Skipped=true means the content_hash matched the stored row and
// no chunking / embedding ran.
type IngestResult struct {
	DocumentID     int64
	ChunksInserted int
	ChunksEmbedded int
	Skipped        bool
	ChunkerName    string
	EmbedderModel  string
	StartedAt      time.Time
	CompletedAt    time.Time
}

// ErrEmpty is returned when the caller passes an empty body. Ingest
// treats that as a caller error, not something to store as an empty
// document.
var ErrEmpty = errors.New("ingest: body is empty")

// IngestText runs the full pipeline for one document.
func (i *Ingester) IngestText(ctx context.Context, in IngestInput) (*IngestResult, error) {
	res := &IngestResult{StartedAt: time.Now().UTC(), ChunkerName: i.chunker.Name()}

	// Comments come out first, so the title, the content hash and the
	// chunks are all taken from the same comment-free text. See
	// stripHTMLComments for why.
	body := stripHTMLComments(strings.TrimSpace(in.Body))
	if body == "" {
		return nil, ErrEmpty
	}
	if in.SourceKind == "" || in.SourcePath == "" {
		return nil, errors.New("ingest: source_kind and source_path are required")
	}
	if in.Visibility == "" {
		in.Visibility = users.VisibilityPublic
	}
	if !users.ValidVisibility(in.Visibility) {
		return nil, fmt.Errorf("ingest: invalid visibility %q", in.Visibility)
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = firstNonEmptyLine(body)
	}
	if title == "" {
		title = in.SourcePath
	}

	newHash := users.HashCorpusContent(body)

	// Content-hash short-circuit: if a document with this
	// (source_kind, source_path) already exists AND its content_hash
	// equals what we're about to store, nothing has changed —
	// re-ingest is a no-op. Callers pay the SELECT but nothing else.
	// A visibility change on unchanged text still needs the upsert
	// (it's a metadata write, no re-chunk), so the short-circuit also
	// requires visibility to match.
	if existing, err := i.users.GetCorpusDocumentByPath(ctx, in.SourceKind, in.SourcePath); err == nil {
		if existing != nil && bytes.Equal(existing.ContentHash, newHash) && existing.Visibility == in.Visibility {
			res.DocumentID = existing.ID
			res.Skipped = true
			res.CompletedAt = time.Now().UTC()
			i.log.Info("ingest skipped (unchanged)",
				slog.String("source_kind", in.SourceKind),
				slog.String("source_path", in.SourcePath),
				slog.Int64("document_id", existing.ID),
			)
			return res, nil
		}
	}

	doc, err := i.users.UpsertCorpusDocument(ctx, users.CorpusDocument{
		SourceKind:  in.SourceKind,
		SourcePath:  in.SourcePath,
		Title:       title,
		Visibility:  in.Visibility,
		Meta:        in.Meta,
		ContentHash: newHash,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert doc: %w", err)
	}
	res.DocumentID = doc.ID

	// Chunk + insert (no embedding yet — insert-first).
	chunks := i.chunker.Chunk(body)
	if len(chunks) == 0 {
		res.CompletedAt = time.Now().UTC()
		return res, nil
	}
	chunkIDs := make([]int64, 0, len(chunks))
	texts := make([]string, 0, len(chunks))
	for _, c := range chunks {
		stored, err := i.users.UpsertCorpusChunk(ctx, users.CorpusChunk{
			DocumentID: doc.ID,
			ChunkIndex: c.Index,
			Text:       c.Text,
			TokenCount: c.TokenCount,
		})
		if err != nil {
			return nil, fmt.Errorf("upsert chunk %d: %w", c.Index, err)
		}
		chunkIDs = append(chunkIDs, stored.ID)
		texts = append(texts, c.Text)
		res.ChunksInserted++
	}

	// Embed pass. Sidecar Embed accepts a batch (max 64 per proto);
	// split into batches so an oversized document still lands.
	const batchSize = 64
	for start := 0; start < len(texts); start += batchSize {
		end := min(start+batchSize, len(texts))
		vectors, model, err := i.sidecar.Embed(ctx, texts[start:end], PurposeDocument)
		if err != nil {
			i.log.Warn("embed batch failed",
				slog.Int64("document_id", doc.ID),
				slog.Int("batch_start", start),
				slog.String("error", err.Error()),
			)
			// Leave the already-inserted chunks with nil embedding;
			// a follow-up sweep can retry without re-chunking.
			continue
		}
		res.EmbedderModel = model
		for j, emb := range vectors {
			if err := i.users.SetCorpusChunkEmbedding(ctx, chunkIDs[start+j], emb, model); err != nil {
				i.log.Warn("set embedding failed",
					slog.Int64("chunk_id", chunkIDs[start+j]),
					slog.String("error", err.Error()),
				)
				continue
			}
			res.ChunksEmbedded++
		}
	}

	res.CompletedAt = time.Now().UTC()
	i.log.Info("ingest done",
		slog.String("source_kind", in.SourceKind),
		slog.String("source_path", in.SourcePath),
		slog.Int64("document_id", doc.ID),
		slog.Int("chunks_inserted", res.ChunksInserted),
		slog.Int("chunks_embedded", res.ChunksEmbedded),
		slog.String("chunker", res.ChunkerName),
		slog.String("embedder", res.EmbedderModel),
	)
	return res, nil
}

// stripHTMLComments removes `<!-- ... -->` spans, including ones
// spanning several lines, leaving anything inside a fenced code block
// alone.
//
// This exists because of a real leak. The corpus convention for
// private material is a scrubbed copy carrying a comment that records
// what was taken out, for example "three passages were removed here:
// the ownership structure, a vendor named in an internal procurement
// disagreement, and a statement that a former employer's customers
// lost trust". Nothing stripped comments, so that note was chunked,
// embedded and retrievable, and because it was the first line it also
// became the document's title and would have appeared as a citation
// label. A note describing what was hidden is worse than the passage
// it replaced: it leaks the substance in summary and advertises that
// something is missing.
//
// Comments are author metadata, never content for the model, so they
// are removed before the title is derived, before the content hash is
// taken, and before chunking. Code fences are respected because a
// README explaining HTML or templating legitimately shows `<!--` as
// an example, and READMEs are ingested.
//
// An unterminated comment strips to the end of the document. That can
// empty it, which returns ErrEmpty and fails the ingest loudly. A
// loud failure on a malformed document is the right trade against
// silently publishing whatever followed the `<!--`.
func stripHTMLComments(text string) string {
	var out strings.Builder
	out.Grow(len(text))
	inFence, inComment := false, false

	for line := range strings.SplitSeq(text, "\n") {
		rest := line
		if inComment {
			_, after, found := strings.Cut(rest, "-->")
			if !found {
				continue // whole line is inside the comment
			}
			inComment, rest = false, after
		}

		// A fence marker only counts outside a comment, which the
		// branch above has already settled for this line.
		trimmed := strings.TrimSpace(rest)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			out.WriteString(line)
			out.WriteString("\n")
			continue
		}
		if inFence {
			out.WriteString(line)
			out.WriteString("\n")
			continue
		}

		for {
			before, after, found := strings.Cut(rest, "<!--")
			if !found {
				out.WriteString(rest)
				break
			}
			out.WriteString(before)
			_, tail, closed := strings.Cut(after, "-->")
			if !closed {
				inComment = true
				break
			}
			rest = tail
		}
		out.WriteString("\n")
	}

	return strings.TrimSpace(out.String())
}

// firstNonEmptyLine returns the first non-blank line of text, with
// leading markdown heading markers stripped. Used as a fallback
// title.
func firstNonEmptyLine(text string) string {
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return strings.TrimSpace(strings.TrimLeft(line, "# "))
	}
	return ""
}
