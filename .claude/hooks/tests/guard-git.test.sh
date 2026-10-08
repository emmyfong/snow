#!/usr/bin/env bash
# Table test for guard-git.sh: feeds hook JSON and checks the exit code.
set -uo pipefail

here=$(cd "$(dirname "$0")" && pwd)
hook="$here/../guard-git.sh"
fails=0

# A scratch repo whose current branch is a feature branch, and one on main.
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
git -C "$scratch" init -q -b feat/x feature
git -C "$scratch" init -q -b main onmain

expect() {
  local want=$1 cmd=$2 cwd=${3:-$scratch/feature}
  local got
  jq -n --arg c "$cmd" --arg d "$cwd" '{tool_name:"Bash",tool_input:{command:$c},cwd:$d}' \
    | bash "$hook" >/dev/null 2>&1
  got=$?
  if [[ $got -ne $want ]]; then
    echo "FAIL: want $want got $got: $cmd (cwd $cwd)"
    fails=$((fails + 1))
  fi
}

# Allowed
expect 0 'git status'
expect 0 'git commit -m "feat: x"'
expect 0 'git log --author=emmy'
expect 0 'git config --get user.email'
expect 0 'git push -u origin feat/x'
expect 0 'git push origin feat/main-menu'
expect 0 'gh pr view 3'
expect 0 'ls -la'

# Blocked: authorship
expect 2 'git commit --author="Bot <b@x>" -m x'
expect 2 'git -c user.email=b@x commit -m x'
expect 2 'git config user.email b@x'
expect 2 'git config --global user.name Bot'
expect 2 'GIT_AUTHOR_NAME=Bot git commit -m x'
expect 2 'GIT_COMMITTER_EMAIL=b@x git commit -m x'
expect 2 'git commit --no-verify -m x'

# Blocked: history and main
expect 2 'git push --force origin feat/x'
expect 2 'git push -f origin feat/x'
expect 2 'git push --force-with-lease'
expect 2 'git push origin +feat/x'
expect 2 'git push origin main'
expect 2 'git push origin HEAD:main'
expect 2 'git push origin refs/heads/main'
expect 2 'cd /tmp && git push origin main'
expect 2 'git push' "$scratch/onmain"
expect 2 'git push -u origin' "$scratch/onmain"

# Blocked: only the user merges
expect 2 'gh pr merge 3 --squash'

# Missing jq blocks with a reason
out=$(echo '{}' | PATH=/nonexistent /bin/bash "$hook" 2>&1); code=$?
if [[ $code -ne 2 || "$out" != *"install jq"* ]]; then
  echo "FAIL: missing jq should block with 'install jq', got $code: $out"
  fails=$((fails + 1))
fi

if [[ $fails -gt 0 ]]; then echo "guard-git: $fails failure(s)"; exit 1; fi
echo "guard-git: ok"
