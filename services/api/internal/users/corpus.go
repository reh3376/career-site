package users

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/reh3376/career-site/services/api/internal/corpusscope"
)

// CorpusDocument mirrors corpus_documents. content_hash and meta are
// exposed so ingest jobs can skip re-embedding unchanged sources.
type CorpusDocument struct {
	ID          int64
	SourceKind  string
	SourcePath  string
	Title       string
	MIME        string
	Visibility  string // VisibilityPublic | VisibilityCorpusOnly
	Meta        []byte // raw jsonb; caller owns parsing
	ContentHash []byte
	IngestedAt  time.Time
	UpdatedAt   time.Time
}

// Visibility values for corpus_documents.visibility (CHECK-constrained).
const (
	VisibilityPublic     = "public"
	VisibilityCorpusOnly = "corpus_only"
)

// ValidVisibility reports whether v is one of the accepted values.
func ValidVisibility(v string) bool {
	return v == VisibilityPublic || v == VisibilityCorpusOnly
}

// CorpusChunk mirrors corpus_chunks. Embedding is nil when the
// chunk was inserted but not yet embedded (insert-first, embed-later
// ingest pattern).
type CorpusChunk struct {
	ID         int64
	DocumentID int64
	ChunkIndex int32
	Text       string
	Embedding  []float32 // nil = not embedded yet
	TokenCount int32
	Meta       []byte
	CreatedAt  time.Time
}

// CorpusHit is a retrieval result: one chunk with the source's title
// + a cosine similarity score in [0, 1] (higher = closer).
type CorpusHit struct {
	Chunk      CorpusChunk
	Title      string
	SourceKind string
	SourcePath string
	Visibility string
	Similarity float32
}

// GetCorpusDocumentByPath returns the document at
// (source_kind, source_path), or nil (no error) when the row does
// not exist. Callers use this to short-circuit re-ingest when the
// content hash hasn't changed.
func (r *Repo) GetCorpusDocumentByPath(
	ctx context.Context, sourceKind, sourcePath string,
) (*CorpusDocument, error) {
	const q = `
    SELECT id, source_kind, source_path, title, mime, visibility,
           COALESCE(meta::text, '{}'), content_hash, ingested_at, updated_at
    FROM corpus_documents
    WHERE source_kind = $1 AND source_path = $2
  `
	d := &CorpusDocument{}
	var metaText string
	err := r.pool.QueryRow(ctx, q, sourceKind, sourcePath).Scan(
		&d.ID, &d.SourceKind, &d.SourcePath, &d.Title, &d.MIME, &d.Visibility,
		&metaText, &d.ContentHash, &d.IngestedAt, &d.UpdatedAt,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get corpus doc by path: %w", err)
	}
	d.Meta = []byte(metaText)
	return d, nil
}

// UpsertCorpusDocument creates or replaces a document by (source_kind,
// source_path). Returns the stored row.
func (r *Repo) UpsertCorpusDocument(
	ctx context.Context, in CorpusDocument,
) (*CorpusDocument, error) {
	if in.MIME == "" {
		in.MIME = "text/markdown"
	}
	if in.Meta == nil {
		in.Meta = []byte("{}")
	}
	if in.Visibility == "" {
		in.Visibility = VisibilityPublic
	}
	if !ValidVisibility(in.Visibility) {
		return nil, fmt.Errorf("invalid visibility %q", in.Visibility)
	}
	const q = `
    INSERT INTO corpus_documents
      (source_kind, source_path, title, mime, visibility, meta, content_hash)
    VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7)
    ON CONFLICT (source_kind, source_path) DO UPDATE
      SET title        = EXCLUDED.title,
          mime         = EXCLUDED.mime,
          visibility   = EXCLUDED.visibility,
          meta         = EXCLUDED.meta,
          content_hash = EXCLUDED.content_hash,
          updated_at   = now()
    RETURNING id, ingested_at, updated_at
  `
	out := in
	err := r.pool.QueryRow(ctx, q,
		in.SourceKind, in.SourcePath, in.Title, in.MIME, in.Visibility, string(in.Meta), in.ContentHash,
	).Scan(&out.ID, &out.IngestedAt, &out.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert corpus doc: %w", err)
	}
	return &out, nil
}

