#!/usr/bin/env bash
# PreToolUse hook for Bash. Blocks git and gh commands that break Snow's
# authorship and merge rules (AGENTS.md: Authorship, Git). Exit 2 blocks.
set -uo pipefail

if ! command -v jq >/dev/null 2>&1; then
  echo "guard-git: jq is missing, so git commands cannot be checked. Ask the user to install jq (sudo apt install jq)." >&2
  exit 2
fi

input=$(cat)
cmd=$(jq -r '.tool_input.command // empty' <<<"$input")
cwd=$(jq -r '.cwd // empty' <<<"$input")
[[ -z "$cmd" ]] && exit 0

block() {
  echo "guard-git: blocked ($1). See AGENTS.md, Authorship and Git. Do not work around this; tell the user." >&2
  exit 2
}

s='[[:space:]]'

[[ "$cmd" =~ GIT_(AUTHOR|COMMITTER)_(NAME|EMAIL) ]] && block "author or committer override"
[[ "$cmd" =~ git$s.*-c$s*(user|author|committer)\. ]] && block "identity override with -c"
[[ "$cmd" =~ git$s+commit.*--author ]] && block "commit --author"
[[ "$cmd" =~ git$s.*--no-verify ]] && block "--no-verify"
if [[ "$cmd" =~ git$s+config$s.*(user|author|committer)\. ]] \
  && ! [[ "$cmd" =~ git$s+config$s+(--get|--get-all|--list|-l)($s|$) ]]; then
  block "git config identity change"
fi
[[ "$cmd" =~ gh$s+pr$s+merge ]] && block "gh pr merge; only the user merges"

if [[ "$cmd" =~ git$s+push ]]; then
  [[ "$cmd" =~ git$s+push.*($s-f($s|$)|--force|$s\+[^[:space:]]) ]] && block "force push"
  [[ "$cmd" =~ git$s+push.*($s|:)(refs/heads/)?main($s|$) ]] && block "push to main"
  dir=${cwd:-$PWD}
  branch=$(git -C "$dir" branch --show-current 2>/dev/null || true)
  [[ "$branch" == "main" ]] && block "push while on main"
fi

exit 0
