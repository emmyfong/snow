#!/usr/bin/env bash
# load-context.sh must read CONTEXT.md from the worktree that cwd is in.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
hook="$here/../load-context.sh"
fails=0
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT

git -C "$scratch" init -q -b main repo
git -C "$scratch/repo" -c user.name=t -c user.email=t@t commit -q --allow-empty -m init
git -C "$scratch/repo" worktree add -q -b feat/x "$scratch/repo/.worktrees/feat-x"
mkdir -p "$scratch/repo/.context" "$scratch/repo/.worktrees/feat-x/.context" "$scratch/repo/.worktrees/feat-x/sub"
echo "MAIN CONTEXT" > "$scratch/repo/.context/CONTEXT.md"
echo "WORKTREE CONTEXT" > "$scratch/repo/.worktrees/feat-x/.context/CONTEXT.md"

run() { jq -n --arg d "$1" '{cwd:$d}' | bash "$hook"; }

[[ "$(run "$scratch/repo")" == *"MAIN CONTEXT"* ]] || { echo "FAIL: main checkout"; fails=$((fails+1)); }
out=$(run "$scratch/repo/.worktrees/feat-x/sub")
[[ "$out" == *"WORKTREE CONTEXT"* && "$out" != *"MAIN CONTEXT"* ]] || { echo "FAIL: worktree got: $out"; fails=$((fails+1)); }
[[ -z "$(run "$scratch")" ]] || { echo "FAIL: non-repo should print nothing"; fails=$((fails+1)); }

if [[ $fails -gt 0 ]]; then echo "load-context: $fails failure(s)"; exit 1; fi
echo "load-context: ok"