// UpsertCorpusChunk stores one chunk. Embedding may be nil; a later
// pass fills it via SetCorpusChunkEmbedding. Idempotent on
// (document_id, chunk_index).
func (r *Repo) UpsertCorpusChunk(
	ctx context.Context, in CorpusChunk,
) (*CorpusChunk, error) {
	if in.Meta == nil {
		in.Meta = []byte("{}")
	}
	const q = `
    INSERT INTO corpus_chunks
      (document_id, chunk_index, text, embedding, token_count, meta)
    VALUES ($1, $2, $3, $4, $5, $6::jsonb)
    ON CONFLICT (document_id, chunk_index) DO UPDATE
      SET text        = EXCLUDED.text,
          embedding   = COALESCE(EXCLUDED.embedding, corpus_chunks.embedding),
          token_count = EXCLUDED.token_count,
          meta        = EXCLUDED.meta
    RETURNING id, created_at
  `
	var embArg any
	if in.Embedding != nil {
		embArg = vectorLiteral(in.Embedding)
	}
	out := in
	err := r.pool.QueryRow(ctx, q,
		in.DocumentID, in.ChunkIndex, in.Text, embArg, in.TokenCount, string(in.Meta),
	).Scan(&out.ID, &out.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert corpus chunk: %w", err)
	}
	return &out, nil
}

