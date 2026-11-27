#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 || ! -r $1 ]]; then
	echo "usage: ci-job-summary.sh <jobs.json>" >&2
	exit 2
fi

jobs_file=$1
if ! jq -e '.jobs | type == "array"' "$jobs_file" >/dev/null; then
	echo "CI jobs response must contain a jobs array" >&2
	exit 2
fi

failures=$(jq -r '
	.jobs[]
	| select(.conclusion == "failure" or .conclusion == "cancelled")
	| "- \(.name) (\(.conclusion)): \(.html_url)"
' "$jobs_file")

if [[ -n $failures ]]; then
	printf 'The following CI jobs failed or were cancelled:\n%s\n' "$failures"
	exit 1
fi

echo "No CI jobs failed or were cancelled."
