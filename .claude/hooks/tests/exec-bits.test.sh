#!/usr/bin/env bash
# Hook scripts must be executable in the git index. A checkout on a Windows
# drive hides a missing exec bit, but a fresh clone then fails every hook.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
root=$(git -C "$here" rev-parse --show-toplevel)
fails=0
while read -r mode _ _ path; do
  [[ "$mode" == 100755 ]] || { echo "FAIL: $path is $mode, want 100755"; fails=$((fails + 1)); }
done < <(git -C "$root" ls-files -s -- '.claude/hooks/*.sh' '.github/scripts/*.sh')
if [[ $fails -gt 0 ]]; then echo "exec-bits: $fails failure(s)"; exit 1; fi
echo "exec-bits: ok"
