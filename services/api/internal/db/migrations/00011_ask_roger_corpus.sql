-- +goose Up
-- +goose StatementBegin

-- Ask Roger Phase 4 groundwork — corpus storage + vector index.
-- The reader for this schema is future retrieval code (JD scoring
-- in item #10, the Ask Roger assistant in Phase 4 proper). Nothing
-- reads it yet; this migration only creates the tables + index so
-- follow-up PRs can plug ingestion + retrieval in behind the
-- storage layer.
--
-- Embedding dimension is fixed at 768, matching Ollama's
-- `nomic-embed-text` model. Chosen as the default because it's
-- small (fast retrieval), runs on any box, and is the reference
-- open-source embed model. If a bigger model is warranted later, a
-- dual-index migration adds `embedding_v2 vector(N)` alongside and
-- swings reads over once the reindex completes — the fixed dim
-- here does not paint us into a corner.

-- pgvector is already available in the pgvector/pgvector:pg16 image;
-- CREATE EXTENSION just registers it in the current DB.
CREATE EXTENSION IF NOT EXISTS vector;

-- One row per source document (a résumé, an article, a career note,
-- an ADR — anything ingested into the assistant's corpus). Content
-- staging lives elsewhere; this table is just the DB-side index.
CREATE TABLE corpus_documents (
  id            bigserial PRIMARY KEY,
  -- Where the text came from: 'article' | 'resume' | 'career_note'
  -- | 'adr' | 'other'. Kept as text (not enum) so a new source kind
  -- doesn't need a schema migration.
  source_kind   text        NOT NULL,
  -- Filesystem path or object-store key of the source, relative to
  -- the ingest root. Unique per source_kind so re-ingest replaces.
  source_path   text        NOT NULL,
  -- Human-readable title extracted at ingest time.
  title         text        NOT NULL,
  -- Content MIME (`text/markdown`, `text/plain`, `application/pdf`
  -- — pre-extraction we may store the raw bytes elsewhere, but the
  -- chunks always hold text).
  mime          text        NOT NULL DEFAULT 'text/markdown',
  -- Arbitrary source metadata (author, date, series, tags…).
  meta          jsonb       NOT NULL DEFAULT '{}'::jsonb,
  -- SHA-256 of the source text at ingest, used to skip re-embed
  -- when nothing has changed.
  content_hash  bytea,
  ingested_at   timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (source_kind, source_path)
);

COMMENT ON TABLE corpus_documents IS
  'One row per ingested source in the Ask Roger corpus.';

-- One row per chunk of a document — the retrieval unit. Chunks are
-- roughly paragraph-sized (targeting ~600 tokens each), overlapping
-- ~15% so a hit near a boundary still returns useful context.
CREATE TABLE corpus_chunks (
  id            bigserial PRIMARY KEY,
  document_id   bigint       NOT NULL REFERENCES corpus_documents(id) ON DELETE CASCADE,
  -- Position within the document, 0-based. Together with document_id
  -- this uniquely identifies a chunk across re-ingests of the same
  -- source.
  chunk_index   integer      NOT NULL,
  -- The raw chunk text. Retrieval returns this verbatim to the
  -- caller (LLM prompt, JD scoring, admin viewer).
  text          text         NOT NULL,
  -- Vector representation (nomic-embed-text = 768 dims). Nullable
  -- so an ingest job can insert-first, embed-later.
  embedding     vector(768),
  -- Approximate token count used for prompt budgeting.
  token_count   integer,
  -- Chunk-level metadata (heading path, page number, section id).
  meta          jsonb        NOT NULL DEFAULT '{}'::jsonb,
  created_at    timestamptz  NOT NULL DEFAULT now(),
  UNIQUE (document_id, chunk_index)
);

COMMENT ON TABLE corpus_chunks IS
  'Retrieval-unit rows for the Ask Roger corpus. 1..N per document.';

-- Approximate-nearest-neighbour index on embedding for retrieval.
-- HNSW is faster + higher recall than IVFFlat at the sizes we
-- expect (a few thousand chunks); m/ef_construction defaults are
-- fine for this scale and can be re-tuned later without a schema
-- change. Cosine distance operator (<=>) is what retrieval will use.
CREATE INDEX corpus_chunks_embedding_hnsw
  ON corpus_chunks USING hnsw (embedding vector_cosine_ops);

-- Point-lookup + document scan.
CREATE INDEX corpus_chunks_document_idx
  ON corpus_chunks (document_id, chunk_index);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS corpus_chunks;
DROP TABLE IF EXISTS corpus_documents;
-- Leave the vector extension in place; other future migrations may
-- rely on it.

-- +goose StatementEnd
