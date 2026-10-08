#!/usr/bin/env bash
# PreToolUse hook for Bash. Blocks git and gh commands that break Snow's
# authorship and merge rules (AGENTS.md: Authorship, Git). Exit 2 blocks.
#
# The command is lexed like a shell would: split into segments on ; & | and
# newlines, with quoted prose and heredoc bodies replaced by a placeholder, so
# a commit message that mentions a rule never trips it. Each git or gh segment
# is then checked by its real arguments, in the directory it would run in.
set -uo pipefail

if ! command -v jq >/dev/null 2>&1; then
  echo "guard-git: jq is missing, so git commands cannot be checked. Ask the user to install jq (sudo apt install jq)." >&2
  exit 2
fi

input=$(cat)
cmd=$(jq -r '.tool_input.command // empty' <<<"$input")
cwd=$(jq -r '.cwd // empty' <<<"$input")
[[ "$cmd" == *git* || "$cmd" == *gh* || "$cmd" == *GIT_* ]] || exit 0

block() {
  echo "guard-git: blocked ($1). See AGENTS.md, Authorship and Git. Do not work around this; tell the user." >&2
  exit 2
}

# Quoted text that contains whitespace is prose (a message, a body, a grep
# pattern), never a ref or a config key. It becomes this placeholder.
PROSE=__prose__
US=$'\x1f'

segments=()
seg=""
tok=""
intok=0
heredocs=()

flush() {
  if ((intok)); then seg+="$tok$US"; fi
  tok=""
  intok=0
}
endseg() {
  flush
  [[ -n "$seg" ]] && segments+=("$seg")
  seg=""
}

