# Snow launcher for Claude Code panes in WSL. wsl.exe runs this with sh. It
# starts claude through the user's interactive login shell, so the PATH
# from .profile or .bashrc (nvm, ~/.local/bin) applies. Snow never edits
# dotfiles. The arguments of this script go to claude: fish reads them as
# $argv, other shells as "$@" after $0.
shell=$(getent passwd "$(id -un)" 2>/dev/null | cut -d: -f7)
[ -n "$shell" ] || shell=${SHELL:-/bin/sh}
case $shell in
*fish) exec "$shell" -lic 'exec claude $argv' "$@" ;;
*) exec "$shell" -lic 'exec claude "$@"' claude "$@" ;;
esac
