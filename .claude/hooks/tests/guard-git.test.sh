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

# A clone whose feature branch tracks origin/main, so a bare push targets main.
git -C "$scratch" init -q --bare -b main remote.git
git -C "$scratch" clone -q remote.git tracking 2>/dev/null
git -C "$scratch/tracking" -c user.name=t -c user.email=t@t commit -q --allow-empty -m init
git -C "$scratch/tracking" push -q origin main
git -C "$scratch/tracking" switch -q -c feat/y --track origin/main

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
expect 2 'gh api -X PUT repos/o/r/pulls/3/merge'

# Blocked: global options and directory changes must not hide a push
expect 2 'git -C . push origin main'
expect 2 'git -C . push --force origin feat/x'
expect 2 'git --no-pager push origin main'
expect 2 "git -C $scratch/onmain push"
expect 2 "cd $scratch/onmain && git push"
expect 2 'git push origin "main"'
expect 2 'git push origin main;'
expect 2 'git push origin main&&echo done'
expect 2 'git push --mirror'
expect 2 'git push --all origin'
expect 2 'git push -uf origin feat/x'
expect 2 'git commit -n -m x'
expect 2 'git config set user.email b@x'
expect 2 'git config --global user.name "Bot Name"'
expect 2 'git push' "$scratch/tracking"
expect 2 'git push origin HEAD' "$scratch/onmain"

# Allowed: reads, messages that mention rules, and pushes of feature branches
expect 0 'git config user.email'
expect 0 'git config --global --get user.email'
expect 0 'git config get user.email'
expect 0 'git commit -m "docs: explain why --no-verify is blocked"'
expect 0 'git commit -m "docs: document the --author rule"'
expect 0 "git commit -F- <<'EOF'
docs: note that git push origin main is blocked
EOF"
expect 0 'git commit -m "$(cat <<EOF
fix: block git push --force
EOF
)"'
expect 0 'git log --grep="git config user.name"'
expect 0 'gh pr create --title x --body "gh pr merge is blocked"'
expect 0 'grep -r "git push" . | grep main'
expect 0 'git push origin feat/x' "$scratch/onmain"
expect 0 "cd $scratch/feature && git push -u origin feat/x" "$scratch/onmain"
expect 0 "git -C $scratch/feature push -u origin feat/x" "$scratch/onmain"
expect 0 'git push origin feat/x && echo main'

# Missing jq blocks with a reason
out=$(echo '{}' | PATH=/nonexistent /bin/bash "$hook" 2>&1); code=$?
if [[ $code -ne 2 || "$out" != *"install jq"* ]]; then
  echo "FAIL: missing jq should block with 'install jq', got $code: $out"
  fails=$((fails + 1))
fi

if [[ $fails -gt 0 ]]; then echo "guard-git: $fails failure(s)"; exit 1; fi
echo "guard-git: ok"
