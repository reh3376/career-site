#!/usr/bin/env bash
# Compare two evaluation runs, in one pass.
#
#   scripts/eval_compare.sh <new_run> [old_run]
#
# Every run so far has been analysed by hand-writing the same queries,
# and the important one is the fiddliest. Requirement ids are
# re-extracted per run and are not stable, so verdicts have to be
# diffed by requirement TEXT. Reading them by id attributes a change to
# the wrong requirement, which is exactly what happened on run 11: CAI
# rose and it was credited to the regulated-manufacturing fix, when the
# fix had not landed and a different requirement had moved. Run 12
# corrected it by matching on text.
#
# A score that has not moved can still have two requirements swapped
# underneath it, so the verdict diff is worth reading even when the
# numbers look identical.
#
# Read-only. Safe to run against a live evaluation; the rows it reads
# are written as each posting finishes.
set -euo pipefail

NEW="${1:?usage: eval_compare.sh <new_run> [old_run]}"
OLD="${2:-$((NEW - 1))}"
SERVER="${SERVER:-career@5.161.62.205}"
PSQL="docker exec career-site-postgres-1 psql -U career -d career -P pager=off"

S() { ssh -o ConnectTimeout=20 "$SERVER" "$@"; }

echo "=== scores: run $NEW against run $OLD ==="
S "$PSQL -c \"
SELECT g.name AS posting, g.expected_gate AS expect, i.gate_side AS got,
       round(i.match_score::numeric,4) AS new,
       (SELECT round(match_score::numeric,4) FROM eval_items
         WHERE eval_run_id=$OLD AND golden_id=i.golden_id) AS old,
       round((i.match_score - (SELECT match_score FROM eval_items
         WHERE eval_run_id=$OLD AND golden_id=i.golden_id))::numeric,4) AS delta,
       left(i.error,46) AS error
  FROM eval_items i JOIN golden_postings g ON g.id=i.golden_id
 WHERE i.eval_run_id=$NEW
 ORDER BY i.match_score DESC NULLS LAST\""

echo
echo "=== the runs themselves ==="
S "$PSQL -x -c \"
SELECT id, status, finished_at-started_at AS took, gate_correct, scored, errors,
       order_violations, round(margin::numeric,4) AS margin,
       corpus_fingerprint, left(note,64) AS note
  FROM eval_runs WHERE id IN ($NEW,$OLD) ORDER BY id DESC\""

echo
echo "=== verdicts that changed, matched by requirement TEXT ==="
echo "(ids are re-extracted per run and are not stable; never diff on them)"
S "$PSQL -c \"
WITH pairs AS (
  SELECT g.name AS posting,
         (SELECT submission_id FROM eval_items WHERE eval_run_id=$OLD AND golden_id=g.id) AS old_sub,
         (SELECT submission_id FROM eval_items WHERE eval_run_id=$NEW AND golden_id=g.id) AS new_sub
    FROM golden_postings g
   WHERE EXISTS (SELECT 1 FROM eval_items WHERE eval_run_id=$NEW AND golden_id=g.id)
),
a AS (SELECT p.posting, r->>'text' AS txt, j->>'verdict' AS v
        FROM pairs p JOIN jd_submissions s ON s.id=p.old_sub,
             jsonb_array_elements(s.assessment->'judgments') j,
             jsonb_array_elements(s.assessment->'requirements') r
       WHERE r->>'id'=j->>'requirement_id'),
b AS (SELECT p.posting, r->>'text' AS txt, j->>'verdict' AS v
        FROM pairs p JOIN jd_submissions s ON s.id=p.new_sub,
             jsonb_array_elements(s.assessment->'judgments') j,
             jsonb_array_elements(s.assessment->'requirements') r
       WHERE r->>'id'=j->>'requirement_id')
SELECT left(coalesce(a.posting,b.posting),32) AS posting,
       left(coalesce(a.txt,b.txt),58) AS requirement,
       a.v AS old, b.v AS new
  FROM a FULL JOIN b ON a.posting=b.posting AND a.txt=b.txt
 WHERE a.v IS DISTINCT FROM b.v
 ORDER BY 1,2\""

echo
echo "=== verdict totals and rationale issues, run $NEW ==="
S "$PSQL -c \"
SELECT j->>'verdict' AS verdict, count(*)
  FROM eval_items i JOIN jd_submissions s ON s.id=i.submission_id,
       jsonb_array_elements(s.assessment->'judgments') j
 WHERE i.eval_run_id=$NEW GROUP BY 1 ORDER BY 2 DESC\""
S "$PSQL -c \"
SELECT ri->>'kind' AS rationale_issue, count(*)
  FROM eval_items i JOIN jd_submissions s ON s.id=i.submission_id,
       jsonb_array_elements(s.assessment->'judgments') j,
       jsonb_array_elements(coalesce(j->'rationale_issues','[]'::jsonb)) ri
 WHERE i.eval_run_id=$NEW GROUP BY 1\""

echo
echo "=== corpus each run read (manifest, migrations 00037/00040) ==="
# content_hash arrives with 00040 and the manifest itself with 00037.
# Against a database that predates either, say so rather than fail: this
# script is most useful mid-run, and a run in flight is exactly when the
# migration carrying the column is still waiting to deploy.
HAVE_MANIFEST=$(S "$PSQL -tAc \"SELECT to_regclass('eval_run_documents') IS NOT NULL\"" | tr -d ' \r')
HAVE_HASH=$(S "$PSQL -tAc \"SELECT count(*) FROM information_schema.columns WHERE table_name='eval_run_documents' AND column_name='content_hash'\"" | tr -d ' \r')
if [ "$HAVE_MANIFEST" != "t" ]; then
  echo "  no manifest on this database (migration 00037 not applied)"
  exit 0
fi
if [ "$HAVE_HASH" != "1" ]; then
  echo "  content_hash not applied yet (migration 00040); showing counts only"
  S "$PSQL -c \"
SELECT eval_run_id AS run, count(*) AS docs, sum(chunk_count) AS chunks
  FROM eval_run_documents WHERE eval_run_id IN ($NEW,$OLD)
 GROUP BY 1 ORDER BY 1 DESC\""
  exit 0
fi
S "$PSQL -c \"
SELECT eval_run_id AS run, count(*) AS docs, sum(chunk_count) AS chunks,
       count(content_hash) AS hashed
  FROM eval_run_documents WHERE eval_run_id IN ($NEW,$OLD)
 GROUP BY 1 ORDER BY 1 DESC\""
S "$PSQL -c \"
SELECT left(coalesce(n.title, o.title),44) AS document,
       CASE WHEN o.document_id IS NULL THEN 'added'
            WHEN n.document_id IS NULL THEN 'removed'
            ELSE 'text changed' END AS change
  FROM (SELECT * FROM eval_run_documents WHERE eval_run_id=$NEW) n
  FULL JOIN (SELECT * FROM eval_run_documents WHERE eval_run_id=$OLD) o
    ON o.document_id = n.document_id
 WHERE o.document_id IS NULL OR n.document_id IS NULL
    OR n.content_hash IS DISTINCT FROM o.content_hash
 ORDER BY 2, 1\""
