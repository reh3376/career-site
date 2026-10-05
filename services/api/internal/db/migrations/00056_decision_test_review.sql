-- +goose Up
-- +goose StatementBegin

-- Curation: the owner's judgement about whether a run, or one block of
-- a run, is fit to use (roadmap M9, sprint S1).
--
-- The dataset is the deliverable, and a dataset nobody has vouched for
-- is collected rather than curated. Thirty answers from somebody who
-- was interrupted at question twelve are indistinguishable in SQL from
-- thirty answers given under the conditions the instrument assumes.
-- Only the owner knows which is which, because the participants tell
-- him and not the database.
--
-- Four decisions from the owner on 2026-10-05 shape this:
--
--   1. `incomplete` does not exclude anything. It describes the run;
--      exactly one value, `do_not_use`, decides its fate. An incomplete
--      run he judges usable stays `incomplete` and counts.
--   2. Blocks are excludable individually. A phone ringing during block
--      three spoils that block and not the other four, and excluding
--      the whole run would discard twenty-four good answers.
--   3. `hold` exists, for "something is odd and I have not decided".
--      Without it an uncertain run is either left unreviewed, which is
--      indistinguishable from not looked at, or marked with a certainty
--      the reviewer does not have.
--   4. Admin only.
--
-- One vocabulary serves both levels, so the words mean the same thing
-- in both places and "is this row usable" has one definition.

-- ------------------------------------------------------ session review

ALTER TABLE dt_sessions
  ADD COLUMN IF NOT EXISTS review_status text        NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS review_reason text        NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS review_note   text        NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS reviewed_at   timestamptz,
  ADD COLUMN IF NOT EXISTS reviewed_by   bigint      REFERENCES users(id) ON DELETE SET NULL;

-- The vocabulary is enforced here as well as in Go. The Go check is
-- what gives a caller a useful error; this is what makes it true of the
-- table regardless of who writes to it, including a hand-run UPDATE at
-- two in the morning.
ALTER TABLE dt_sessions
  DROP CONSTRAINT IF EXISTS dt_sessions_review_status_ck,
  ADD  CONSTRAINT dt_sessions_review_status_ck
       CHECK (review_status IN ('', 'good', 'incomplete', 'hold', 'do_not_use'));

-- A reason belongs to an exclusion. `do_not_use` on its own cannot
-- answer "how much data are we losing, and to what", and the
-- distinction that matters is between the instrument failing and this
-- run being invalid: the first is a bug to go and repair, the second is
-- nothing to fix and simply costs a data point.
ALTER TABLE dt_sessions
  DROP CONSTRAINT IF EXISTS dt_sessions_review_reason_ck,
  ADD  CONSTRAINT dt_sessions_review_reason_ck
       CHECK (review_reason IN ('', 'instrument_fault', 'participant_reported', 'duplicate', 'other'));

-- Reviewing is work, so finding the next unreviewed run has to be
-- cheap. Partial, because the reviewed rows are the ones nobody is
-- queueing on.
CREATE INDEX IF NOT EXISTS idx_dt_sessions_unreviewed
  ON dt_sessions (tenant_id, started_at DESC)
  WHERE review_status = '' AND NOT is_synthetic;

CREATE INDEX IF NOT EXISTS idx_dt_sessions_review_status
  ON dt_sessions (tenant_id, review_status);

COMMENT ON COLUMN dt_sessions.review_status IS
  'Owner''s judgement: good, incomplete, hold, do_not_use, or empty for not yet reviewed. Only do_not_use excludes the run from the views that make claims about people. Never a reason to delete anything.';

-- -------------------------------------------------------- block review

