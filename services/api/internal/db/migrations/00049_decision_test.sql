-- +goose Up
-- +goose StatementBegin

-- The decision test (docs/fsd-decision-test.md).
--
-- A fifteen-minute instrument measuring what happens to a person's
-- accuracy and to their confidence as attention is loaded. Five blocks
-- of six questions; a number memorised in two seconds and held across
-- each block; the load escalating 3 digits, 4 digits, 4 digits +1 per
-- digit, 4 digits +3 per digit, and then back to 3 digits as a control.
--
-- The test is the collection mechanism. **The dataset is the product.**
-- These tables are built to FSD §7b rather than to whatever the UI
-- happens to need, because a badly curated dataset makes every later
-- question an archaeology project and the cost compounds with every
-- participant.
--
-- Four rules run through all of it:
--
--   1. Nothing is ever deleted. Abandoned sessions, repeats, expired
--      answers and failed recalls are columns, never DELETEs. The same
--      reasoning that keeps decision_log as training data: the question
--      nobody has asked yet gets answered by a row somebody nearly
--      deleted.
--
--   2. Provenance on every session. Items will change, timings will
--      change, scoring will change, and nothing in the numbers
--      themselves says so. instrument_version, item_set_version and
--      key_version are what let pilot data and real data share these
--      tables without silently becoming one dataset wearing two names.
--
--   3. Identity is separable from measurement. Names and emails live in
--      their own table, so clearing a participant anonymises every
--      measurement they produced while leaving the measurements intact,
--      exactly as the event stream drops identity at 13 months while
--      the counts survive.
--
--   4. One row per question presented. That is the grain. Block and
--      session figures are derived, never stored as the source of
--      truth, per docs/metrics.md.
--
-- Relationships are named in the comments as the edges they become, per
-- ADR 0030: collection is relational, graph analysis comes later from a
-- clean projection, and the export should be mechanical rather than a
-- redesign.

-- ---------------------------------------------------------------- items

-- The item bank, and the answer key.
--
-- THE KEY LIVES HERE AND NOWHERE ELSE. It is never sent to a browser,
-- in any form, including as a hash the client could test guesses
-- against (FSD §2.5). Grading is server-side: the client posts a choice
-- and receives an acknowledgement, never a verdict. That is what makes
-- "no correctness feedback during the test" a property of the system
-- rather than a UI convention, and it is why confidence stays a usable
-- dependent variable.
--
-- The item set is standardized: every participant sees the same thirty
-- items in the same order. An answer key that escapes contaminates the
-- instrument permanently and only has to escape once, which is also why
-- a database dump now carries the key and must be handled as the secret
-- it is.
--
-- Items are data rather than code so a bad item can be corrected
-- without a deploy, and the correction is recorded as a new version
-- rather than overwriting history.
CREATE TABLE IF NOT EXISTS dt_items (
  id              bigserial   PRIMARY KEY,
  tenant_id       bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  -- Stable human key, e.g. 'A1', 'D4', 'P2'. Survives an export, so a
  -- node in a later graph projection traces back to this row.
  code            text        NOT NULL,
  -- Bumped when the wording or the options change. Sessions record the
  -- item_set_version they ran, so a reworded item does not silently
  -- pool with its predecessor.
  version         int         NOT NULL DEFAULT 1,
  -- arithmetic | base_rate | conjunction | syllogism
  family          text        NOT NULL,
  -- scored | practice. Practice items are shown before anything counts
  -- and never enter the results.
  kind            text        NOT NULL DEFAULT 'scored',
  prompt          text        NOT NULL,
  -- A one-line framing shown above the question, e.g. "Assume both
  -- statements are true." The syllogisms need it because this test
  -- deliberately degrades the working memory that instructions live in;
  -- without it, instruction decay would grow across exactly the blocks
  -- where the load effect is measured and would be indistinguishable
  -- from it.
  reminder        text        NOT NULL DEFAULT '',
  -- Ordered options as presented. Position is fixed for everyone
  -- because the test is standardized, so the correct option's position
  -- is varied deliberately across the set at authoring time.
  options         jsonb       NOT NULL,
  -- Index into options. NEVER LEAVES THE SERVER.
  correct_index   int         NOT NULL,
  -- The intended intuitive wrong answer. Separate from "any wrong
  -- answer" because a lure chosen under load is the measurement, and
  -- "other" is a different event entirely.
  lure_index      int         NOT NULL,
  -- Why the lure pulls. For the explanation page and for a reviewer
  -- deciding whether a dead item should be replaced.
  rationale       text        NOT NULL DEFAULT '',
  active          boolean     NOT NULL DEFAULT true,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT dt_items_code_version_uniq UNIQUE (tenant_id, code, version),
  CONSTRAINT dt_items_family_ck CHECK (family IN ('arithmetic','base_rate','conjunction','syllogism')),
  CONSTRAINT dt_items_kind_ck   CHECK (kind IN ('scored','practice')),
  CONSTRAINT dt_items_idx_ck    CHECK (correct_index >= 0 AND lure_index >= 0 AND correct_index <> lure_index)
);

