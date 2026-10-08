#!/usr/bin/env bash
# Conventional Commit PR title check.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
check="$here/../../../.github/scripts/check-pr-title.sh"
fails=0
expect() {
  bash "$check" "$2" >/dev/null 2>&1; local got=$?
  [[ $got -eq $1 ]] || { echo "FAIL: want $1 got $got: $2"; fails=$((fails+1)); }
}
expect 0 'feat: add panes'
expect 0 'fix(term): handle ConPTY wait'
expect 0 'feat(pkg/api): add session events'
expect 0 'feat(term)!: change Pane interface'
expect 0 'chore: ignore local docs'
expect 1 'Feat: add panes'
expect 1 'feat:add panes'
expect 1 'feature: add panes'
expect 1 'add panes'
expect 1 ''
if [[ $fails -gt 0 ]]; then echo "pr-title: $fails failure(s)"; exit 1; fi
echo "pr-title: ok"
