-- +goose Up
-- +goose StatementBegin

-- Ask Roger's conversation store (FR-CHAT-08).
--
-- The contract in proto/career/v1/chat.proto has existed since the
-- schema was drafted and nothing has ever backed it. This is that
-- backing, and nothing more: no retrieval, no model, no persona. Those
-- are separable and this is the part that does not depend on where
-- inference ends up running, which is still open.

CREATE TABLE IF NOT EXISTS chat_conversations (
  id                 bigserial PRIMARY KEY,
  tenant_id          bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  -- Conversations belong to a member. ON DELETE CASCADE because a
  -- member who deletes their account should not leave their questions
  -- behind: FR-CHAT-08 makes deletion the member's right, and a row
  -- surviving the account is the opposite of that.
  user_id            bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  -- Derived from the first question unless renamed. Not the question
  -- itself: a title is shown in a list and a question can be a
  -- paragraph.
  title              text        NOT NULL DEFAULT '',
  -- The persona in force when the conversation opened. Recorded per
  -- conversation and again per message, because the owner can edit the
  -- persona mid-conversation and an answer has to be explicable by the
  -- persona that produced it, not the current one.
  persona_version    text        NOT NULL DEFAULT '',
  -- The content item the panel was opened from, when any. Used for
  -- suggestions (FR-CHAT-09).
  context_content_id text        NOT NULL DEFAULT '',
  started_at         timestamptz NOT NULL DEFAULT now(),
  last_message_at    timestamptz NOT NULL DEFAULT now(),
  -- Soft delete. The member sees it gone immediately; the row is
  -- removed by a later job, which is what FR-CHAT-08 asks for and what
  -- makes an accidental delete recoverable inside that window.
  deleted_at         timestamptz
);

COMMENT ON TABLE chat_conversations IS
  'One Ask Roger conversation. Soft-deleted on the member''s request (FR-CHAT-08); a job hard-deletes later.';

CREATE INDEX IF NOT EXISTS idx_chat_conversations_user
    ON chat_conversations (tenant_id, user_id, last_message_at DESC)
 WHERE deleted_at IS NULL;

-- Finding what a sweeper should remove without scanning live rows.
CREATE INDEX IF NOT EXISTS idx_chat_conversations_deleted
    ON chat_conversations (deleted_at)
 WHERE deleted_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS chat_messages (
  id              bigserial PRIMARY KEY,
  tenant_id       bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  conversation_id bigint      NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
  -- user | assistant | owner. The third is the owner replying to an
  -- escalation (FR-CHAT-10), which is a real message in the thread
  -- rather than an email that happens elsewhere.
  role            text        NOT NULL,
  text            text        NOT NULL DEFAULT '',
  created_at      timestamptz NOT NULL DEFAULT now(),

  -- Assistant-side only, all defaulting to the quiet case.
  --
  -- These are four different things that all look like "a short
  -- answer" to a reader, and collapsing them would make the surface
  -- unable to explain itself: declined as out of scope, nothing found
  -- to answer from, answered from the Q&A bank while the model was
  -- unavailable, and matched directly from the Q&A bank.
  out_of_scope    boolean     NOT NULL DEFAULT false,
  no_support      boolean     NOT NULL DEFAULT false,
  degraded        boolean     NOT NULL DEFAULT false,
  qa_match        boolean     NOT NULL DEFAULT false,
  persona_version text        NOT NULL DEFAULT '',

  -- The member's rating (FR-CHAT-14). NULL is unrated, which is
  -- different from neutral and has to stay distinguishable.
  rating_up       boolean,
  rating_comment  text        NOT NULL DEFAULT '',
  rated_at        timestamptz,

  -- Joins this message to the pipeline run that produced it, the same
  -- way jd_runs does for a review, so a bad answer can be traced to the
  -- model, prompts and corpus behind it rather than guessed at.
  run_id          uuid,

  CONSTRAINT chat_messages_role_known CHECK (role IN ('user', 'assistant', 'owner'))
);

COMMENT ON TABLE chat_messages IS
  'One message. Assistant rows carry why the answer looks as it does (out_of_scope, no_support, degraded, qa_match) because four different situations otherwise render identically.';

CREATE INDEX IF NOT EXISTS idx_chat_messages_conversation
    ON chat_messages (conversation_id, id);

-- Citations are rows rather than jsonb on the message, unlike the
-- judgments on a JD submission. The difference is what gets asked of
-- them: a judgment is only ever read back with its submission, while
-- "which corpus documents does the assistant actually cite, and which
-- has it never once used" is a question about the corpus, and that is a
-- join rather than a scan of every message's json.
CREATE TABLE IF NOT EXISTS chat_citations (
  id         bigserial PRIMARY KEY,
  message_id bigint  NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
  -- The chunk actually shown to the model. Kept as a plain id rather
  -- than a foreign key: a re-index replaces chunks, and a citation
  -- recording what was cited at the time must not disappear because the
  -- corpus moved underneath it.
  chunk_id   bigint  NOT NULL,
  -- Denormalised for the same reason, and because a private-corpus
  -- source shows a title with no link at all (FR-CHAT-04).
  content_id text    NOT NULL DEFAULT '',
  title      text    NOT NULL DEFAULT '',
  path       text    NOT NULL DEFAULT '',
  heading    text    NOT NULL DEFAULT '',
  rank       integer NOT NULL DEFAULT 0
);

COMMENT ON TABLE chat_citations IS
  'What an answer cited, denormalised so a re-index cannot rewrite the history of an answer. An empty path means a private-corpus source, which is shown as a title and never linked (FR-CHAT-04).';

CREATE INDEX IF NOT EXISTS idx_chat_citations_message ON chat_citations (message_id, rank);
CREATE INDEX IF NOT EXISTS idx_chat_citations_chunk   ON chat_citations (chunk_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS chat_citations;
DROP TABLE IF EXISTS chat_messages;
DROP TABLE IF EXISTS chat_conversations;
-- +goose StatementEnd