CREATE TRIGGER dt_items_set_updated_at
BEFORE UPDATE ON dt_items
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMENT ON TABLE dt_items IS
  'Decision test item bank including the answer key. The key never leaves the server: grading is server-side and the client receives prompt and options only. Versioned so a reworded item does not pool with its predecessor.';

-- --------------------------------------------------------- participants

-- Who took it, where they chose to say.
--
-- Separate from dt_sessions so that identity can be cleared without
-- touching a single measurement. Every field is optional (FSD §5): the
-- form is shown after the effort warning and posts with the session
-- rather than on keystroke, so somebody who reads the warning and
-- leaves has given us nothing at all.
--
-- The email address is an IDENTIFIER as well as a contact, and the
-- privacy policy says so in those terms rather than burying it as "we
-- may contact you". Its four declared uses: results on request,
-- follow-up, a thank-you, and recognising a repeat participant.
--
-- Edge: a participant TOOK a session.
CREATE TABLE IF NOT EXISTS dt_participants (
  id              bigserial   PRIMARY KEY,
  tenant_id       bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  -- Stable opaque handle used in URLs and exports, so no row is
  -- addressed by a guessable serial.
  public_id       uuid        NOT NULL DEFAULT gen_random_uuid(),
  -- A member, when signed in. ON DELETE SET NULL, not CASCADE: deleting
  -- an account must not delete the measurements it produced, it must
  -- orphan them.
  user_id         bigint      REFERENCES users(id) ON DELETE SET NULL,
  display_name    text        NOT NULL DEFAULT '',
  age_range       text        NOT NULL DEFAULT '',
  education       text        NOT NULL DEFAULT '',
  occupation      text        NOT NULL DEFAULT '',
  email           text        NOT NULL DEFAULT '',
  -- Lowercased email, for recognising a repeat without case games.
  email_key       text        NOT NULL DEFAULT '',
  -- Asked for their results. Nothing is sent to anyone who did not.
  wants_results   boolean     NOT NULL DEFAULT false,
  -- Set when identity is cleared on request or by retention. The row
  -- stays, the person does not.
  anonymized_at   timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER dt_participants_set_updated_at
BEFORE UPDATE ON dt_participants
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE UNIQUE INDEX IF NOT EXISTS idx_dt_participants_public ON dt_participants (public_id);
CREATE INDEX IF NOT EXISTS idx_dt_participants_email ON dt_participants (tenant_id, email_key) WHERE email_key <> '';
CREATE INDEX IF NOT EXISTS idx_dt_participants_user  ON dt_participants (user_id) WHERE user_id IS NOT NULL;

COMMENT ON TABLE dt_participants IS
  'Optional identity for a decision test participant, kept apart from the measurements so clearing it anonymises a session without destroying its data. Email is an identifier as well as a contact and the privacy policy states that.';

-- -------------------------------------------------------------- sessions

-- One sitting of the test, complete or not.
--
-- A session is created when the start button is pressed and is never
-- deleted. An interrupted run cannot be resumed (FSD §5b): resuming
-- would poison the timing control the instrument depends on, since a
-- question answered after a four-minute phone call is not the same
-- question and nothing in the data would say so. So a session that
-- stops is marked abandoned and kept, because where people stop is the
-- only measurement available of the fifteen-minute burden.
--
-- Edges: a participant TOOK this session; this session CONTAINS answers
-- and recalls.
CREATE TABLE IF NOT EXISTS dt_sessions (
  id                  bigserial   PRIMARY KEY,
  tenant_id           bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  public_id           uuid        NOT NULL DEFAULT gen_random_uuid(),
  participant_id      bigint      REFERENCES dt_participants(id) ON DELETE SET NULL,

  -- PROVENANCE. Without these, the first reworded item turns one
  -- dataset into two wearing the same name.
  instrument_version  text        NOT NULL,  -- block structure, counts, phase limits
  item_set_version    text        NOT NULL,  -- which items, in which order
  key_version         text        NOT NULL,  -- the scoring key in force

  -- running | completed | abandoned. Abandoned rows are excluded from
  -- analysis by status, never by deletion.
  status              text        NOT NULL DEFAULT 'running',

  -- Conditions, each of which splits the sample and so must be stored
  -- rather than inferred.
  audio_mode          text        NOT NULL DEFAULT 'sound',   -- sound | visual
  device_class        text        NOT NULL DEFAULT 'unknown', -- desktop | tablet | phone | unknown
  -- The tap-along check: pressing in time with the tick, or the
  -- heartbeat in visual mode. Observed rather than self-reported, and
  -- it doubles as a baseline (below).
  tap_check_passed    boolean     NOT NULL DEFAULT false,
  -- Unloaded reaction time and its variability, from the tap check.
  -- These turn latency from an absolute into a relative measure: a slow
  -- participant under load is only interesting against their own
  -- unloaded speed.
  baseline_rt_ms      int,
  baseline_rt_sd_ms   int,

  -- Repeat detection, in descending reliability: account, email, cookie.
  is_repeat           boolean     NOT NULL DEFAULT false,
  repeat_matched_by   text        NOT NULL DEFAULT '',  -- account | email | cookie | ''
  -- The first-party anonymous id (docs/events/README.md). Detectable,
  -- not preventable: a private window defeats it, which is why repeats
  -- are marked rather than blocked.
  visitor_key         text        NOT NULL DEFAULT '',

  -- Did they convert the number at encoding or carry it and transform
  -- at recall (FSD §3.1). Different strategies produce different load
  -- during the questions, so an uncontrolled variable becomes a
  -- recorded one. Asked in the debrief.
  recall_strategy     text        NOT NULL DEFAULT '',  -- encode | defer | ''

  started_at          timestamptz NOT NULL DEFAULT now(),
  finished_at         timestamptz,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT dt_sessions_status_ck CHECK (status IN ('running','completed','abandoned')),
  CONSTRAINT dt_sessions_audio_ck  CHECK (audio_mode IN ('sound','visual')),
  CONSTRAINT dt_sessions_device_ck CHECK (device_class IN ('desktop','tablet','phone','unknown'))
);

CREATE TRIGGER dt_sessions_set_updated_at
BEFORE UPDATE ON dt_sessions
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE UNIQUE INDEX IF NOT EXISTS idx_dt_sessions_public ON dt_sessions (public_id);
CREATE INDEX IF NOT EXISTS idx_dt_sessions_status  ON dt_sessions (tenant_id, status, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_dt_sessions_version ON dt_sessions (tenant_id, instrument_version, item_set_version);
CREATE INDEX IF NOT EXISTS idx_dt_sessions_visitor ON dt_sessions (visitor_key) WHERE visitor_key <> '';

COMMENT ON TABLE dt_sessions IS
  'One sitting of the decision test. Never deleted: abandoned runs are marked, because where people stop measures the fifteen-minute burden. Carries the provenance that keeps pilot and production data separable in one table.';

-- --------------------------------------------------------------- answers

-- THE GRAIN. One row per question presented.
--
-- Everything else is derived from this: block figures, session figures,
-- the confidence gap. Aggregates are views, never stored as the source
-- of truth (docs/metrics.md).
--
-- Edges: this answer SITS IN a session and IS OF an item.
CREATE TABLE IF NOT EXISTS dt_answers (
  id              bigserial   PRIMARY KEY,
  session_id      bigint      NOT NULL REFERENCES dt_sessions(id) ON DELETE CASCADE,
  item_id         bigint      NOT NULL REFERENCES dt_items(id),
  -- Denormalised deliberately: the item may be reworded later and this
  -- records what was actually in front of this participant.
  item_code       text        NOT NULL,
  item_version    int         NOT NULL,

  -- 1..5. Block 5 returns to block 1's difficulty and is the fatigue
  -- control: without it the ramp is monotonic and load is perfectly
  -- confounded with time-on-task.
  block_no        int         NOT NULL,
  -- d3 | d4 | d4_plus1 | d4_plus3 | d3_control
  block_load      text        NOT NULL,
  position_in_block int       NOT NULL,
  position_overall  int       NOT NULL,

  -- correct | lure | other | expired.
  --
  -- expired is its own outcome, not dropped and not merged into error.
  -- A timeout under load is a result, and it is the one outcome that
  -- cannot be reconstructed afterwards if it is thrown away. Expect
  -- them to cluster in blocks 3 and 4, which is itself a measurement of
  -- where the executive gave out.
  outcome         text        NOT NULL,
  chosen_index    int,        -- null when expired

  -- Shown to answered. Interpreted against the session's baseline_rt_ms
  -- rather than absolutely.
  latency_ms      int,
  -- The participant's own rating, collected on EVERY question. Error
  -- rate alone answers "does load make people wrong"; the research
  -- question is about being wrong *and sure*, which needs confidence on
  -- the same row. Null only when the question expired.
  confidence      int,

  shown_at        timestamptz NOT NULL DEFAULT now(),
  answered_at     timestamptz,

  CONSTRAINT dt_answers_outcome_ck CHECK (outcome IN ('correct','lure','other','expired')),
  CONSTRAINT dt_answers_block_ck   CHECK (block_no BETWEEN 1 AND 5),
  CONSTRAINT dt_answers_load_ck    CHECK (block_load IN ('d3','d4','d4_plus1','d4_plus3','d3_control')),
  CONSTRAINT dt_answers_conf_ck    CHECK (confidence IS NULL OR (confidence BETWEEN 0 AND 100)),
  CONSTRAINT dt_answers_uniq       UNIQUE (session_id, position_overall)
);

CREATE INDEX IF NOT EXISTS idx_dt_answers_session ON dt_answers (session_id, position_overall);
CREATE INDEX IF NOT EXISTS idx_dt_answers_item    ON dt_answers (item_id);
CREATE INDEX IF NOT EXISTS idx_dt_answers_block   ON dt_answers (block_load, outcome);

COMMENT ON TABLE dt_answers IS
  'One row per question presented: the grain of the whole dataset. Outcome includes expired as a first-class result rather than missing data, because a timeout under load is a measurement of where the executive gave out.';

-- --------------------------------------------------------------- recalls

-- One row per block: the number held, and what came back.
--
-- A missed recall is a DATA POINT, NOT A FAILURE (owner, 2026-10-04).
-- The block's six answers are kept and scored normally. Discarding them
-- would have deleted blocks 3 and 4 for exactly the participants the
-- test is about.
--
-- The transformation blocks give recall two distinct failure modes and
-- the distinction is the sharpest signal in the instrument:
--
--   exact         held it and operated on it
--   untransformed digits right, operation not applied (1234 for 2345):
--                 STORAGE HELD, EXECUTIVE FAILED
--   wrong_digits  storage failed
--
-- Scoring recall pass/fail would throw that away.
--
-- Edge: this recall BELONGS TO a session and CLOSES a block.
CREATE TABLE IF NOT EXISTS dt_recalls (
  id                bigserial   PRIMARY KEY,
  session_id        bigint      NOT NULL REFERENCES dt_sessions(id) ON DELETE CASCADE,
  block_no          int         NOT NULL,
  block_load        text        NOT NULL,
  -- What was shown, and what a correct response was after the block's
  -- transformation. Both stored: the expected value is derivable, and
  -- deriving it later means re-implementing the rule in the analysis.
  presented_digits  text        NOT NULL,
  expected_digits   text        NOT NULL,
  response_digits   text        NOT NULL DEFAULT '',
  -- exact | untransformed | wrong_digits | partial | expired
  outcome           text        NOT NULL,
  digits_correct    int         NOT NULL DEFAULT 0,
  latency_ms        int,
  created_at        timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT dt_recalls_outcome_ck CHECK (outcome IN ('exact','untransformed','wrong_digits','partial','expired')),
  CONSTRAINT dt_recalls_block_ck   CHECK (block_no BETWEEN 1 AND 5),
  CONSTRAINT dt_recalls_uniq       UNIQUE (session_id, block_no)
);

CREATE INDEX IF NOT EXISTS idx_dt_recalls_session ON dt_recalls (session_id, block_no);

COMMENT ON TABLE dt_recalls IS
  'One row per block. Outcome distinguishes storage failure from executive failure: correct digits returned untransformed means the number survived and the operation did not, which is the mechanism this test exists to measure.';

-- ------------------------------------------------------- analysis view

-- THE PROJECTION BOUNDARY (ADR 0030).
--
-- Analysis reads this, the CSV export is built on it, and a later graph
-- load exports from it. Nothing outside it needs to know the table
-- layout, which is what keeps that export mechanical rather than a
-- redesign.
--
-- One row per answer with everything a correlation needs already
-- joined: demographics, conditions, baseline, block load, item family,
-- outcome, latency, confidence. The relationships the owner is looking
-- for, education against calibration, baseline attention against
-- susceptibility, load against the confidence gap, are each one GROUP
-- BY from here, or they will not be looked for.
CREATE OR REPLACE VIEW v_dt_answers AS
SELECT
  a.id                      AS answer_id,
  s.public_id               AS session_key,
  p.public_id               AS participant_key,
  s.status,
  s.instrument_version,
  s.item_set_version,
  s.key_version,
  s.audio_mode,
  s.device_class,
  s.tap_check_passed,
  s.baseline_rt_ms,
  s.baseline_rt_sd_ms,
  s.is_repeat,
  s.recall_strategy,
  p.age_range,
  p.education,
  p.occupation,
  (p.email_key <> '')       AS gave_email,
  a.block_no,
  a.block_load,
  a.position_in_block,
  a.position_overall,
  a.item_code,
  a.item_version,
  i.family                  AS item_family,
  a.outcome,
  (a.outcome = 'correct')   AS is_correct,
  (a.outcome = 'lure')      AS is_lure,
  (a.outcome = 'expired')   AS is_expired,
  a.latency_ms,
  -- Latency against the participant's own unloaded speed. The absolute
  -- number conflates how fast a person is with how hard the question
  -- was; this does not.
  CASE WHEN s.baseline_rt_ms > 0 THEN a.latency_ms::numeric / s.baseline_rt_ms END
                            AS latency_vs_baseline,
  a.confidence,
  r.outcome                 AS recall_outcome,
  (r.outcome = 'untransformed') AS recall_executive_failure,
  s.started_at
FROM dt_answers a
JOIN dt_sessions s     ON s.id = a.session_id
JOIN dt_items i        ON i.id = a.item_id
LEFT JOIN dt_participants p ON p.id = s.participant_id
LEFT JOIN dt_recalls r ON r.session_id = a.session_id AND r.block_no = a.block_no;

COMMENT ON VIEW v_dt_answers IS
  'The projection boundary (ADR 0030). One row per answer with demographics, conditions, baseline, load and outcome already joined. Analysis, the CSV export and any later graph load all read this and nothing else.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP VIEW IF EXISTS v_dt_answers;
DROP TABLE IF EXISTS dt_recalls;
DROP TABLE IF EXISTS dt_answers;
DROP TABLE IF EXISTS dt_sessions;
DROP TABLE IF EXISTS dt_participants;
DROP TABLE IF EXISTS dt_items;
-- +goose StatementEnd
