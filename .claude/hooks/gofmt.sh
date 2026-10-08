#!/usr/bin/env bash
# PostToolUse hook for Edit and Write. Formats edited Go files so formatting
# never reaches review or CI.
set -uo pipefail
command -v jq >/dev/null 2>&1 || exit 0
command -v gofmt >/dev/null 2>&1 || exit 0
file=$(jq -r '.tool_input.file_path // empty')
[[ "$file" == *.go && -f "$file" ]] && gofmt -w "$file"
exit 0
