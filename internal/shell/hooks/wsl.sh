# Snow launcher for WSL panes. wsl.exe runs this with sh. It finds the user's
# login shell inside the distribution and starts it with Snow's folder hook
# from this same folder. Snow never edits dotfiles.
hooks=$(dirname "$0")
shell=$(getent passwd "$(id -un)" 2>/dev/null | cut -d: -f7)
[ -n "$shell" ] || shell=${SHELL:-/bin/sh}
case "${shell##*/}" in
  bash)
    exec "$shell" --rcfile "$hooks/bash.sh" -i
    ;;
  zsh)
    export SNOW_USER_ZDOTDIR="${ZDOTDIR:-$HOME}"
    export ZDOTDIR="$hooks/zsh"
    exec "$shell" -l
    ;;
  *)
    exec "$shell" -l
    ;;
esac
