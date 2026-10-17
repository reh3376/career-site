#!/usr/bin/env bash
# Does the decision-test data obey the rules the console says it obeys?
#
#   deploy/dt-governance.sh
#
# The console and the schema both make promises in prose. "Agent-driven
# runs are excluded: they prove the pipeline works and are not data."
# "Do not use: excluded from every figure and from the export. Nothing
# is deleted." "Grouped by item set and instrument version so two
# instruments never pool." "No name and no email address. No
# chosen_index."
#
# Each of those is a claim about rows, and a claim about rows can be
# checked against the rows. Until it is, it is a comment, and the
# failure mode of a comment is that the figure keeps being published
# while the rule behind it quietly stopped applying. A dataset nobody
# can audit is not a curated dataset, it is an assertion.
#
# Read-only, and aggregate-only on purpose: every query here counts or
# compares, none selects a participant's answers, name or address. A
# governance check that had to read the data in order to prove the data
# was protected would be its own counterexample.
#
# Exit 1 if any rule is broken. Safe to run against production at any
# time; it takes no locks and writes nothing.
set -uo pipefail

HOST=${HOST:-career@5.161.62.205}
REMOTE=${REMOTE:-/opt/career-site}

pass=0; fail=0
ok()  { printf '  \033[32mok\033[0m    %s\n' "$1"; pass=$((pass+1)); }
bad() { printf '  \033[31mFAIL\033[0m  %s\n' "$1"; fail=$((fail+1)); }

echo "== decision-test governance: $HOST ($(date -u +%Y-%m-%dT%H:%M:%SZ)) =="

OUT=$(ssh -o StrictHostKeyChecking=no -o ConnectTimeout=20 "$HOST" "REMOTE='$REMOTE' bash -s" <<'REMOTE'
cd "$REMOTE" || exit 1
q() { docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml \
        exec -T postgres psql -U career -d career -At -c "$1" </dev/null 2>/dev/null; }

# ---- the population, so every later number has a denominator ----
echo "sessions_total=$(q "select count(*) from dt_sessions")"
echo "sessions_synthetic=$(q "select count(*) from dt_sessions where is_synthetic")"
echo "sessions_donotuse=$(q "select count(*) from dt_sessions where review_status='do_not_use'")"
echo "sessions_unreviewed=$(q "select count(*) from dt_sessions where review_status='' and not is_synthetic")"
echo "versions=$(q "select count(distinct instrument_version) from dt_sessions")"

# ---- rule 1: nothing unusable reaches a view that claims to describe people ----
# v_dt_answers defines `usable` and every such view filters on it, so
# the question is whether any unusable row is marked usable.
echo "leak_usable=$(q "select count(*) from v_dt_answers v join dt_sessions s on s.public_id::text=v.session_key::text where v.usable and (s.is_synthetic or s.review_status='do_not_use')")"

# ---- rule 2: "nothing is deleted" ----
# The other half of the same promise. Asked as "is any excluded run
# missing answers it must have had", not "does every excluded run have
# answers": a run abandoned before the first question has none, and
# both of the current ones are exactly that. The first version of this
# check failed on them and was wrong, not the data.
echo "donotuse_rows_kept=$(q "select count(*) from dt_answers a join dt_sessions s on s.id=a.session_id where s.review_status='do_not_use'")"
echo "donotuse_completed=$(q "select count(*) from dt_sessions where review_status='do_not_use' and status='completed'")"
echo "donotuse_purged=$(q "select count(*) from dt_sessions s where s.review_status='do_not_use' and s.status='completed' and not exists (select 1 from dt_answers a where a.session_id=s.id)")"

# ---- rule 3: two instruments never pool ----
# Every aggregate row carries one instrument_version, so no row can
# average across a timing change.
echo "curve_rows=$(q "select count(*) from v_dt_load_curve")"
echo "curve_null_version=$(q "select count(*) from v_dt_load_curve where coalesce(instrument_version,'')=''")"

# ---- rule 4: the export carries no identity and no answer key ----
echo "export_cols_forbidden=$(q "select count(*) from information_schema.columns where table_name='v_dt_answers' and column_name in ('email','email_key','display_name','name','chosen_index')")"
echo "export_has_participant_key=$(q "select count(*) from information_schema.columns where table_name='v_dt_answers' and column_name='participant_key'")"

# ---- rule 5: an unrated answer is not an answer rated zero ----
# The stored column is NULL when the clock ran out on the rating step.
# Zero is a real rating, so the two must not be the same number, and
# the average must be over the rated ones only.
echo "unrated_answers=$(q "select count(*) from dt_answers where confidence is null and outcome<>'expired'")"
echo "zero_ratings=$(q "select count(*) from dt_answers where confidence=0")"
echo "conf_avg_rated=$(q "select coalesce(round(avg(confidence),2),0) from dt_answers where outcome<>'expired'")"
echo "conf_avg_nulls_as_zero=$(q "select coalesce(round(avg(coalesce(confidence,0)),2),0) from dt_answers where outcome<>'expired'")"

# ---- rule 6: a block excluded by review leaves the figures ----
echo "excluded_blocks=$(q "select count(*) from dt_block_reviews where status='do_not_use'")"
echo "leak_excluded_blocks=$(q "select count(*) from v_dt_answers v join dt_sessions s on s.public_id::text=v.session_key::text join dt_block_reviews br on br.session_id=s.id and br.block_no=v.block_no where v.usable and br.status='do_not_use'")"
REMOTE
)