// SetCorpusChunkEmbedding fills the embedding for one chunk and
// records which embedder produced it. Used by the embed pass in an
// insert-first, embed-later ingest and by the embed sweep.
func (r *Repo) SetCorpusChunkEmbedding(
	ctx context.Context, chunkID int64, embedding []float32, embedderModel string,
) error {
	if len(embedding) == 0 {
		return fmt.Errorf("empty embedding")
	}
	const q = `
    UPDATE corpus_chunks
    SET embedding = $2, embedder_model = NULLIF($3, ''), embedded_at = now()
    WHERE id = $1
  `
	tag, err := r.pool.Exec(ctx, q, chunkID, vectorLiteral(embedding), embedderModel)
	if err != nil {
		return fmt.Errorf("set chunk embedding: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListChunksNeedingEmbedding returns chunks whose vector is missing or
// was produced by a different embedder than `currentModel`, oldest
// first, capped at limit. Text is included so the caller can embed
// without a second query.
func (r *Repo) ListChunksNeedingEmbedding(
	ctx context.Context, currentModel string, limit int,
) ([]CorpusChunk, error) {
	if limit <= 0 || limit > 500 {
		limit = 64
	}
	const q = `
    SELECT id, document_id, chunk_index, text, COALESCE(token_count, 0), created_at
    FROM corpus_chunks
    WHERE embedding IS NULL OR embedder_model IS DISTINCT FROM $1
    ORDER BY id
    LIMIT $2
  `
	rows, err := r.pool.Query(ctx, q, currentModel, limit)
	if err != nil {
		return nil, fmt.Errorf("list chunks needing embedding: %w", err)
	}
	defer rows.Close()
	var out []CorpusChunk
	for rows.Next() {
		var c CorpusChunk
		if err := rows.Scan(&c.ID, &c.DocumentID, &c.ChunkIndex, &c.Text, &c.TokenCount, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan chunk: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CountChunksNeedingEmbedding is the sweep's "remaining" number.
func (r *Repo) CountChunksNeedingEmbedding(ctx context.Context, currentModel string) (int32, error) {
	const q = `
    SELECT COUNT(*)::int FROM corpus_chunks
    WHERE embedding IS NULL OR embedder_model IS DISTINCT FROM $1
  `
	var n int32
	if err := r.pool.QueryRow(ctx, q, currentModel).Scan(&n); err != nil {
		return 0, fmt.Errorf("count chunks needing embedding: %w", err)
	}
	return n, nil
}

// EmbedderCount is one row of the per-embedder breakdown shown on
// /admin/corpus. Model is "" for chunks that have no embedding.
type EmbedderCount struct {
	Model string
	Count int32
}

// CountChunksByEmbedder groups every chunk by the embedder that
// produced its vector so the admin can see, after a provider flip,
// how much of the corpus is still on the old one.
func (r *Repo) CountChunksByEmbedder(ctx context.Context) ([]EmbedderCount, error) {
	const q = `
    SELECT CASE WHEN embedding IS NULL THEN '' ELSE COALESCE(embedder_model, 'untracked') END AS model,
           COUNT(*)::int
    FROM corpus_chunks
    GROUP BY 1
    ORDER BY 2 DESC, 1
  `
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("count chunks by embedder: %w", err)
	}
	defer rows.Close()
	var out []EmbedderCount
	for rows.Next() {
		var c EmbedderCount
		if err := rows.Scan(&c.Model, &c.Count); err != nil {
			return nil, fmt.Errorf("scan embedder count: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SearchCorpus returns the top-k chunks nearest to the query
// embedding by cosine distance. Similarity in the result is
// (1 - distance) so callers see a "higher = closer" number they can
// threshold against directly (JD_MATCH_THRESHOLD gates the JD flow).
//
// The HNSW index on corpus_chunks.embedding makes this fast even
// as the corpus grows; the query below is what pgvector's docs
// call the "canonical KNN" pattern.
func (r *Repo) SearchCorpus(
	ctx context.Context, embedding []float32, topK int,
) ([]CorpusHit, error) {
	return r.searchCorpus(ctx, embedding, "", topK)
}

// searchCorpus is the shared body; sourceKind "" means any kind (a
// per-kind search was tried as a résumé anchor for the JD judge and
// dropped: cosine similarity is unreliable for tenure and credential
// facts, see docs/llm-tuning-log.md 2026-09-22).
func (r *Repo) searchCorpus(
	ctx context.Context, embedding []float32, sourceKind string, topK int,
) ([]CorpusHit, error) {
	if len(embedding) == 0 {
		return nil, fmt.Errorf("empty embedding")
	}
	if topK <= 0 || topK > 200 {
		topK = 20
	}
	// $3 = '' means any kind; the index is still used for the ordering.
	//
	// $4 is the visibility gate. Most of the corpus is private, and
	// retrieved text reaches the judge's context and the résumé
	// generator, so a submitted posting must not be able to steer
	// similarity search across client and NDA material. The scope rides
	// the context and defaults to public, so a caller that forgets to
	// widen it retrieves too little rather than too much. See
	// internal/corpusscope.
	const q = `
    SELECT c.id, c.document_id, c.chunk_index, c.text,
           COALESCE(c.token_count, 0),
           d.title, d.source_kind, d.source_path, d.visibility,
           1 - (c.embedding <=> $1) AS similarity
    FROM corpus_chunks c
    JOIN corpus_documents d ON d.id = c.document_id
    WHERE c.embedding IS NOT NULL
      AND ($3 = '' OR d.source_kind = $3)
      AND ($4 = true OR d.visibility = 'public')
    ORDER BY c.embedding <=> $1
    LIMIT $2
  `
	rows, err := r.pool.Query(ctx, q,
		vectorLiteral(embedding), topK, sourceKind, corpusscope.AllowsPrivate(ctx))
	if err != nil {
		return nil, fmt.Errorf("search corpus: %w", err)
	}
	defer rows.Close()

	var out []CorpusHit
	for rows.Next() {
		var h CorpusHit
		if err := rows.Scan(
			&h.Chunk.ID, &h.Chunk.DocumentID, &h.Chunk.ChunkIndex, &h.Chunk.Text,
			&h.Chunk.TokenCount,
			&h.Title, &h.SourceKind, &h.SourcePath, &h.Visibility,
			&h.Similarity,
		); err != nil {
			return nil, fmt.Errorf("scan corpus hit: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// ListChunksByKind returns every chunk of documents with the given
// source_kind in document order, as hits with similarity 0. Used to
// hand the résumé writer the master résumé regardless of what the JD
// retrieval surfaced, so roles and dates always have a source.
func (r *Repo) ListChunksByKind(ctx context.Context, sourceKind string, limit int) ([]CorpusHit, error) {
	if limit <= 0 || limit > 200 {
		limit = 40
	}
	const q = `
    SELECT c.id, c.document_id, c.chunk_index, c.text, COALESCE(c.token_count, 0),
           d.title, d.source_kind, d.source_path, d.visibility
    FROM corpus_chunks c
    JOIN corpus_documents d ON d.id = c.document_id
    WHERE d.source_kind = $1
    ORDER BY d.id, c.chunk_index
    LIMIT $2
  `
	rows, err := r.pool.Query(ctx, q, sourceKind, limit)
	if err != nil {
		return nil, fmt.Errorf("list chunks by kind: %w", err)
	}
	defer rows.Close()
	var out []CorpusHit
	for rows.Next() {
		var h CorpusHit
		if err := rows.Scan(
			&h.Chunk.ID, &h.Chunk.DocumentID, &h.Chunk.ChunkIndex, &h.Chunk.Text,
			&h.Chunk.TokenCount, &h.Title, &h.SourceKind, &h.SourcePath, &h.Visibility,
		); err != nil {
			return nil, fmt.Errorf("scan chunk by kind: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// GetCorpusChunks returns the chunks with the given ids (any order),
// joined to their document for title / kind / visibility.
func (r *Repo) GetCorpusChunks(ctx context.Context, ids []int64) ([]CorpusHit, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	const q = `
    SELECT c.id, c.document_id, c.chunk_index, c.text, COALESCE(c.token_count, 0),
           d.title, d.source_kind, d.source_path, d.visibility
    FROM corpus_chunks c
    JOIN corpus_documents d ON d.id = c.document_id
    WHERE c.id = ANY($1)
  `
	rows, err := r.pool.Query(ctx, q, ids)
	if err != nil {
		return nil, fmt.Errorf("get corpus chunks: %w", err)
	}
	defer rows.Close()
	var out []CorpusHit
	for rows.Next() {
		var h CorpusHit
		if err := rows.Scan(
			&h.Chunk.ID, &h.Chunk.DocumentID, &h.Chunk.ChunkIndex, &h.Chunk.Text,
			&h.Chunk.TokenCount, &h.Title, &h.SourceKind, &h.SourcePath, &h.Visibility,
		); err != nil {
			return nil, fmt.Errorf("scan corpus chunk: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// HashCorpusContent returns SHA-256 of the source text — used by
// ingest jobs to decide whether a document needs re-embedding.
func HashCorpusContent(text string) []byte {
	sum := sha256.Sum256([]byte(text))
	return sum[:]
}

// CorpusListRow is a document + counts for /admin/corpus. Kept
// separate from CorpusDocument so the list query can compute the
// chunk counts in one round-trip without loading every chunk body.
type CorpusListRow struct {
	Document      CorpusDocument
	ChunkCount    int32
	EmbeddedCount int32
}

// CorpusListSummary is the whole-corpus header the admin page shows.
type CorpusListSummary struct {
	Rows            []CorpusListRow
	TotalDocuments  int32
	TotalChunks     int32
	TotalEmbedded   int32
	TotalPublic     int32
	TotalCorpusOnly int32
}

// ListCorpusDocuments returns every document with chunk + embedded
// counts, newest first. Caps at 500 to keep the payload bounded;
// the admin needs pagination once we ingest more than that (not
// today — the corpus is a handful of docs).
func (r *Repo) ListCorpusDocuments(ctx context.Context) (*CorpusListSummary, error) {
	const q = `
    SELECT d.id, d.source_kind, d.source_path, d.title, d.mime, d.visibility,
           d.ingested_at, d.updated_at,
           COUNT(c.id)::int                                     AS chunk_count,
           COUNT(c.id) FILTER (WHERE c.embedding IS NOT NULL)::int AS embedded_count
    FROM corpus_documents d
    LEFT JOIN corpus_chunks c ON c.document_id = d.id
    GROUP BY d.id
    ORDER BY d.ingested_at DESC
    LIMIT 500
  `
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list corpus documents: %w", err)
	}
	defer rows.Close()

	out := &CorpusListSummary{}
	for rows.Next() {
		var row CorpusListRow
		if err := rows.Scan(
			&row.Document.ID, &row.Document.SourceKind, &row.Document.SourcePath,
			&row.Document.Title, &row.Document.MIME, &row.Document.Visibility,
			&row.Document.IngestedAt, &row.Document.UpdatedAt,
			&row.ChunkCount, &row.EmbeddedCount,
		); err != nil {
			return nil, fmt.Errorf("scan corpus row: %w", err)
		}
		out.Rows = append(out.Rows, row)
		out.TotalChunks += row.ChunkCount
		out.TotalEmbedded += row.EmbeddedCount
		if row.Document.Visibility == VisibilityCorpusOnly {
			out.TotalCorpusOnly++
		} else {
			out.TotalPublic++
		}
	}
	out.TotalDocuments = int32(len(out.Rows))
	return out, rows.Err()
}

// vectorLiteral formats a []float32 as pgvector's text literal:
// `[0.1,0.2,0.3]`. pgx's parameter binding accepts this as-is for
// the vector type; no third-party pgvector-go dependency needed.
func vectorLiteral(v []float32) string {
	var b strings.Builder
	b.Grow(len(v) * 8)
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		// %g is exact enough for retrieval; pgvector rounds internally.
		b.WriteString(strconv.FormatFloat(float64(f), 'g', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}
