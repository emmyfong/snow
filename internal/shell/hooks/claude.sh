# Snow launcher for Claude Code panes in WSL. wsl.exe runs this with sh. It
# starts claude through the user's interactive login shell, so the PATH
# from .profile or .bashrc (nvm, ~/.local/bin) applies. Snow never edits
# dotfiles.
shell=$(getent passwd "$(id -un)" 2>/dev/null | cut -d: -f7)
[ -n "$shell" ] || shell=${SHELL:-/bin/sh}
exec "$shell" -lic 'exec claude'
