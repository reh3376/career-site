#!/usr/bin/env bash
# Push the private decision-test item bank to the server.
#
#   deploy/items-sync.sh                 # sync from the default path
#   deploy/items-sync.sh path/to.json    # from somewhere else
#   DRY_RUN=1 deploy/items-sync.sh       # show what would change
#
# **Why this exists.** The item bank is an answer key. It was seeded in
# a migration in a public repository on 2026-10-04, which made the run
# page's claim that a leaked key is fatal straightforwardly false. The
# published items are retired in migration 00060 and the live bank now
# lives outside the repository, in docs/personal/, which is gitignored.
#
# This is the same shape as corpus-sync.sh, for the same reason: content
# the repository must not hold, pushed to the box by the owner.
#
# **What the server does without it.** Migration 00060 seeds a fixture
# bank, which is structurally correct and obviously not the instrument.
# The api refuses to draw from fixtures unless DT_ALLOW_FIXTURE_BANK=1,
# which production does not set, so a box that has never been synced
# refuses to start a run rather than asking a volunteer which answer is
# the number four. The failure is loud and the fix is this script.
set -euo pipefail

SERVER=${SERVER:-career@5.161.62.205}
REMOTE=${REMOTE:-/opt/career-site}
REPO_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
SRC=${1:-$REPO_DIR/docs/personal/decision-test-item-drafts.json}

S() { ssh -o ConnectTimeout=20 "$SERVER" "$@"; }
CS='docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml'
say() { printf '\n\033[1m== %s\033[0m\n' "$*"; }
die() { printf '\n\033[31m%s\033[0m\n' "$*" >&2; exit 1; }

[ -f "$SRC" ] || die "no item file at $SRC. The bank is not in this repository; it lives in docs/personal/."

# Refuse to sync something that is not a whole bank. A partial upload
# would deactivate the fixtures and leave the pools short, which the
# draw would then refuse on: the test would be down and the cause would
# be this script having been given a truncated file.
say "checking $SRC"
python3 - "$SRC" <<'PY'
import json, sys, collections
items = json.load(open(sys.argv[1]))
need = {"arithmetic": 30, "syllogism": 30, "base_rate": 15, "conjunction": 15}
have = collections.Counter(i["family"] for i in items)
problems = []
for fam, n in need.items():
    if have[fam] < n:
        problems.append(f"{fam}: {have[fam]} items, need at least {n}")
codes = [i["code"] for i in items]
dupes = [c for c, n in collections.Counter(codes).items() if n > 1]
if dupes:
    problems.append("duplicate codes: " + ", ".join(sorted(dupes)))
for i in items:
    if i["correct"] == i["lure"]:
        problems.append(f"{i['code']}: the lure is on the correct answer")
    if not (0 <= i["correct"] < len(i["options"])):
        problems.append(f"{i['code']}: correct index out of range")
    if not (0 <= i["lure"] < len(i["options"])):
        problems.append(f"{i['code']}: lure index out of range")
if problems:
    print("\n".join("  " + p for p in problems))
    sys.exit(1)
print(f"  {len(items)} items, " + ", ".join(f"{k} {have[k]}" for k in sorted(have)))
PY

# Rendered to SQL here and piped in, rather than copying the file to the
# box. The key never lands on the server's disk, only in its database.
say "building the upsert"
SQL=$(python3 - "$SRC" <<'PY'
import json, sys
items = json.load(open(sys.argv[1]))
def q(s): return "'" + str(s).replace("'", "''") + "'"
print("BEGIN;")
# Upsert, so re-running after fixing one item's wording is safe and
# does not renumber anything.
for n, i in enumerate(items, start=1):
    print(f"""INSERT INTO dt_items
  (tenant_id, code, version, family, kind, prompt, reminder, options,
   correct_index, lure_index, rationale, active, position, is_fixture)
VALUES (1, {q(i['code'])}, 1, {q(i['family'])}, 'scored', {q(i['prompt'])},
        {q(i.get('reminder',''))}, {q(json.dumps(i['options']))}::jsonb,
        {i['correct']}, {i['lure']}, {q(i.get('why',''))}, true, {n}, false)
ON CONFLICT (tenant_id, code, version) DO UPDATE SET
  family = excluded.family, prompt = excluded.prompt,
  reminder = excluded.reminder, options = excluded.options,
  correct_index = excluded.correct_index, lure_index = excluded.lure_index,
  rationale = excluded.rationale, active = true, is_fixture = false,
  updated_at = now();""")
# Only once the real bank is in. Deactivating first would leave a window
# with no bank at all, and the draw refuses on an empty pool.
print("UPDATE dt_items SET active = false, updated_at = now() WHERE is_fixture AND active;")
print("COMMIT;")
PY
)

if [ -n "${DRY_RUN:-}" ]; then
  say "dry run, not sending"
  echo "$SQL" | head -20
  echo "  ... $(echo "$SQL" | wc -l | tr -d ' ') lines total"
  exit 0
fi

say "sending to $SERVER"
printf '%s\n' "$SQL" | S "cd $REMOTE && $CS exec -T postgres psql -U career -d career -v ON_ERROR_STOP=1 -q"

say "what the server holds now"
S "cd $REMOTE && $CS exec -T postgres psql -U career -d career -P pager=off -c 'SELECT category, real_items, fixtures, per_test, spare, can_vary FROM v_dt_item_pools ORDER BY category;'"

# The whole point: every category must be able to vary, or a draw is not
# a draw and two participants see the same test.
BAD=$(S "cd $REMOTE && $CS exec -T postgres psql -U career -d career -Atc \"SELECT count(*) FROM v_dt_item_pools WHERE NOT can_vary\"" | tr -d '\r')
if [ "$BAD" != "0" ]; then
  die "$BAD categories still cannot vary. The draw will work but every participant sees the same questions."
fi
say "done: the live bank is private and every category can vary"
