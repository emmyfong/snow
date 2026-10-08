#!/usr/bin/env bash
# SessionStart hook. Prints this working copy's .context/CONTEXT.md so a new
# or compacted session starts from the handoff instead of the full history.
set -uo pipefail
command -v jq >/dev/null 2>&1 || exit 0
cwd=$(jq -r '.cwd // empty')
top=$(git -C "${cwd:-$PWD}" rev-parse --show-toplevel 2>/dev/null) || exit 0
file="$top/.context/CONTEXT.md"
[[ -f "$file" ]] || exit 0
echo "Working memory from $file (read the Handoff section first):"
echo
cat "$file"