-- One row per reviewed block. Absent means not reviewed, which is why
-- there is no row created up front: a table of empty judgements reads
-- as a backlog that does not exist.
--
-- A dedicated table rather than columns on dt_recalls. There is already
-- exactly one recall row per session per block and it would have fitted,
-- but dt_recalls holds a measurement and this holds an opinion about
-- it. Putting a curator's note in the table that stores what somebody
-- typed is the kind of overload that reads fine today and confuses
-- whoever next joins the two.
CREATE TABLE IF NOT EXISTS dt_block_reviews (
  id          bigserial   PRIMARY KEY,
  session_id  bigint      NOT NULL REFERENCES dt_sessions(id) ON DELETE CASCADE,
  -- 1 to 5. Not constrained against dt_answers: a block can be judged
  -- unusable precisely because it has no answers in it.
  block_no    int         NOT NULL,
  status      text        NOT NULL,
  reason      text        NOT NULL DEFAULT '',
  note        text        NOT NULL DEFAULT '',
  reviewed_at timestamptz NOT NULL DEFAULT now(),
  reviewed_by bigint      REFERENCES users(id) ON DELETE SET NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT dt_block_reviews_uniq UNIQUE (session_id, block_no),
  CONSTRAINT dt_block_reviews_block_ck  CHECK (block_no BETWEEN 1 AND 5),
  CONSTRAINT dt_block_reviews_status_ck CHECK (status IN ('good', 'incomplete', 'hold', 'do_not_use')),
  CONSTRAINT dt_block_reviews_reason_ck CHECK (reason IN ('', 'instrument_fault', 'participant_reported', 'duplicate', 'other'))
);

CREATE TRIGGER dt_block_reviews_set_updated_at
BEFORE UPDATE ON dt_block_reviews
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMENT ON TABLE dt_block_reviews IS
  'Per-block curation. Same vocabulary as the session so the words mean one thing; only do_not_use excludes. No row means not reviewed, rather than a table of empty judgements.';

-- ------------------------------------------------------------- history

-- Every judgement, appended.
--
-- The current verdict lives on the session row and in dt_block_reviews
-- so every query can filter cheaply. This is the record of how it got
-- there. "Nothing is ever deleted" has applied to the measurements
-- since migration 00049, and there is no reason the judgements about
-- them should be the one mutable thing in the system: "why is this
-- excluded, and since when" is the question somebody will ask about a
-- published finding, and an overwrite cannot answer it.
CREATE TABLE IF NOT EXISTS dt_review_events (
  id          bigserial   PRIMARY KEY,
  session_id  bigint      NOT NULL REFERENCES dt_sessions(id) ON DELETE CASCADE,
  -- Null for a session-level judgement, 1 to 5 for a block.
  block_no    int,
  status      text        NOT NULL,
  reason      text        NOT NULL DEFAULT '',
  note        text        NOT NULL DEFAULT '',
  reviewed_by bigint      REFERENCES users(id) ON DELETE SET NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT dt_review_events_block_ck CHECK (block_no IS NULL OR block_no BETWEEN 1 AND 5)
);

CREATE INDEX IF NOT EXISTS idx_dt_review_events_session
  ON dt_review_events (session_id, created_at DESC);

COMMENT ON TABLE dt_review_events IS
  'Append-only history of curation decisions, session and block. The current verdict is on dt_sessions and dt_block_reviews; this says how it got there and when.';

-- +goose StatementEnd

-- +goose StatementBegin

-- --------------------------------------------- usable, defined once
--
-- Rebuilt in dependency order, deepest last, as 00054 established.
-- CASCADE is deliberately not used: it would also silently take a view
-- added after this file was written.
DROP VIEW IF EXISTS v_dt_items;
DROP VIEW IF EXISTS v_dt_threshold;
DROP VIEW IF EXISTS v_dt_calibration;
DROP VIEW IF EXISTS v_dt_load_curve;
DROP VIEW IF EXISTS v_dt_sessions;
DROP VIEW IF EXISTS v_dt_blocks;
DROP VIEW IF EXISTS v_dt_answers;

