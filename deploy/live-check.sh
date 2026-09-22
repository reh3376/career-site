#!/usr/bin/env bash
# Live functional check of a running career-site deployment.
#
# Read-only except for one clearly-labelled JD submission when --submit
# is given. Prints a compact report; run it before a maintenance
# window and again after, then diff the two outputs.
#
# Usage:
#   deploy/live-check.sh [--submit] [https://rogerhenley.dev] [user@host]
#
# The public checks run from wherever the script is invoked. The admin
# checks run ON the server over ssh so the admin password never leaves
# /opt/career-site/.env.prod.
set -uo pipefail

SUBMIT=0
if [[ "${1:-}" == "--submit" ]]; then SUBMIT=1; shift; fi
BASE="${1:-https://rogerhenley.dev}"
HOST="${2:-career@5.161.62.205}"
pass=0; fail=0
ok()   { pass=$((pass+1)); printf '  ok    %s\n' "$1"; }
bad()  { fail=$((fail+1)); printf '  FAIL  %s\n' "$1"; }
check(){ if eval "$2"; then ok "$1"; else bad "$1"; fi; }

echo "== live check: $BASE  ($(date -u +%Y-%m-%dT%H:%M:%SZ)) =="

# The access policy, asserted from outside. It lives in
# apps/web/src/lib/public-routes.ts; if a route moves across the line
# there, it moves here too, and a mismatch means the site is not
# enforcing what we think it is.
echo "-- access policy (evidence is public, the reviewer is not)"
for path in / /login /contact /register /articles /gallery /privacy /terms; do
  code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 20 "$BASE$path")
  check "GET $path (anonymous) -> $code" "[ '$code' = '200' ]"
done
for path in /jd-upload /home /settings /admin; do
  code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 20 "$BASE$path")
  check "GET $path (anonymous) -> $code, redirected" "[ '$code' = '307' ] || [ '$code' = '302' ]"
done
# The gallery serves a subset anonymously; if the filter broke, every
# photo would be public. Counting the rendered derivatives is the
# cheapest way to notice from outside.
shown=$(curl -s --max-time 20 "$BASE/gallery" | grep -o 'images/gallery/[a-z0-9-]*-1600' | sort -u | wc -l | tr -d ' ')
check "gallery shows the public subset to anonymous -> $shown photos" "[ '$shown' -gt 0 ] && [ '$shown' -le 8 ]"
# Crawlers are invited to the public surface and kept off the rest.
robots=$(curl -s --max-time 10 "$BASE/robots.txt")
check "robots allows /articles" "echo '$robots' | grep -q 'Allow: /articles'"
check "robots disallows /jd-upload" "echo '$robots' | grep -q 'Disallow: /jd-upload'"
smap=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$BASE/sitemap.xml")
check "sitemap.xml -> $smap" "[ '$smap' = '200' ]"
hz=$(curl -s --max-time 10 "$BASE/api/healthz"); check "healthz: $hz" "echo '$hz' | grep -q '\"ok\"'"
rz=$(curl -s --max-time 10 "$BASE/api/readyz");  check "readyz: $rz"   "echo '$rz' | grep -q '\"postgres\":true' && echo '$rz' | grep -q '\"sidecar\":true'"
code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$BASE/api/jd/resume/1.pdf"); check "pdf route refuses without a session -> $code" "[ '$code' = '401' ] || [ '$code' = '403' ] || [ '$code' = '404' ]"
code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 -H 'Content-Type: application/json' -X POST "$BASE/api/career.v1.JdService/SubmitJd" -d '{"jdText":"anonymous probe"}')
check "SubmitJd refuses anonymous callers -> $code" "[ '$code' = '401' ] || [ '$code' = '403' ]"