n=${#cmd}
i=0
while ((i < n)); do
  c=${cmd:i:1}
  case "$c" in
    "'" | '"')
      j=$((i + 1))
      part=""
      while ((j < n)) && [[ "${cmd:j:1}" != "$c" ]]; do
        if [[ "$c" == '"' && "${cmd:j:1}" == '\' ]]; then
          part+=${cmd:j+1:1}
          j=$((j + 2))
          continue
        fi
        part+=${cmd:j:1}
        j=$((j + 1))
      done
      if [[ "$part" =~ [[:space:]] ]]; then tok+=$PROSE; else tok+=$part; fi
      intok=1
      i=$((j + 1))
      ;;
    ' ' | $'\t')
      flush
      i=$((i + 1))
      ;;
    $'\n')
      endseg
      i=$((i + 1))
      # Skip heredoc bodies that start after this line.
      for d in "${heredocs[@]}"; do
        while ((i < n)); do
          rest=${cmd:i}
          line=${rest%%$'\n'*}
          i=$((i + ${#line} + 1))
          [[ "${line#"${line%%[!$'\t']*}"}" == "$d" ]] && break
        done
      done
      heredocs=()
      ;;
    ';' | '&' | '|')
      endseg
      i=$((i + 1))
      ;;
    '<')
      if [[ "${cmd:i:2}" == '<<' && "${cmd:i:3}" != '<<<' ]]; then
        flush
        j=$((i + 2))
        [[ "${cmd:j:1}" == '-' ]] && j=$((j + 1))
        while [[ "${cmd:j:1}" == ' ' ]]; do j=$((j + 1)); done
        d=""
        while ((j < n)) && [[ ! "${cmd:j:1}" =~ [[:space:]\;\&\|\)] ]]; do
          d+=${cmd:j:1}
          j=$((j + 1))
        done
        d=${d//\'/}
        d=${d//\"/}
        heredocs+=("$d")
        i=$j
      else
        tok+=$c
        intok=1
        i=$((i + 1))
      fi
      ;;
    '\')
      tok+=${cmd:i+1:1}
      intok=1
      i=$((i + 2))
      ;;
    *)
      tok+=$c
      intok=1
      i=$((i + 1))
      ;;
  esac
done
endseg

resolve() { # resolve <base> <path>
  local p=$2
  [[ "$p" == "~"* ]] && p="$HOME${p#\~}"
  [[ "$p" == /* ]] && echo "$p" || echo "$1/$p"
}

identity_key() { [[ "${1,,}" =~ ^(user|author|committer)\. ]]; }

check_config() {
  local a key=0 read=0 write=0 after=0
  for a in "$@"; do
    if ((key)); then
      [[ "$a" != -* ]] && after=1
      continue
    fi
    case "$a" in
      set | unset | --unset | --unset-all | --add | --replace-all | --rename-section | --remove-section | --edit | -e) write=1 ;;
      get | list | --get | --get-all | --get-regexp | --list | -l) read=1 ;;
      *) identity_key "$a" && key=1 ;;
    esac
  done
  ((key)) || return 0
  ((write)) && block "git config identity change"
  ((read)) && return 0
  ((after)) && block "git config identity change"
  return 0
}

check_push() { # check_push <dir> <args...>
  local dir=$1 a r dst skip=0
  shift
  local pos=()
  for a in "$@"; do
    if ((skip)); then
      skip=0
      continue
    fi
    case "$a" in
      --force | --force-with-lease | --force-with-lease=* | --force-if-includes) block "force push" ;;
      --mirror | --all | --branches) block "push of every branch, including main" ;;
      -o | --push-option | --repo | --receive-pack | --exec) skip=1 ;;
      --*) ;;
      -*) [[ "$a" == *f* ]] && block "force push" ;;
      *) pos+=("$a") ;;
    esac
  done
  local branch
  branch=$(git -C "$dir" branch --show-current 2>/dev/null || true)
  if ((${#pos[@]} <= 1)); then
    [[ "$branch" == "main" ]] && block "push while on main"
    local up
    up=$(git -C "$dir" rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>/dev/null || true)
    [[ "$up" == */main ]] && block "push to an upstream of main"
    return 0
  fi
  for r in "${pos[@]:1}"; do
    [[ "$r" == +* ]] && block "force push"
    dst=${r##*:}
    dst=${dst#refs/heads/}
    [[ "$dst" == "main" ]] && block "push to main"
    [[ "$dst" == "HEAD" && "$branch" == "main" ]] && block "push while on main"
  done
}

check_git() { # check_git <dir> <args...>
  local dir=$1
  shift
  while (($#)); do
    case "$1" in
      -C)
        dir=$(resolve "$dir" "${2-}")
        shift 2
        ;;
      -c)
        identity_key "${2-}" && block "identity override with -c"
        shift 2
        ;;
      -c*)
        identity_key "${1#-c}" && block "identity override with -c"
        shift
        ;;
      --config-env=*)
        identity_key "${1#--config-env=}" && block "identity override with --config-env"
        shift
        ;;
      --git-dir | --work-tree | --namespace) shift 2 ;;
      -*) shift ;;
      *) break ;;
    esac
  done
  local sub=${1-}
  shift || true
  local a
  case "$sub" in
    commit)
      for a in "$@"; do
        case "$a" in
          --author | --author=*) block "commit --author" ;;
          --no-verify | -n) block "--no-verify" ;;
        esac
      done
      ;;
    config) check_config "$@" ;;
    push) check_push "$dir" "$@" ;;
  esac
  for a in "$@"; do
    [[ "$a" == --no-verify ]] && block "--no-verify"
  done
}

check_gh() {
  local prev="" a
  for a in "$@"; do
    [[ "$prev" == pr && "$a" == merge ]] && block "gh pr merge; only the user merges"
    [[ "$a" == */pulls/*/merge ]] && block "PR merge through the API; only the user merges"
    prev=$a
  done
}

dir=${cwd:-$PWD}
for s in "${segments[@]}"; do
  IFS=$US read -r -a t <<<"$s"
  for a in "${t[@]}"; do
    [[ "$a" =~ ^GIT_(AUTHOR|COMMITTER)_(NAME|EMAIL)= ]] && block "author or committer override"
  done
  k=0
  while ((k < ${#t[@]})) && [[ "${t[k]}" =~ ^[A-Za-z_][A-Za-z0-9_]*= || "${t[k]}" == env || "${t[k]}" == command ]]; do
    k=$((k + 1))
  done
  ((k < ${#t[@]})) || continue
  case "${t[k]}" in
    cd) dir=$(resolve "$dir" "${t[k + 1]:-$HOME}") ;;
    git) check_git "$dir" "${t[@]:k+1}" ;;
    gh) check_gh "${t[@]:k+1}" ;;
  esac
done

exit 0