CREATE VIEW v_dt_answers AS
SELECT
  a.id                      AS answer_id,
  s.public_id               AS session_key,
  p.public_id               AS participant_key,
  s.status,
  s.is_synthetic,
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
  -- Curation. The status is carried; review_note deliberately is not.
  -- A note is free text written by a curator and can contain anything,
  -- including something about a person, and this view is the export.
  -- The note is read on the admin surface, where identity is already
  -- visible by design.
  s.review_status           AS session_review_status,
  coalesce(br.status, '')   AS block_review_status,
  -- THE ONE DEFINITION OF USABLE. Every view that makes a claim about
  -- people filters on this and nothing else, so there is one place to
  -- read what the word means and one place to change it.
  (NOT s.is_synthetic
     AND s.review_status <> 'do_not_use'
     AND coalesce(br.status, '') <> 'do_not_use')
                            AS usable,
  p.age_range,
  p.education,
  p.occupation,
  (p.email_key <> '')       AS gave_email,
  a.block_no,
  a.block_load,
  dt_load_rank(a.block_load) AS load_rank,
  a.position_in_block,
  a.position_overall,
  a.item_code,
  a.item_version,
  i.family                  AS item_family,
  a.outcome,
  (a.outcome = 'correct')   AS is_correct,
  (a.outcome = 'lure')      AS is_lure,
  (a.outcome = 'expired')   AS is_expired,
  (a.outcome NOT IN ('correct', 'expired')
     AND a.confidence >= dt_confident_threshold())
                            AS confidently_wrong,
  (a.outcome = 'lure' AND a.confidence >= dt_confident_threshold())
                            AS confidently_lured,
  a.latency_ms,
  CASE WHEN s.baseline_rt_ms > 0 THEN a.latency_ms::numeric / s.baseline_rt_ms END
                            AS latency_vs_baseline,
  a.confidence,
  CASE WHEN a.confidence IS NULL THEN NULL
       ELSE round(power(a.confidence / 100.0
                        - CASE WHEN a.outcome = 'correct' THEN 1 ELSE 0 END, 2), 4)
  END                       AS brier,
  r.outcome                 AS recall_outcome,
  (r.outcome = 'untransformed') AS recall_executive_failure,
  r.digits_held,
  CASE
    WHEN r.outcome IS NULL OR r.outcome = 'expired' THEN NULL
    WHEN length(r.presented_digits) = 0 THEN NULL
    ELSE round(1 - (r.digits_held::numeric / length(r.presented_digits)), 2)
  END                       AS memory_failure,
  s.started_at
FROM dt_answers a
JOIN dt_sessions s     ON s.id = a.session_id
JOIN dt_items i        ON i.id = a.item_id
LEFT JOIN dt_participants p ON p.id = s.participant_id
LEFT JOIN dt_recalls r ON r.session_id = a.session_id AND r.block_no = a.block_no
LEFT JOIN dt_block_reviews br ON br.session_id = a.session_id AND br.block_no = a.block_no;

COMMENT ON VIEW v_dt_answers IS
  'One row per decision test question presented. The grain, the export, and the base of every other dt view. Carries curation status and the single `usable` definition every claim-making view filters on. No name, no email, no chosen_index and no curator note: the answer key and the participant must not be reconstructable from an export.';

-- +goose StatementEnd

-- +goose StatementBegin

