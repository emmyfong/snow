__snow_zdotdir=$ZDOTDIR
ZDOTDIR=${SNOW_USER_ZDOTDIR:-$HOME}
[[ -f $ZDOTDIR/.zprofile ]] && source $ZDOTDIR/.zprofile
ZDOTDIR=$__snow_zdotdir
