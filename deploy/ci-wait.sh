#!/usr/bin/env bash
# Wait for a CI run to finish, with a bound and a diagnosis.
#
#   deploy/ci-wait.sh                 # the newest ci run on the current branch
#   deploy/ci-wait.sh main            # ... on a branch
#   deploy/ci-wait.sh main 37362565239  # ... a specific run
#
# Exit codes, so a caller can branch on the reason rather than on text:
#   0  the run succeeded
#   1  the run finished and did not succeed
#   2  the wait timed out with jobs still queued (not a code failure)
#   3  no run found yet
#
# **Why this exists.** The obvious thing to write is
#
#     until [ "$(gh run view "$R" --json status -q .status)" = completed ]; do sleep 30; done
#
# and it is wrong in the one case that matters. On 2026-10-05 GitHub had
# an incident assigning hosted runners. A job sat in `queued` for
# fifteen minutes having executed no steps, the run never reached
# `completed`, and every loop of that shape span silently for as long as
# anyone let it. Worse, the symptom reads exactly like a hung test
# suite, so an hour went into looking for a fault in the repository
# before anybody looked at githubstatus.com.
#
# So this bounds the wait, and when it gives up it says WHICH jobs are
# still waiting and that a queued job with no steps is a runner
# assignment problem rather than anything in the code. That sentence is
# the whole value of the script.
set -euo pipefail

BRANCH=${1:-$(git rev-parse --abbrev-ref HEAD)}
RUN=${2:-}
# 25 minutes by default. Long enough for a cold cache on the slowest
# job observed (images, about 6 minutes) with room to spare, short
# enough that a stuck run is reported inside a coffee break.
TIMEOUT_S=${CI_WAIT_TIMEOUT:-1500}
INTERVAL_S=${CI_WAIT_INTERVAL:-20}

say() { printf '\n\033[1m== %s\033[0m\n' "$*"; }

if [ -z "$RUN" ]; then
  RUN=$(gh run list --branch "$BRANCH" --workflow ci --limit 1 --json databaseId -q '.[0].databaseId' 2>/dev/null || true)
fi
if [ -z "$RUN" ] || [ "$RUN" = "null" ]; then
  echo "no ci run found for $BRANCH yet" >&2
  exit 3
fi

# Refuse to wait on a run for a different commit.
#
# `gh run list` served a stale page once while writing this, handing
# back a run from eighteen hours earlier. That one had succeeded, so an
# unchecked wait would have reported success for a commit CI had never
# seen, which is the worst possible failure for a gate: it is wrong in
# the direction of proceeding. Only skipped when a run id was passed
# explicitly, since then the caller has already chosen.
if [ -z "${2:-}" ]; then
  RUN_SHA=$(gh run view "$RUN" --json headSha -q .headSha)
  TIP_SHA=$(git rev-parse "origin/$BRANCH" 2>/dev/null || git rev-parse "$BRANCH")
  if [ "$RUN_SHA" != "$TIP_SHA" ]; then
    echo "run $RUN is for ${RUN_SHA:0:7}, but $BRANCH is at ${TIP_SHA:0:7}" >&2
    echo "either the run for the tip has not been created yet, or the listing is stale" >&2
    echo "re-run this in a moment, or pass the run id explicitly" >&2
    exit 3
  fi
fi

say "waiting on ci run $RUN ($BRANCH), up to $((TIMEOUT_S / 60)) minutes"
START=$(date +%s)

while :; do
  STATUS=$(gh run view "$RUN" --json status -q .status)
  [ "$STATUS" = "completed" ] && break

  ELAPSED=$(( $(date +%s) - START ))
  if [ "$ELAPSED" -ge "$TIMEOUT_S" ]; then
    say "gave up after $((ELAPSED / 60))m: the run is still $STATUS"

    # The diagnosis. A job that is queued with no steps never got a
    # runner; a job that is in_progress is genuinely slow. Those need
    # opposite responses and look identical in a run summary.
    gh run view "$RUN" --json jobs \
      -q '.jobs[] | select(.conclusion == null) | "  \(.status)  \(.name)"' || true

    cat <<'WHY'

  A job sitting in `queued` has executed no steps, so nothing in this
  repository can be the cause: it is waiting to be given a runner.
  Check https://www.githubstatus.com before changing anything, and
  prefer waiting over re-running, because a re-run queues into the same
  assignment path and discards the evidence.

  A job in `in_progress` for this long is the opposite case and is worth
  reading the log for.
WHY
    exit 2
  fi
  sleep "$INTERVAL_S"
done

CONCLUSION=$(gh run view "$RUN" --json conclusion -q .conclusion)
SHA=$(gh run view "$RUN" --json headSha -q '.headSha[0:7]')

if [ "$CONCLUSION" = "success" ]; then
  say "ci success on $SHA"
  exit 0
fi

say "ci $CONCLUSION on $SHA"
gh run view "$RUN" --json jobs -q '.jobs[] | select(.conclusion != "success") | "  \(.name): \(.conclusion)"'

# cancelled is worth separating from failed. It usually means either a
# newer push superseded this run, which is the concurrency group doing
# its job, or a job was never assigned a runner and GitHub gave up on
# it. Neither is a test failure, and reading it as one sends people
# looking for a bug that is not there.
if [ "$CONCLUSION" = "cancelled" ]; then
  cat <<'WHY'

  Cancelled, not failed. Either a newer commit superseded this run
  (expected: the concurrency group cancels in progress) or a job was
  never given a runner. Check whether the head sha above is still the
  tip before investigating anything else.
WHY
fi
exit 1
