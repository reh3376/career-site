#!/usr/bin/env bash
# S4 exit checks: is the auth interceptor enforcing, correctly, live?
#
# The plan's exit criteria for S4, executable, so they are run rather
# than recited. Safe to run in either mode; it reports the mode it found
# and says which expectations apply.
#
# **What this can and cannot establish.** Four of the six criteria can be
# checked against production with the credentials available. Two cannot,
# and they are named in the output rather than quietly skipped:
#
#   - a member session calling an admin procedure, expecting
#     permission_denied. There is one admin and the other accounts belong
#     to real people whose passwords nobody has, so producing a
#     member-only session means registering an account in production.
#   - GetMe on a non-active session, the allow_unverified path, which
#     needs an account that is not active.
#
# Both are covered by the enforce-mode tests in
# internal/authpolicy/interceptor_test.go. Saying so is the point: "the
# exit checks passed" must not be read as covering them.
#
# Usage:
#   deploy/auth-enforce-checks.sh [https://rogerhenley.dev] [user@host]
set -uo pipefail

BASE="${1:-https://rogerhenley.dev}"
HOST="${2:-career@5.161.62.205}"
pass=0; fail=0

ok()  { printf '  ok    %s\n' "$1"; pass=$((pass+1)); }
bad() { printf '  FAIL  %s\n' "$1"; fail=$((fail+1)); }

code() { curl -s -o /dev/null -w '%{http_code}' --max-time 20 "$@"; }

MODE=$(ssh -o ConnectTimeout=20 "$HOST" \
  "cd /opt/career-site && grep '^AUTH_INTERCEPTOR_MODE=' .env.prod | cut -d= -f2" 2>/dev/null)
MODE="${MODE:-observe}"
echo "interceptor mode on the server: $MODE"
echo

echo "-- public surfaces stay reachable (these must hold in either mode)"
for p in / /decision-test /login; do
  c=$(code "$BASE$p")
  [ "$c" = "200" ] && ok "$p -> 200" || bad "$p -> $c, expected 200"
done
c=$(code -H 'Content-Type: application/json' -X POST \
  "$BASE/api/career.v1.SystemService/GetVersion" -d '{}')
[ "$c" = "200" ] && ok "GetVersion anonymous -> 200" || bad "GetVersion anonymous -> $c"

# The decision test is the busiest public surface and the one with real
# participants. StartSession is PUBLIC, and it must not need a session.
c=$(code -H 'Content-Type: application/json' -X POST \
  "$BASE/api/career.v1.DecisionTestService/StartSession" \
  -d '{"synthetic":true,"intake":{}}')
case "$c" in
  200|400|422) ok "decision test StartSession reachable anonymously -> $c" ;;
  401|403) bad "decision test StartSession refused anonymously -> $c (the interceptor is denying a PUBLIC procedure)" ;;
  *) bad "decision test StartSession -> $c" ;;
esac

echo
echo "-- an admin procedure refuses an anonymous caller"
c=$(code -H 'Content-Type: application/json' -X POST \
  "$BASE/api/career.v1.AdminService/ListMembers" -d '{}')
if [ "$MODE" = "enforce" ]; then
  [ "$c" = "401" ] && ok "ListMembers anonymous -> 401 unauthenticated" \
    || bad "ListMembers anonymous -> $c, expected 401"
else
  # In observe mode the handler's own requireAdmin denies, so 401 is
  # still right; it just does not prove the interceptor did it.
  [ "$c" = "401" ] && ok "ListMembers anonymous -> 401 (by the handler's gate; observe mode)" \
    || bad "ListMembers anonymous -> $c, expected 401"
fi

# A MEMBER procedure, same question.
c=$(code -H 'Content-Type: application/json' -X POST \
  "$BASE/api/career.v1.MemberService/GetHistory" -d '{}')
[ "$c" = "401" ] && ok "GetHistory anonymous -> 401" || bad "GetHistory anonymous -> $c, expected 401"

echo
echo "-- an admin session still reaches the admin surface (over ssh, password stays on the box)"
ADMIN_OUT=$(ssh -o ConnectTimeout=20 "$HOST" "BASE='$BASE' bash -s" <<'REMOTE'
cd /opt/career-site || exit 1
U=$(grep ^ADMIN_USERNAME .env.prod | cut -d= -f2)
P=$(grep ^CAREER_SITE_ADMIN_PW .env.prod | cut -d= -f2)
CJ=$(mktemp); trap 'rm -f "$CJ"' EXIT
lc=$(curl -s -o /dev/null -w '%{http_code}' -c "$CJ" -H 'Content-Type: application/json' \
  -X POST "$BASE/api/career.v1.AuthService/Login" \
  -d "$(python3 -c "import json;print(json.dumps({'email':'$U','password':'$P'}))")")
echo "login:$lc"
[ "$lc" = "200" ] || exit 0
for rpc in AdminService/ListMembers AdminService/ListCorpusDocuments AdminService/GetOpsStatus; do
  echo "$rpc:$(curl -s -o /dev/null -w '%{http_code}' -b "$CJ" -H 'Content-Type: application/json' \
    -X POST "$BASE/api/career.v1.$rpc" -d '{}')"
done
# The admin is also an active member, so the member surface answers too.
for rpc in MemberService/GetMe MemberService/GetHistory; do
  echo "$rpc:$(curl -s -o /dev/null -w '%{http_code}' -b "$CJ" -H 'Content-Type: application/json' \
    -X POST "$BASE/api/career.v1.$rpc" -d '{}')"
done
curl -s -o /dev/null -b "$CJ" -H 'Content-Type: application/json' \
  -X POST "$BASE/api/career.v1.AuthService/Logout" -d '{}'
REMOTE
)

lg=$(echo "$ADMIN_OUT" | grep '^login:' | cut -d: -f2)
if [ "$lg" = "200" ]; then
  ok "admin login -> 200"
  while IFS= read -r line; do
    case "$line" in
      login:*) ;;
      *:200) ok "${line%:*} with an admin session -> 200" ;;
      *:429) printf '  skip  %s rate limited\n' "${line%:*}" ;;
      *) bad "${line%:*} with an admin session -> ${line##*:}, expected 200" ;;
    esac
  done <<< "$ADMIN_OUT"
elif [ "$lg" = "429" ]; then
  echo "  skip  admin login rate limited; wait three minutes and retry"
else
  bad "admin login -> ${lg:-no answer}"
fi

echo
echo "-- not checked live, and why"
echo "  a member session on an admin procedure (expecting permission_denied):"
echo "    needs a non-admin session. One admin exists and the other accounts"
echo "    belong to real people, so this would mean registering an account in"
echo "    production. Covered by TestEnforceModeDecisions."
echo "  GetMe on a non-active session (the allow_unverified path):"
echo "    needs an account that is not active. Covered by"
echo "    TestEnforceModeDecisions/allow_unverified_procedure,_suspended_member."

echo
echo "-- what the interceptor logged while this ran"
ssh -o ConnectTimeout=20 "$HOST" "cd /opt/career-site && docker compose --env-file .env.prod \
  -f docker-compose.yml -f docker-compose.prod.yml logs api --since 5m </dev/null 2>/dev/null \
  | grep -E 'auth policy disagreement|no policy|auth policy observation' | tail -5" \
  | sed 's/^api-1  | /    /' || true

echo
echo "== result: $pass ok, $fail failed (mode=$MODE) =="
[ "$fail" -eq 0 ] || exit 1
