# Snow folder hook for zsh. Snow points ZDOTDIR here; each file loads the
# user's file of the same name from their real ZDOTDIR, then hands control
# back so zsh reads the next Snow file. Snow never edits dotfiles.
__snow_zdotdir=$ZDOTDIR
ZDOTDIR=${SNOW_USER_ZDOTDIR:-$HOME}
[[ -f $ZDOTDIR/.zshenv ]] && source $ZDOTDIR/.zshenv
ZDOTDIR=$__snow_zdotdir