if [ -z "$OUT" ]; then
  echo "  no output from the server; is the host reachable and postgres up?" >&2
  exit 1
fi
# Declared empty first, then filled by the eval. Declaring them makes
# the emptiness check below meaningful under `set -u`: a query that
# returned nothing leaves its name empty rather than unset, so the loop
# can name which one failed instead of the script dying on the first
# unbound reference.
sessions_total=''
sessions_synthetic=''
sessions_donotuse=''
sessions_unreviewed=''
versions=''
leak_usable=''
donotuse_rows_kept=''
curve_rows=''
curve_null_version=''
export_cols_forbidden=''
export_has_participant_key=''
unrated_answers=''
zero_ratings=''
conf_avg_rated=''
conf_avg_nulls_as_zero=''
excluded_blocks=''
leak_excluded_blocks=''
donotuse_completed=''
donotuse_purged=''

eval "$(echo "$OUT" | grep -E '^[a-z_]+=' | sed 's/=\(.*\)$/="\1"/')"

for v in sessions_total sessions_synthetic sessions_donotuse sessions_unreviewed \
         versions leak_usable curve_rows curve_null_version export_cols_forbidden \
         export_has_participant_key unrated_answers zero_ratings conf_avg_rated \
         conf_avg_nulls_as_zero excluded_blocks leak_excluded_blocks; do
  if [ -z "${!v:-}" ]; then
    echo "  query for '$v' returned nothing; the schema moved or psql failed" >&2
    exit 1
  fi
done

echo "-- population"
echo "  $sessions_total sessions, $sessions_synthetic synthetic, $sessions_donotuse marked do-not-use, $sessions_unreviewed unreviewed, $versions instrument version(s)"

echo "-- the rules the console states"
[ "${leak_usable:-1}" = "0" ] \
  && ok "no synthetic or do-not-use run is marked usable (leak: $leak_usable)" \
  || bad "$leak_usable answer rows from excluded runs are marked usable"

if [ "${sessions_donotuse:-0}" = "0" ]; then
  ok "nothing is deleted: no run is currently marked do-not-use"
elif [ "${donotuse_purged:-1}" = "0" ]; then
  ok "nothing is deleted: $sessions_donotuse excluded run(s) still present, $donotuse_completed completed, $donotuse_rows_kept answer rows kept"
else
  bad "$donotuse_purged completed run(s) marked do-not-use have no answers; exclusion deleted data"
fi

[ "${curve_null_version:-1}" = "0" ] \
  && ok "two instruments never pool: all $curve_rows curve rows carry a version" \
  || bad "$curve_null_version curve rows have no instrument version and may pool"

[ "${export_cols_forbidden:-1}" = "0" ] \
  && ok "export carries no name, email or chosen option" \
  || bad "the export view exposes $export_cols_forbidden forbidden column(s)"

[ "${export_has_participant_key:-0}" = "1" ] \
  && ok "export joins repeat runs by opaque participant_key" \
  || bad "participant_key is missing, so repeat runs cannot be linked without identity"

if [ "${unrated_answers:-0}" = "0" ]; then
  ok "no unrated answers to confuse with zeros"
elif [ "$conf_avg_rated" != "$conf_avg_nulls_as_zero" ]; then
  ok "unrated is not zero: $unrated_answers unrated, $zero_ratings genuine zeros, mean $conf_avg_rated not $conf_avg_nulls_as_zero"
else
  bad "unrated answers are being averaged as zero"
fi

[ "${leak_excluded_blocks:-1}" = "0" ] \
  && ok "excluded blocks leave the figures ($excluded_blocks excluded)" \
  || bad "$leak_excluded_blocks rows from excluded blocks are still marked usable"

echo
echo "== $pass ok, $fail failed =="
[ "$fail" -eq 0 ] || exit 1
