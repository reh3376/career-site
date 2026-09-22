-- +goose Up
-- +goose StatementBegin

-- Every decision the JD reviewer makes, logged with what it was made
-- from, so the owner can review each one and record his own verdict.
-- The reviewed rows are the human-in-the-loop training set for the
-- Ask Roger adapter. See docs/decision-log.md.
CREATE TABLE decision_log (
  id                bigserial PRIMARY KEY,
  -- jd_requirement_verdict | jd_gate | (later) resume_item, chat_answer
  kind              text        NOT NULL,
  -- What the decision belongs to (jd_submission + its id) and, within
  -- it, which item (requirement id); '' when the row is per-ref.
  ref_kind          text        NOT NULL,
  ref_id            bigint      NOT NULL,
  key               text        NOT NULL DEFAULT '',
  -- Who decided: the model name as reported by the gateway, or 'code'.
  model             text        NOT NULL,
  prompt_id         text        NOT NULL DEFAULT '',
  prompt_version    integer     NOT NULL DEFAULT 0,
  num_ctx           integer     NOT NULL DEFAULT 0,
  -- What it was decided from, and what was decided (shape per kind).
  input             jsonb       NOT NULL,
  output            jsonb       NOT NULL,
  -- The exact exchange, for replay and for training. Empty for code.
  prompt_text       text        NOT NULL DEFAULT '',
  response_text     text        NOT NULL DEFAULT '',
  prompt_tokens     integer     NOT NULL DEFAULT 0,
  completion_tokens integer     NOT NULL DEFAULT 0,
  latency_ms        bigint      NOT NULL DEFAULT 0,
  created_at        timestamptz NOT NULL DEFAULT now(),
  -- The owner's review. Same vocabulary as output.verdict so agreement
  -- is a direct comparison. NULL reviewed_at = not yet reviewed.
  human_verdict     text,
  human_note        text,
  reviewed_by       bigint REFERENCES users(id) ON DELETE SET NULL,
  reviewed_at       timestamptz
);

CREATE INDEX idx_decision_log_ref ON decision_log (ref_kind, ref_id, id);
CREATE INDEX idx_decision_log_unreviewed ON decision_log (kind, id DESC) WHERE reviewed_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS decision_log;

-- +goose StatementEnd
