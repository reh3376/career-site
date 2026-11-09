#!/usr/bin/env bash
# Build a pull request description from every commit the branch carries
# beyond its base, so the description always says what the PR contains.
#
# Why this exists: auto-pr.yml used to write the body once, when it
# opened the PR, as one line per commit. Every push after that changed
# the PR and not the description, so a reviewer read about the first
# commit and merged the third. The owner's rule (2026-11-08): all work
# pushed as part of a PR is documented in its description. Commit
# messages in this repository already explain what and why, so the
# description is assembled from them rather than written twice.
#
# The generated part sits between two HTML-comment markers. On an
# update only that part is replaced, so whatever a person typed outside
# it (the docs-discipline checklist ticks, the test plan, roll-out
# notes) survives the next push.
#
#   scripts/pr_body.sh summary [base] [head]            the marked block only
#   scripts/pr_body.sh full    [base] [head]            block + the PR template
#   scripts/pr_body.sh merge   [base] [head] <bodyfile> existing body with the
#                                                       block replaced, or the
#                                                       block prepended when
#                                                       the body has no markers
#
# Defaults: base origin/main, head HEAD. BRANCH names the branch in the
# header when HEAD is detached (the workflow sets it).
set -euo pipefail

MODE=${1:-full}
BASE=${2:-origin/main}
HEAD=${3:-HEAD}
BODYFILE=${4:-}
TEMPLATE=${PR_TEMPLATE:-.github/PULL_REQUEST_TEMPLATE.md}
START='<!-- auto-pr:summary:start -->'
END='<!-- auto-pr:summary:end -->'

branch_name() {
  if [ -n "${BRANCH:-}" ]; then echo "$BRANCH"; return; fi
  local n; n=$(git rev-parse --abbrev-ref "$HEAD" 2>/dev/null || true)
  [ -n "$n" ] && [ "$n" != "HEAD" ] && echo "$n" || git rev-parse --short "$HEAD"
}

# The commit body without its trailers, with blank lines trimmed at
# both ends. awk rather than sed: the loop that trims trailing blanks
# is written differently in GNU and BSD sed and this runs on both.
commit_body() {
  git log -1 --format='%b' "$1" | awk '
    /^(Co-Authored-By|Co-authored-by|Signed-off-by|Reviewed-by|Change-Id):/ { next }
    { lines[++n] = $0 }
    END {
      s = 1; while (s <= n && lines[s] ~ /^[[:space:]]*$/) s++
      e = n; while (e >= s && lines[e] ~ /^[[:space:]]*$/) e--
      for (i = s; i <= e; i++) print lines[i]
    }'
}

summary() {
  local range="$BASE..$HEAD" count branch
  count=$(git rev-list --count "$range")
  if [ "$count" -eq 0 ]; then
    echo "no commits in $range" >&2
    exit 1
  fi
  branch=$(branch_name)
  echo "$START"
  echo "Opened and kept current by \`.github/workflows/auto-pr.yml\`: this section is rebuilt from the branch's commit messages on every push, so every commit in the PR is summarised here. Anything written outside the markers is kept. Flip to Ready-for-review when the work is done."
  echo
  echo "## Summary"
  echo
  echo "$count commit(s) on \`$branch\` beyond \`${BASE#origin/}\`, oldest first."
  echo
  local c short subject author
  for c in $(git rev-list --reverse "$range"); do
    short=$(git rev-parse --short "$c")
    subject=$(git log -1 --format='%s' "$c")
    author=$(git log -1 --format='%an' "$c")
    echo "### $subject"
    echo
    echo "\`$short\` · $author"
    echo
    commit_body "$c"
    echo
  done
  echo "## Files changed"
  echo
  echo '```'
  # Three-dot: what the branch changed relative to the merge base, not
  # what main did meanwhile. Capped so a wide refactor does not drown
  # the summary; the last line is git's own total.
  git diff --stat=110 "$BASE...$HEAD" | tail -n 60
  echo '```'
  if git log --format='%an' "$range" | grep -q '^Claude'; then
    echo
    echo "🤖 Generated with [Claude Code](https://claude.com/claude-code)"
  fi
  echo "$END"
}

template_rest() {
  # The template minus its HTML comments and minus the "What's in this
  # PR" heading, which the generated summary replaces. The checklist
  # and the sections after it are kept for the author to fill.
  [ -f "$TEMPLATE" ] || return 0
  echo
  echo "---"
  echo
  sed '/<!--/,/-->/d' "$TEMPLATE" | grep -v "^## What's in this PR" | awk 'NR==1 && /^[[:space:]]*$/ {next} {print}'
}

case "$MODE" in
  summary) summary ;;
  full) summary; template_rest ;;
  merge)
    [ -n "$BODYFILE" ] && [ -f "$BODYFILE" ] || { echo "merge needs the existing body as a file" >&2; exit 2; }
    tmp=$(mktemp); trap 'rm -f "$tmp"' EXIT
    summary > "$tmp"
    if grep -qF "$START" "$BODYFILE" && grep -qF "$END" "$BODYFILE"; then
      awk -v f="$tmp" -v start="$START" -v end="$END" '
        BEGIN { while ((getline l < f) > 0) s = s l "\n" }
        index($0, start) == 1 { printf "%s", s; skip = 1; next }
        index($0, end) == 1   { skip = 0; next }
        !skip { print }' "$BODYFILE"
    else
      # A body written before the markers existed: the old one-line
      # commit list is superseded, the rest of the body is kept below.
      cat "$tmp"
      echo
      echo "---"
      echo
      echo "_Description as it stood before the summary above was generated:_"
      echo
      cat "$BODYFILE"
    fi
    ;;
  *) echo "usage: $0 summary|full|merge [base] [head] [bodyfile]" >&2; exit 2 ;;
esac
