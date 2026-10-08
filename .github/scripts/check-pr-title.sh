#!/usr/bin/env bash
# Fails unless $1 is a Conventional Commit title. The PR title becomes the
# squash commit on main.
set -uo pipefail
title=${1-}
re='^(feat|fix|refactor|test|docs|chore|ci|perf)(\([a-z0-9/_-]+\))?!?: [^[:space:]].*$'
if [[ "$title" =~ $re ]]; then exit 0; fi
echo "PR title is not a Conventional Commit: '$title'" >&2
echo "Use: <type>(<scope>): <summary>, type one of feat fix refactor test docs chore ci perf." >&2
exit 1
