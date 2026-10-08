#!/usr/bin/env bash
# Runs every hook test in this folder.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
status=0
for t in "$here"/*.test.sh; do
  bash "$t" || status=1
done
exit $status
