//go:build linux || windows

package tool

// wslResolveScript canonicalizes each path argument inside a WSL distro and
// writes it followed by a NUL byte. readlink -m is GNU-only and BusyBox has
// no -m, so the script resolves the longest existing prefix with readlink -f
// and follows a dangling link by hand. A missing component is kept, so a
// write can create it. Only a followed link counts against the link limit, so
// a deep new path still resolves.
const wslResolveScript = `
resolve() {
  p=$1
  rest=
  n=0
  while :; do
    if t=$(readlink -f "$p" 2>/dev/null); then
      printf '%s%s\0' "$t" "$rest"
      return 0
    fi
    if [ -L "$p" ]; then
      n=$((n+1))
      if [ "$n" -gt 40 ]; then
        printf 'too many links\n' >&2
        return 1
      fi
      t=$(readlink "$p") || return 1
      case $t in
        /*) p=$t ;;
        *) p=$(dirname "$p")/$t ;;
      esac
      continue
    fi
    if [ "$p" = / ]; then
      printf '/%s\0' "${rest#/}"
      return 0
    fi
    rest=/${p##*/}$rest
    p=$(dirname "$p")
  done
}

for path in "$@"; do
  resolve "$path" || exit 1
done
`
