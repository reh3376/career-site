#!/usr/bin/env bash
# Exercise the read-only admin and member surfaces, to build the
# observe-mode sample.
#
# **Why this exists.** S3 of docs/sprint-auth-interceptor.md runs the auth
# interceptor in observe mode for a week, comparing its decision against
# what each handler's own gate does. The exit criterion is "no
# disagreements", and that is only evidence if something was compared.
#
# The first fifteen minutes of observation produced `compared=5`, all of
# it from deploy/live-check.sh, because nobody was using the site. A week
# at that rate would prove the mechanism runs and nothing about whether
# the 69 `requireAdmin` handlers agree with the interceptor, since almost
# none of them would have been called. The admin console is the only way
# most of them are reachable and much of it is used rarely.
#
# So this calls them directly. One run exercises every read-only admin
# procedure, which is a far better sample than clicking through the
# pages: the console only covers the surfaces that have a UI, and this
# covers every procedure that has a handler.
#
# **Read-only, enforced by an allowlist rather than by a pattern.** The
# list below is written out in full and deliberately excludes everything
# that mutates. A rule like "anything starting with Get or List" would
# silently pick up a new procedure the day someone adds
# `GetAndClearQueue`, and this script runs against production.
#
# Usage:
#   deploy/exercise-admin.sh [n] [https://rogerhenley.dev] [user@host]
#
#   n   how many passes to make (default 1). One login covers the whole
#       run; each pass is one call per procedure below.
#
# The calls run ON the server over ssh, so the admin password never
# leaves /opt/career-site/.env.prod, the same arrangement live-check.sh
# uses.
set -uo pipefail

PASSES="${1:-1}"
BASE="${2:-https://rogerhenley.dev}"
HOST="${3:-career@5.161.62.205}"

case "$PASSES" in ''|*[!0-9]*) echo "passes must be a number" >&2; exit 1;; esac

echo "exercising the read-only admin surface: $PASSES pass(es) against $BASE"

ssh -o StrictHostKeyChecking=no -o ConnectTimeout=15 "$HOST" \
  "BASE='$BASE' PASSES='$PASSES' bash -s" <<'REMOTE'
cd /opt/career-site || exit 1

# Read-only AdminService procedures. Every one of these only reads.
#
# Excluded on purpose, beyond the obvious mutations: GetCalendarConnectURL,
# which mints an OAuth authorization URL and carries state, and
# ChatService.RunAdminQuery and AdminService.RunDbQuery, which execute
# SQL supplied by the caller. None of those belong in a script that runs
# unattended against production.
ADMIN_RPCS="
ExportDecisionLog
ExportDecisionTestData
GetAnalytics
GetAudit
GetCalendarStatus
GetCorpusStatus
GetDecisionTestAnalysis
GetDecisionTestSettings
GetGate
GetJdFitBands
GetJdSubmissionLimit
GetMetrics
GetOpsStatus
GetPersona
GetReviewQueue
GetSchedulerSettings
ListAccessGrants
ListContactMessages
ListCorpusDocuments
ListDbTables
ListDecisionLog
ListDecisionTestRuns
ListEvalRuns
ListGoldenPostings
ListJdSubmissions
ListMeetings
ListMemberActivity
ListMembers
ListQaEntries
ListSavedQueries
"

# Procedures on other services, as Service/Method.
#
# ChatService.ListAdminQueries is gated by requireChatAdmin, a different
# helper from requireAdmin that checks account status as well as role.
#
# The rest are MEMBER-level, and they matter more than their number
# suggests. The owner's 2026-10-09 decision made the interceptor require
# an active account for MEMBER, which is stricter than the three bare
# `LookupSessionUser` gates behind GetMe and GetHistory. MEMBER is
# therefore the level whose semantics actually changed, and the first 780
# comparisons were almost entirely ADMIN, so it had barely been
# exercised.
#
# These nine cover all three separate `requireMember` definitions
# (chat.go, jd.go, meetings.go) and both bare-lookup handlers. The admin
# account is itself an active member, so it can call them.
#
# Read-only only. Excluded: SubmitJd (starts a 25-minute LLM pipeline),
# BookMeeting and CancelMeeting, CreateConversation, SendMessage,
# DeleteConversation and RateMessage (all mutations),
# ActivityService.RecordEvents (writes events), and anything needing an
# id argument such as GetConversation and GetJdResult.
OTHER_RPCS="
ChatService/ListAdminQueries
ChatService/ListConversations
ChatService/GetSuggestions
MemberService/GetMe
MemberService/GetHistory
JdService/ListMySubmissions
JdService/GetJdReviewConfig
MeetingService/GetMeetingOptions
MeetingService/GetAvailability
MeetingService/ListMyMeetings
"

U=$(grep ^ADMIN_USERNAME .env.prod | cut -d= -f2)
P=$(grep ^CAREER_SITE_ADMIN_PW .env.prod | cut -d= -f2)
[ -n "$U" ] && [ -n "$P" ] || { echo "  no admin credentials in .env.prod"; exit 1; }

total=0; ok=0; unimpl=0; badreq=0; denied=0; other=0
declare -A SEEN