# The JD submission itself is a member action, so it runs on the server
# with the admin session (below), where the password already lives.
echo "-- server side (over ssh)"
REMOTE_OUT=$(mktemp)
ssh -o StrictHostKeyChecking=no -o ConnectTimeout=15 "$HOST" "BASE='$BASE' SUBMIT='$SUBMIT' bash -s" <<'REMOTE' | tee "$REMOTE_OUT"
cd /opt/career-site || exit 1
# Admin RPCs go through the public HTTPS origin: Caddy redirects plain
# http://localhost to HTTPS, and the api port is not published.
# stdin is this script; `compose exec -T` would consume it, hence </dev/null.
cs() { docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml "$@" </dev/null; }
echo "  image tag: $(grep ^IMAGE_TAG .env.prod | cut -d= -f2)  embed=$(grep ^SIDECAR_EMBED_PROVIDER .env.prod | cut -d= -f2)  llm=$(grep ^SIDECAR_LLM_PROVIDER .env.prod | cut -d= -f2)  pdf_pw_set=$([ -n "$(grep ^RESUME_PDF_OWNER_PASSWORD .env.prod | cut -d= -f2)" ] && echo yes || echo no)"
echo "  containers:"; cs ps --format '    {{.Name}} {{.Status}}'
echo "  migration: $(cs exec -T postgres psql -U career -d career -At -c 'select max(version_id) from goose_db_version' 2>/dev/null)"
echo "  corpus: $(cs exec -T postgres psql -U career -d career -At -c "select count(*)||' docs, '||(select count(*) from corpus_chunks)||' chunks, '||(select count(*) from corpus_chunks where embedding is not null)||' embedded, '||coalesce((select string_agg(distinct coalesce(embedder_model,'none'),'+') from corpus_chunks),'-') from corpus_documents" 2>/dev/null)"
echo "  users: $(cs exec -T postgres psql -U career -d career -At -c "select count(*)||' total, '||count(*) filter (where status='active')||' active' from users" 2>/dev/null)"
echo "  jd: $(cs exec -T postgres psql -U career -d career -At -c "select count(*)||' submissions, last status '||coalesce((select status from jd_submissions order by id desc limit 1),'-') from jd_submissions" 2>/dev/null)"
echo "  deliveries: $(cs exec -T postgres psql -U career -d career -At -c "select count(*)||' total, '||count(*) filter (where error is not null)||' failed' from notification_deliveries" 2>/dev/null)"
U=$(grep ^ADMIN_USERNAME .env.prod | cut -d= -f2); P=$(grep ^CAREER_SITE_ADMIN_PW .env.prod | cut -d= -f2)
CJ=$(mktemp)
code=$(curl -s -o /dev/null -w '%{http_code}' -c "$CJ" -H 'Content-Type: application/json' -X POST "$BASE/api/career.v1.AuthService/Login" -d "$(python3 -c "import json;print(json.dumps({'email':'$U','password':'$P'}))")")
echo "  admin login: http $code"
for rpc in ListMembers ListCorpusDocuments ListJdSubmissions ListMemberActivity; do
  c=$(curl -s -o /dev/null -w '%{http_code}' -b "$CJ" -H 'Content-Type: application/json' -X POST "$BASE/api/career.v1.AdminService/$rpc" -d '{}')
  echo "  admin $rpc: http $c"
done
if [ "$SUBMIT" = 1 ]; then
  echo "  -- JD submission (labelled LIVE TEST, as the admin member)"
  body='{"jdText":"LIVE TEST submission from deploy/live-check.sh. Principal Controls and Industrial AI Engineer for a distilled spirits producer: Rockwell ControlLogix and Ignition standards, OT network design, historian and MQTT Unified Namespace, hub-and-spoke data platform, model predictive control of the still, delivery pipelines with release gates, post-mortems, multi-year digital strategy. 15+ years process manufacturing automation.","source":"JD_SOURCE_PASTE","roleHint":"LIVE TEST","employerHint":"live-check"}'
  resp=$(curl -s --max-time 20 -b "$CJ" -H 'Content-Type: application/json' -X POST "$BASE/api/career.v1.JdService/SubmitJd" -d "$body")
  id=$(echo "$resp" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("submissionId",""))' 2>/dev/null)
  tok=$(echo "$resp" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("resultToken",""))' 2>/dev/null)
  echo "  SubmitJd: id=${id:-none} token=${tok:0:6}..."
  # The LLM pipeline on the CPX31 CPU is one call per requirement plus
  # the résumé: allow up to 25 minutes before calling it stuck.
  status=""; t0=$(date +%s)
  for _ in $(seq 1 100); do
    sleep 15
    status=$(curl -s --max-time 10 -b "$CJ" -H 'Content-Type: application/json' -X POST "$BASE/api/career.v1.JdService/GetJdResult" -d "{\"submissionId\":\"$id\",\"resultToken\":\"$tok\"}" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("status",""), round(d.get("matchScore",0) or 0,3), "pdf" if d.get("generatedResumeUrl") else "no-pdf")' 2>/dev/null)
    case "$status" in JD_STATUS_RECEIVED*|JD_STATUS_SCORING*|JD_STATUS_GENERATING*|"") continue;; *) break;; esac
  done
  echo "  GetJdResult after $(( $(date +%s) - t0 )) s: ${status:-no answer}"
  echo "  llm calls for this JD:"
  cs exec -T postgres psql -U career -d career -At -c "select '    '||prompt_id||' ok='||ok||' prompt='||prompt_tokens||' out='||completion_tokens||' '||(latency_ms/1000)||'s '||coalesce(left(error,60),'') from llm_usage where ref_id=${id:-0} order by id" 2>/dev/null
  echo "  RESULT_JD: ${status:-none}"
fi
rm -f "$CJ"
echo "  host: $(free -m | awk '/Mem/{print $2" MB ram, "$7" MB avail"}'), swap $(free -m | awk '/Swap/{print $2}') MB, disk $(df -h / | awk 'NR==2{print $4" free"}'), up $(uptime -p)"
echo "  restart policy: $(docker inspect $(docker ps -q) --format '{{.HostConfig.RestartPolicy.Name}}' | sort -u | tr '\n' ' ')docker.service=$(systemctl is-enabled docker)"
REMOTE

if [[ $SUBMIT = 1 ]]; then
  jd=$(grep -o 'RESULT_JD: .*' "$REMOTE_OUT" | cut -d' ' -f2-)
  check "JD pipeline finished: ${jd:-no result}" "echo '$jd' | grep -qE 'READY|BELOW_THRESHOLD'"
fi
rm -f "$REMOTE_OUT"

echo "== result: $pass ok, $fail failed =="
[ "$fail" = 0 ]
