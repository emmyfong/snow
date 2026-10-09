# Snow folder hook for bash. Snow starts bash with --rcfile pointing here.
# Load the user's startup files as a login shell would, then report the
# current folder with OSC 7 before each prompt. Snow never edits dotfiles.
if [ -f "$HOME/.bash_profile" ]; then
  . "$HOME/.bash_profile"
elif [ -f "$HOME/.bash_login" ]; then
  . "$HOME/.bash_login"
elif [ -f "$HOME/.profile" ]; then
  . "$HOME/.profile"
elif [ -f "$HOME/.bashrc" ]; then
  . "$HOME/.bashrc"
fi
__snow_osc7() { printf '\033]7;file://%s%s\033\\' "${HOSTNAME:-localhost}" "$PWD"; }
case ";${PROMPT_COMMAND:-};" in
  *";__snow_osc7;"*) ;;
  *) PROMPT_COMMAND="__snow_osc7${PROMPT_COMMAND:+;$PROMPT_COMMAND}" ;;
esac
