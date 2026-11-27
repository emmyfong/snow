#!/usr/bin/env bash
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
summary="$here/../ci-job-summary.sh"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

cat > "$tmp/failed.json" <<'JSON'
{"jobs":[
  {"name":"test (ubuntu-latest)","conclusion":"failure","html_url":"https://example.test/ubuntu"},
  {"name":"test (macos-latest)","conclusion":"cancelled","html_url":"https://example.test/macos"},
  {"name":"test (windows-latest)","conclusion":"skipped","html_url":"https://example.test/windows"},
  {"name":"lint","conclusion":"success","html_url":"https://example.test/lint"}
]}
JSON

if output=$(bash "$summary" "$tmp/failed.json"); then
	echo "expected failed and cancelled jobs to return status 1" >&2
	exit 1
fi
for expected in \
	"test (ubuntu-latest) (failure): https://example.test/ubuntu" \
	"test (macos-latest) (cancelled): https://example.test/macos"; do
	if [[ $output != *"$expected"* ]]; then
		echo "summary missing '$expected': $output" >&2
		exit 1
	fi
done
if [[ $output == *"test (windows-latest)"* || $output == *"lint"* ]]; then
	echo "summary included a skipped or successful job: $output" >&2
	exit 1
fi

cat > "$tmp/success.json" <<'JSON'
{"jobs":[
  {"name":"test (windows-latest)","conclusion":"success","html_url":"https://example.test/windows"},
  {"name":"pr-title","conclusion":"skipped","html_url":"https://example.test/title"}
]}
JSON

output=$(bash "$summary" "$tmp/success.json")
if [[ $output != "No CI jobs failed or were cancelled." ]]; then
	echo "unexpected success summary: $output" >&2
	exit 1
fi

echo "ci-job-summary: ok"