CREATE VIEW v_dt_blocks AS
SELECT
  session_key,
  is_synthetic,
  status,
  session_review_status,
  block_review_status,
  instrument_version,
  item_set_version,
  block_no,
  block_load,
  load_rank,
  count(*)                                        AS answered,
  count(*) FILTER (WHERE is_correct)              AS correct,
  count(*) FILTER (WHERE is_lure)                 AS lure,
  count(*) FILTER (WHERE is_expired)              AS expired,
  count(*) FILTER (WHERE confidently_wrong)       AS confidently_wrong,
  count(*) FILTER (WHERE confidently_lured)       AS confidently_lured,
  round(100.0 * count(*) FILTER (WHERE is_correct) / count(*))  AS accuracy_pct,
  round(100.0 * count(*) FILTER (WHERE is_expired) / count(*))  AS expiry_pct,
  round(avg(confidence))                          AS mean_confidence,
  round(avg(confidence) - 100.0 * count(*) FILTER (WHERE is_correct) / count(*))
                                                  AS gap_pct,
  round(avg(brier), 4)                            AS mean_brier,
  round(avg(latency_ms))                          AS mean_latency_ms,
  round(avg(latency_vs_baseline), 2)              AS mean_latency_vs_baseline,
  max(recall_outcome)                             AS recall_outcome,
  max(digits_held)                                AS digits_held,
  max(memory_failure)                             AS memory_failure,
  bool_or(recall_executive_failure)               AS recall_executive_failure,
  min(started_at)                                 AS started_at
FROM v_dt_answers
GROUP BY session_key, is_synthetic, status, session_review_status,
         block_review_status, instrument_version, item_set_version,
         block_no, block_load, load_rank;

COMMENT ON VIEW v_dt_blocks IS
  'One row per decision test session per block, with its curation status. The review surface, so nothing is filtered out: an excluded block still appears here, labelled.';

-- +goose StatementEnd

-- +goose StatementBegin

CREATE VIEW v_dt_sessions AS
WITH blocks AS (SELECT * FROM v_dt_blocks)
SELECT
  b.session_key,
  b.is_synthetic,
  b.status,
  b.session_review_status,
  b.instrument_version,
  b.item_set_version,
  min(b.started_at)                               AS started_at,
  count(DISTINCT b.block_no)                      AS blocks_reached,
  count(DISTINCT b.block_no) FILTER (WHERE b.block_review_status = 'do_not_use')
                                                  AS blocks_excluded,
  sum(b.answered)                                 AS answered,
  sum(b.correct)                                  AS correct,
  sum(b.expired)                                  AS expired,
  sum(b.confidently_wrong)                        AS confidently_wrong,
  sum(b.confidently_lured)                        AS confidently_lured,
  round(100.0 * sum(b.correct) / sum(b.answered))       AS accuracy_pct,
  round(avg(b.mean_confidence))                   AS mean_confidence,
  round(avg(b.mean_confidence) - 100.0 * sum(b.correct) / sum(b.answered))
                                                  AS gap_pct,
  round(avg(b.mean_brier), 4)                     AS mean_brier,
  round(avg(b.accuracy_pct)      FILTER (WHERE b.block_no IN (1, 2))) AS early_accuracy_pct,
  round(avg(b.mean_confidence)   FILTER (WHERE b.block_no IN (1, 2))) AS early_confidence_pct,
  round(avg(b.accuracy_pct)      FILTER (WHERE b.block_no = 4))       AS hard_accuracy_pct,
  round(avg(b.mean_confidence)   FILTER (WHERE b.block_no = 4))       AS hard_confidence_pct,
  round(avg(b.accuracy_pct) FILTER (WHERE b.block_no = 1))            AS block1_accuracy_pct,
  round(avg(b.accuracy_pct) FILTER (WHERE b.block_no = 5))            AS block5_accuracy_pct,
  round(avg(b.accuracy_pct) FILTER (WHERE b.block_no = 5))
    - round(avg(b.accuracy_pct) FILTER (WHERE b.block_no = 1))        AS fatigue_delta_pct,
  round(avg(b.memory_failure), 2)                 AS mean_memory_failure,
  count(*) FILTER (WHERE b.recall_executive_failure) AS untransformed_blocks
FROM blocks b
GROUP BY b.session_key, b.is_synthetic, b.status, b.session_review_status,
         b.instrument_version, b.item_set_version;

COMMENT ON VIEW v_dt_sessions IS
  'One row per decision test run, with its curation status and how many of its blocks are excluded. The review surface: includes synthetic, unfinished and excluded runs, labelled rather than filtered.';

