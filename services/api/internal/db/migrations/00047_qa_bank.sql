-- +goose Up
-- +goose StatementBegin

-- The Q&A bank (FR-CHAT-17, FR-CHAT-06, FR-CHAT-12).
--
-- Three separate requirements land on one table, and it is worth
-- setting out why, because the bank looks like a cache and is not one.
--
--   FR-CHAT-17  when the model is unavailable, answer from the bank.
--   FR-CHAT-12  when the budget cap is reached, bank-only mode.
--   FR-CHAT-06  compensation, references, employer-confidential matters
--               and personal life are refused *unless* the owner has
--               written an approved statement. The bank is the only
--               place such a statement can exist.
--
-- So an entry is not a remembered answer. It is the owner's own words,
-- written in advance, served verbatim and never paraphrased by a model.
-- That is what lets it answer the questions the assistant is otherwise
-- forbidden to touch.
--
-- On this hardware it is also the fast path, and that is the reason it
-- is being built now rather than later. Generation on the box gives 10
-- to 25 seconds to first token; a bank hit needs one embedding call and
-- a vector comparison, and returns at once. The common question should
-- never reach the model at all.

CREATE TABLE IF NOT EXISTS qa_entries (
  id          bigserial PRIMARY KEY,
  tenant_id   bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  -- The question as the owner would put it. Shown in admin and used as
  -- a suggested question (FR-CHAT-09); matching happens against the
  -- phrasings table, which includes this one.
  question    text        NOT NULL,
  -- The answer, in the owner's voice, served word for word. A model
  -- never rewrites this: the whole value of the bank is that what comes
  -- back is what he wrote.
  answer      text        NOT NULL,
  -- Sources to show beneath the answer (FR-CHAT-04), as
  -- [{"title": "...", "path": "/..."}].
  --
  -- jsonb here, where chat_citations is a table, and the difference is
  -- the same one drawn in 00045: a citation on a generated message is
  -- queried across messages to ask what the corpus is actually used
  -- for, while these are authored by hand, always read with their
  -- entry, and never aggregated.
  sources     jsonb       NOT NULL DEFAULT '[]'::jsonb,
  -- Free tags for admin filtering and for grouping suggestions.
  tags        text[]      NOT NULL DEFAULT '{}',
  -- Marks an entry that deliberately answers something rule 7 of the
  -- persona otherwise refuses: compensation, references, an employer
  -- matter, something personal. Not enforcement, which lives in the
  -- prompt and in the pipeline; this is so the owner can list every
  -- place he has opened one of those doors, and so opening one is a
  -- deliberate act rather than a side effect of adding an entry.
  covers_restricted boolean NOT NULL DEFAULT false,
  -- Enabling is the approval (FR-CHAT-06). A disabled entry is never
  -- served and never matched, which is how a statement gets withdrawn
  -- without losing what it said.
  enabled     boolean     NOT NULL DEFAULT false,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE qa_entries IS
  'Owner-written answers served verbatim. The fast path on a CPU-only box, the degrade path when the model is down (FR-CHAT-17), and the only way an FR-CHAT-06 restricted topic is ever answered.';

COMMENT ON COLUMN qa_entries.enabled IS
  'Enabling is the approval. Disabled entries are never matched or served.';

CREATE INDEX IF NOT EXISTS idx_qa_entries_enabled
    ON qa_entries (tenant_id) WHERE enabled;

-- One entry, many ways of asking it.
--
-- Separate rows rather than an array column because each phrasing
-- carries its own embedding and matching is a nearest-neighbour search
-- over all of them. "What does he know about MQTT", "has he used a
-- unified namespace" and "unified namespace experience?" are three
-- vectors and one answer.
CREATE TABLE IF NOT EXISTS qa_phrasings (
  id        bigserial PRIMARY KEY,
  entry_id  bigint      NOT NULL REFERENCES qa_entries(id) ON DELETE CASCADE,
  text      text        NOT NULL,
  -- nomic-embed-text, 768 dims, the same recipe as corpus_chunks.
  -- Nullable so an entry can be written now and embedded by the job
  -- that follows; an unembedded phrasing simply never matches.
  embedding vector(768),
  -- True for the phrasing generated from qa_entries.question, so a
  -- later edit of the question can update its own row without guessing
  -- which of the phrasings it was.
  canonical boolean     NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE qa_phrasings IS
  'Ways of asking one qa_entries row. Matching is nearest-neighbour over these embeddings; the answer comes from the entry.';

CREATE INDEX IF NOT EXISTS qa_phrasings_embedding_hnsw
    ON qa_phrasings USING hnsw (embedding vector_cosine_ops);

CREATE INDEX IF NOT EXISTS idx_qa_phrasings_entry ON qa_phrasings (entry_id);

-- At most one canonical phrasing per entry.
CREATE UNIQUE INDEX IF NOT EXISTS idx_qa_phrasings_canonical
    ON qa_phrasings (entry_id) WHERE canonical;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS qa_phrasings;
DROP TABLE IF EXISTS qa_entries;
-- +goose StatementEnd
