-- +goose Up
-- +goose StatementBegin

-- Whether the assistant may draw on a document at all (FR-CHAT-18).
--
-- This is a different axis from visibility and the two must not be
-- confused. Visibility decides whether a source can be *quoted, linked
-- or named*: private material may inform an answer and must never be
-- cited (the locked rule in the Phase 4 notes). This decides whether
-- the assistant sees the document in the first place.
--
-- A document can be public and excluded, if it is accurate but not
-- something the owner wants speaking for him. It can be private and
-- included, informing an answer it can never be credited for.
--
-- Defaulting to true, deliberately, and it is worth saying why because
-- the requirement is phrased as an allow-list. The corpus is 36
-- documents, all curated by the owner for his own career site, so
-- "material he is willing to have represent him" is what the corpus
-- already is. Defaulting false would launch the assistant mute: it
-- would retrieve nothing, answer "I have no support for that" to every
-- question, and look broken rather than unconfigured. An
-- over-inclusive assistant bounded by the visibility rules is a
-- smaller failure than a silent one.
ALTER TABLE corpus_documents
  ADD COLUMN IF NOT EXISTS chatbot_include boolean NOT NULL DEFAULT true;

COMMENT ON COLUMN corpus_documents.chatbot_include IS
  'May Ask Roger draw on this document (FR-CHAT-18). Separate from visibility, which decides whether a source may be cited. Public-and-excluded and private-and-included are both meaningful.';

-- Retrieval filters on this and on visibility together, and the corpus
-- is small enough that a partial index on the excluded rows is the
-- cheaper shape: almost everything is included, so indexing the
-- exceptions is what a planner can actually use.
CREATE INDEX IF NOT EXISTS idx_corpus_documents_chat_excluded
    ON corpus_documents (id) WHERE NOT chatbot_include;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_corpus_documents_chat_excluded;
ALTER TABLE corpus_documents DROP COLUMN IF EXISTS chatbot_include;
-- +goose StatementEnd