-- +goose StatementEnd

-- +goose StatementBegin

-- The claim-making views. All three filter on `usable` and nothing
-- else, and all three now report their own review composition, because
-- an unreviewed run counts (the owner reviews asynchronously and the
-- alternative is an empty dataset) and a figure computed mostly from
-- unreviewed data has to say so on the same row it appears in.
CREATE VIEW v_dt_load_curve AS
SELECT
  item_set_version,
  instrument_version,
  block_load,
  load_rank,
  count(DISTINCT session_key)                     AS sessions,
  count(DISTINCT session_key) FILTER (WHERE session_review_status <> '')
                                                  AS sessions_reviewed,
  count(DISTINCT session_key) FILTER (WHERE session_review_status = '')
                                                  AS sessions_unreviewed,
  count(*)                                        AS answers,
  round(100.0 * count(*) FILTER (WHERE is_correct) / count(*))        AS accuracy_pct,
  round(100.0 * count(*) FILTER (WHERE is_lure) / count(*))           AS lure_pct,
  round(100.0 * count(*) FILTER (WHERE is_expired) / count(*))        AS expiry_pct,
  round(avg(confidence))                          AS mean_confidence,
  round(avg(confidence) - 100.0 * count(*) FILTER (WHERE is_correct) / count(*))
                                                  AS gap_pct,
  round(100.0 * count(*) FILTER (WHERE confidently_wrong) / count(*)) AS confidently_wrong_pct,
  round(100.0 * count(*) FILTER (WHERE confidently_lured) / count(*)) AS confidently_lured_pct,
  round(avg(brier), 4)                            AS mean_brier,
  round(avg(latency_ms))                          AS mean_latency_ms,
  round(avg(latency_vs_baseline), 2)              AS mean_latency_vs_baseline,
  round(avg(memory_failure), 2)                   AS mean_memory_failure,
  (SELECT string_agg(DISTINCT f.item_family, '+' ORDER BY f.item_family)
     FROM v_dt_answers f
    WHERE f.item_set_version = v.item_set_version
      AND f.block_load = v.block_load
      AND f.usable)                               AS families
FROM v_dt_answers v
WHERE usable
GROUP BY item_set_version, instrument_version, block_load, load_rank
ORDER BY item_set_version, instrument_version, load_rank, block_load;

COMMENT ON VIEW v_dt_load_curve IS
  'Accuracy and confidence by load level: the instrument''s result. Filters on `usable`, so synthetic runs, runs marked do_not_use and individually excluded blocks are all absent. Grouped by item set and instrument version so two instruments never pool. sessions_unreviewed says how much of the figure nobody has vouched for.';

CREATE VIEW v_dt_calibration AS
SELECT
  item_set_version,
  instrument_version,
  block_load,
  load_rank,
  (confidence / 10) * 10                          AS confidence_band,
  count(*)                                        AS answers,
  count(DISTINCT session_key) FILTER (WHERE session_review_status = '')
                                                  AS sessions_unreviewed,
  round(100.0 * count(*) FILTER (WHERE is_correct) / count(*))        AS accuracy_pct,
  ((confidence / 10) * 10 + 5)
    - round(100.0 * count(*) FILTER (WHERE is_correct) / count(*))    AS overclaim_pct,
  round(avg(brier), 4)                            AS mean_brier
FROM v_dt_answers
WHERE usable AND confidence IS NOT NULL
GROUP BY item_set_version, instrument_version, block_load, load_rank, (confidence / 10) * 10
ORDER BY item_set_version, instrument_version, load_rank, block_load, confidence_band;

COMMENT ON VIEW v_dt_calibration IS
  'Confidence bands of ten against the accuracy achieved in each, split by load and instrument. Filters on `usable`.';

