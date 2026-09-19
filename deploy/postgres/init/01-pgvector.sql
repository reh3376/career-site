-- pgvector is bundled in the pgvector/pgvector:pg16 image. Enable it so the
-- corpus_chunks.embedding column (FSD §8.4) works out of the box in dev; the
-- migrations that create that table land in Phase 4.
CREATE EXTENSION IF NOT EXISTS vector;
