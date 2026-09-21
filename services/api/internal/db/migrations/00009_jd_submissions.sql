-- +goose Up
-- +goose StatementBegin

-- Public JD-upload plumbing. A visitor pastes / uploads a job
-- description on /jd-upload; the API stores the raw text here, later
-- extracts text from PDFs (sidecar), embeds + retrieves against
-- Roger's private career corpus, scores fit, and — if the score
-- clears the threshold — generates a tailored 2-page resume from
-- the master corpus. Everything after "stores the raw text" ships in
-- follow-up PRs; this migration is the persistence backbone every
-- one of them shares.
--
-- Non-goals: contact-form-style triage lives in support_messages;
-- this table is specifically for JD → résumé requests.

CREATE TABLE jd_submissions (
  id                    bigserial PRIMARY KEY,
  -- Hashed identifiers for rate-limit + audit without storing the raw
  -- IP / UA. SHA-256(salt || value) computed in Go; salt is the
  -- decision-token secret so a leaked DB dump can't reverse them.
  ip_hash               bytea,
  ua_hash               bytea,
  -- Where the text came from: 'paste' (textarea), 'pdf' (uploaded
  -- and extracted server-side), 'text_upload' (uploaded plain text).
  -- Kept as text (not enum) so a new source kind doesn't need a
  -- migration.
  source_kind           text        NOT NULL,
  -- The full JD as text (post-extraction for PDFs). Capped by the
  -- RPC at ~50 000 characters — well past a normal JD, small enough
  -- that we can store the raw body inline instead of an object-store
  -- key.
  jd_text               text        NOT NULL,
  -- SHA-256 of the normalised (whitespace-collapsed, lowercased)
  -- jd_text. Deduping key for retries and per-JD rate-limiting.
  jd_hash               bytea       NOT NULL,
  -- First ~400 chars of jd_text kept as a separate column so the
  -- admin list can render a preview without SELECT-ing the full body.
  text_head             text        NOT NULL,
  -- Optional context the submitter can provide on the form to help
  -- with tailoring / triage. Never required.
  role_hint             text,
  employer_hint         text,
  contact_email         text,
  -- Lifecycle. See handler for the state machine.
  --   received          — just stored; nothing has looked at it yet
  --   scoring           — retrieval + scoring in flight (async job)
  --   below_threshold   — score < 0.65; caller gets the polite fallback
  --   generating        — score ≥ 0.65; resume generation in flight
  --   ready             — resume built; generated_resume_url set
  --   failed            — anything above raised an error; see error col
  status                text        NOT NULL DEFAULT 'received',
  match_score           double precision,     -- 0.0 .. 1.0 when known
  generated_resume_url  text,                 -- object-store URL or filesystem path
  error                 text,
  created_at            timestamptz NOT NULL DEFAULT now(),
  completed_at          timestamptz
);

-- Admin listings + per-JD dedup lookups.
CREATE INDEX idx_jd_submissions_created ON jd_submissions (created_at DESC);
CREATE INDEX idx_jd_submissions_hash    ON jd_submissions (jd_hash);
CREATE INDEX idx_jd_submissions_status  ON jd_submissions (status);

COMMENT ON TABLE  jd_submissions IS 'Public JD-upload requests → tailored-resume responses.';
COMMENT ON COLUMN jd_submissions.jd_hash IS 'SHA-256 of normalised text; used for retry-dedup and per-JD rate-limit.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS jd_submissions;

-- +goose StatementEnd