CREATE VIEW v_dt_items AS
SELECT
  item_code,
  item_version,
  item_family,
  count(*)                                        AS answers,
  count(DISTINCT session_key)                     AS sessions,
  count(DISTINCT session_key) FILTER (WHERE session_review_status = '')
                                                  AS sessions_unreviewed,
  round(100.0 * count(*) FILTER (WHERE is_correct) / count(*))        AS accuracy_pct,
  round(100.0 * count(*) FILTER (WHERE is_lure) / count(*))           AS lure_pct,
  round(100.0 * count(*) FILTER (WHERE is_expired) / count(*))        AS expiry_pct,
  round(avg(confidence))                          AS mean_confidence,
  round(avg(confidence) - 100.0 * count(*) FILTER (WHERE is_correct) / count(*))
                                                  AS gap_pct,
  round(avg(brier), 4)                            AS mean_brier,
  round(avg(latency_ms))                          AS mean_latency_ms,
  array_agg(DISTINCT block_no ORDER BY block_no)  AS block_nos,
  array_agg(DISTINCT item_set_version)            AS item_set_versions
FROM v_dt_answers
WHERE usable
GROUP BY item_code, item_version, item_family
ORDER BY item_code;

COMMENT ON VIEW v_dt_items IS
  'Per item: accuracy, how often the lure pulled, expiry rate and the confidence gap. Filters on `usable`. A lure_pct near zero is an item measuring nothing.';

CREATE VIEW v_dt_threshold AS
SELECT
  s.session_key,
  s.is_synthetic,
  s.status,
  s.session_review_status,
  s.accuracy_pct,
  s.gap_pct,
  s.block5_accuracy_pct,
  s.block1_accuracy_pct,
  s.fatigue_delta_pct,
  s.blocks_excluded,
  (SELECT min(a.load_rank) FROM v_dt_answers a
    WHERE a.session_key = s.session_key AND a.confidently_lured
      AND a.block_no < 5 AND a.usable)                            AS lured_at_rank,
  (SELECT min(a.block_load) FROM v_dt_answers a
    WHERE a.session_key = s.session_key AND a.confidently_lured
      AND a.block_no < 5 AND a.usable
      AND a.load_rank = (SELECT min(b.load_rank) FROM v_dt_answers b
                          WHERE b.session_key = s.session_key AND b.confidently_lured
                            AND b.block_no < 5 AND b.usable))     AS lured_at_load,
  (SELECT min(a.load_rank) FROM v_dt_answers a
    WHERE a.session_key = s.session_key AND a.confidently_wrong
      AND a.block_no < 5 AND a.usable)                            AS wrong_at_rank,
  (SELECT bool_or(a.confidently_lured) FROM v_dt_answers a
    WHERE a.session_key = s.session_key AND a.block_no = 5 AND a.usable)
                                                                  AS lured_in_control,
  s.confidently_lured,
  s.confidently_wrong,
  s.started_at
FROM v_dt_sessions s;

COMMENT ON VIEW v_dt_threshold IS
  'Per run, the lowest load on the ramp (blocks 1 to 4) at which this participant took the intended wrong answer while still reporting high confidence. Block 5 is excluded from the min() because it ranks as block 1''s difficulty and would report a fatigued participant''s threshold as the easiest load; it is reported beside as lured_in_control. The subqueries respect `usable`, so an excluded block cannot set somebody''s threshold.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Deliberately a no-op.
--
-- dt_block_reviews cannot be dropped here: v_dt_answers joins it, so
-- the DROP is refused, and CASCADE would take the view with it and
-- break the admin console and the export. Reverting the views instead
-- would mean carrying a second copy of forty columns in this file,
-- which would drift.
--
-- A down migration in this project is a development convenience, not a
-- rollback path: production only ever goes up, and a rollback rolls the
-- image back rather than the schema (deploy/README.md). The review
-- columns and tables are additions, so leaving them costs nothing.
SELECT 1;
-- +goose StatementEnd