# One login for the whole run, not one per pass.
#
# The login rate limiter counts *successful* logins, not only failures.
# It is ratelimit.New(5, 5.0/(15*60)) keyed on (ip, email): five
# attempts, then one token every three minutes. A first version of this
# script logged in once per pass and got http 429 on the fifth pass.
#
# Reusing the session is also closer to what it stands in for, since an
# admin opening the console signs in once and then makes many calls. So
# prefer one run with a large pass count over many small runs, which is
# both kinder to the limiter and a better sample per token spent.
CJ=$(mktemp)
trap 'rm -f "$CJ"' EXIT
code=$(curl -s -o /dev/null -w '%{http_code}' -c "$CJ" \
  -H 'Content-Type: application/json' -X POST \
  "$BASE/api/career.v1.AuthService/Login" \
  -d "$(python3 -c "import json;print(json.dumps({'email':'$U','password':'$P'}))")")
if [ "$code" != "200" ]; then
  echo "  admin login failed with http $code"
  [ "$code" = "429" ] && echo "  (rate limited; wait a few minutes and retry)"
  # Exit 2, distinct from the denial exit below. Conflating the two
  # printed "an admin procedure refused an authenticated admin" when the
  # truth was that the login never happened, which is a different
  # problem and would have sent somebody looking in the wrong place.
  exit 2
fi
echo "  signed in, reusing the session for $PASSES pass(es)"

for pass in $(seq 1 "$PASSES"); do
  for rpc in $ADMIN_RPCS; do
    c=$(curl -s -o /dev/null -w '%{http_code}' --max-time 20 -b "$CJ" \
      -H 'Content-Type: application/json' -X POST \
      "$BASE/api/career.v1.AdminService/$rpc" -d '{}')
    total=$((total+1)); SEEN[$rpc]=$c
    case "$c" in
      200) ok=$((ok+1));;
      501) unimpl=$((unimpl+1));;
      400|422) badreq=$((badreq+1));;
      401|403) denied=$((denied+1));;
      *) other=$((other+1));;
    esac
  done

  for sm in $OTHER_RPCS; do
    rpc="${sm#*/}"
    c=$(curl -s -o /dev/null -w '%{http_code}' --max-time 20 -b "$CJ" \
      -H 'Content-Type: application/json' -X POST \
      "$BASE/api/career.v1.${sm%%/*}/$rpc" -d '{}')
    total=$((total+1)); SEEN[$sm]=$c
    case "$c" in
      200) ok=$((ok+1));;
      501) unimpl=$((unimpl+1));;
      400|422) badreq=$((badreq+1));;
      401|403) denied=$((denied+1));;
      *) other=$((other+1));;
    esac
  done

  echo "  pass $pass done"
done

# One logout at the end, closing the session this run opened. Logout is
# PUBLIC as of 2026-10-09, so it needs no session of its own.
curl -s -o /dev/null -b "$CJ" -H 'Content-Type: application/json' \
  -X POST "$BASE/api/career.v1.AuthService/Logout" -d '{}'

echo "  calls: $total total, $ok ok, $unimpl unimplemented, $badreq needs-arguments, $other other"

# A 401 or 403 here would mean the admin session was refused an admin
# procedure, which is the one outcome that must not happen.
if [ "$denied" -gt 0 ]; then
  echo "  DENIED: $denied call(s) were refused for an authenticated admin."
  echo "  Per-procedure codes:"
  for rpc in "${!SEEN[@]}"; do
    case "${SEEN[$rpc]}" in 401|403) echo "    $rpc -> ${SEEN[$rpc]}";; esac
  done
  exit 1
fi

# What counts as a meaningful comparison is "the handler's own gate ran",
# not "the call succeeded".
#
# Every gate is the first statement in its handler, so a 400 from
# in-handler validation means the gate already ran and allowed the
# caller: MeetingService/GetAvailability calls requireMember and only
# then rejects a missing duration. Checked in meetings.go rather than
# assumed, because there is no validation interceptor in front of these
# handlers, so a 400 can only come from inside one.
#
# A 501 is the opposite. It comes from the embedded
# Unimplemented*ServiceHandler, so no handler and no gate ran. The
# interceptor still compared it, since it decides before Connect answers,
# but there was nothing on the other side to agree with.
echo "  gate ran and agreed (the meaningful comparisons):"
for rpc in "${!SEEN[@]}"; do
  case "${SEEN[$rpc]}" in
    200) echo "    $rpc";;
    400|422) echo "    $rpc (gate ran, then rejected the empty request)";;
  esac
done | sort

echo "  no handler, so no gate ran:"
for rpc in "${!SEEN[@]}"; do
  [ "${SEEN[$rpc]}" = "501" ] && echo "    $rpc -> 501 unimplemented"
done | sort

echo "-- any disagreement the interceptor logged while this ran:"
docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml \
  logs api --since 10m </dev/null 2>/dev/null \
  | grep -E 'auth policy disagreement|handler_denied_permission|no policy' \
  | tail -20 || echo "    none"
REMOTE

rc=$?
echo
case "$rc" in
  0) ;;
  2) echo "Could not sign in, so nothing was exercised. Nothing is wrong with"
     echo "the auth policy; retry once the login rate limit has cleared."
     exit 2;;
  *) echo "FAILED: an admin procedure refused an authenticated admin. Investigate"
     echo "before S4 flips AUTH_INTERCEPTOR_MODE to enforce."
     exit "$rc";;
esac
echo "The running totals arrive with the next periodic summary, within 15"
echo "minutes. To read it now:"
echo "  ssh $HOST \"cd /opt/career-site && docker compose --env-file .env.prod \\"
echo "    -f docker-compose.yml -f docker-compose.prod.yml logs api --since 20m \\"
echo "    | grep 'auth policy observation' | tail -2\""
