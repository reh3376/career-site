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

echo "-- public endpoints"
for path in / /jd-upload /articles /login /contact /register; do
  code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 20 "$BASE$path")
  check "GET $path -> $code" "[ '$code' = '200' ]"
done
hz=$(curl -s --max-time 10 "$BASE/api/healthz"); check "healthz: $hz" "echo '$hz' | grep -q '\"ok\"'"
rz=$(curl -s --max-time 10 "$BASE/api/readyz");  check "readyz: $rz"   "echo '$rz' | grep -q '\"postgres\":true' && echo '$rz' | grep -q '\"sidecar\":true'"
code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$BASE/api/jd/resume/1.pdf"); check "pdf route refuses without token -> $code" "[ '$code' = '403' ] || [ '$code' = '404' ]"

if [[ $SUBMIT = 1 ]]; then
  echo "-- JD submission (labelled LIVE TEST)"
  body='{"jdText":"LIVE TEST submission from deploy/live-check.sh. Principal Controls and Industrial AI Engineer for a distilled spirits producer: Rockwell ControlLogix and Ignition standards, OT network design, historian and MQTT Unified Namespace, hub-and-spoke data platform, model predictive control of the still, delivery pipelines with release gates, post-mortems, multi-year digital strategy. 15+ years process manufacturing automation.","source":"JD_SOURCE_PASTE","roleHint":"LIVE TEST","employerHint":"live-check"}'
  resp=$(curl -s --max-time 20 -H 'Content-Type: application/json' -X POST "$BASE/api/career.v1.JdService/SubmitJd" -d "$body")
  id=$(echo "$resp" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("submissionId",""))' 2>/dev/null)
  tok=$(echo "$resp" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("resultToken",""))' 2>/dev/null)
  check "SubmitJd returned id=$id token=${tok:0:6}..." "[ -n '$id' ] && [ -n '$tok' ]"
  status=""
  for _ in $(seq 1 30); do
    sleep 4
    status=$(curl -s --max-time 10 -H 'Content-Type: application/json' -X POST "$BASE/api/career.v1.JdService/GetJdResult" -d "{\"submissionId\":\"$id\",\"resultToken\":\"$tok\"}" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("status",""), round(d.get("matchScore",0) or 0,3))' 2>/dev/null)
    case "$status" in JD_STATUS_RECEIVED*|JD_STATUS_SCORING*|"") continue;; *) break;; esac
  done
  check "GetJdResult reached a terminal state: $status" "echo '$status' | grep -qE 'BELOW_THRESHOLD|GENERATING|READY|FAILED'"
fi

echo "-- server side (over ssh)"
ssh -o StrictHostKeyChecking=no -o ConnectTimeout=15 "$HOST" "BASE='$BASE' bash -s" <<'REMOTE'
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
rm -f "$CJ"
echo "  host: $(free -m | awk '/Mem/{print $2" MB ram, "$7" MB avail"}'), swap $(free -m | awk '/Swap/{print $2}') MB, disk $(df -h / | awk 'NR==2{print $4" free"}'), up $(uptime -p)"
echo "  restart policy: $(docker inspect $(docker ps -q) --format '{{.HostConfig.RestartPolicy.Name}}' | sort -u | tr '\n' ' ')docker.service=$(systemctl is-enabled docker)"
REMOTE

echo "== result: $pass ok, $fail failed =="
[ "$fail" = 0 ]
