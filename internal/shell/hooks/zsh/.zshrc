# Load the user's .zshrc, add the OSC 7 hook, and leave ZDOTDIR as the user's
# so zsh reads their .zlogin and later sessions see their own setting.
ZDOTDIR=${SNOW_USER_ZDOTDIR:-$HOME}
unset __snow_zdotdir
[[ -f $ZDOTDIR/.zshrc ]] && source $ZDOTDIR/.zshrc
__snow_osc7() { printf '\033]7;file://%s%s\033\\' "${HOST:-localhost}" "$PWD" }
autoload -Uz add-zsh-hook
add-zsh-hook precmd __snow_osc7
