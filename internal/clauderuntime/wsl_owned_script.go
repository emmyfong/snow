package clauderuntime

// wslOwnedScript relays the protocol input into a fresh Linux session. The
// leader stays alive until cleanup so its group ID cannot be reused while we
// signal it. The runtime resets TERM; only the supervising leader ignores it.
// Closing Snow's input ends the relay and starts cleanup even without a control
// reply. A receipt means no non-zombie process remains in that owned group.
const wslOwnedScript = `set -eu
run=$1
shift
started=0
trap 'if [ "$started" = 0 ]; then printf "not-started\n" > "$run/receipt"; fi' 0
for utility in setsid mkfifo ps awk cat sleep mv; do
 command -v "$utility" >/dev/null 2>&1 || exit 78
done
mkfifo "$run/input"
exec 3<&0
setsid sh -c '
set -eu
run=$1
shift
trap "" TERM
exec 4<&0
(trap - TERM; exec "$@") <&4 &
child=$!
printf "%s\n" "$$" > "$run/leader"
exec 1>&-
wait "$child" || :
 : > "$run/exited"
while :; do sleep 1; done
' snow-owned "$run" "$@" < "$run/input" &
leader=$!
started=1
exec 1>&-
cat <&3 > "$run/input" &
relay=$!
while kill -0 "$relay" 2>/dev/null; do
 if [ -f "$run/exited" ]; then
  kill "$relay" 2>/dev/null || :
  break
 fi
 sleep .01
done
wait "$relay" 2>/dev/null || :
count=0
while [ ! -f "$run/leader" ]; do
 count=$((count+1))
 [ "$count" -lt 100 ] || exit 78
 sleep .01
done
[ "$(cat "$run/leader")" = "$leader" ] || exit 78
kill -TERM "-$leader"
sleep .1
kill -KILL "-$leader"
wait "$leader" 2>/dev/null || :
count=0
while :; do
 states=$(ps -eo pgid=,stat=) || exit 78
 live=$(printf '%s\n' "$states" | awk -v group="$leader" '$1 == group && $2 !~ /^Z/ {print "live"}')
 [ -n "$live" ] || break
 count=$((count+1))
 [ "$count" -lt 100 ] || exit 78
 sleep .01
done
printf 'stopped\n' > "$run/receipt.tmp"
mv "$run/receipt.tmp" "$run/receipt"`

// wslReceiptScript checks only a fresh launch's receipt. It never guesses which
// processes to signal from a saved PID, folder, or executable name.
const wslReceiptScript = `set -eu
run="$HOME/` + wslConfigBase + `/$1/runs/$2"
count=0
while [ ! -f "$run/receipt" ]; do
 count=$((count+1))
 [ "$count" -lt 100 ] || exit 78
 sleep .05
done
case $(cat "$run/receipt") in
stopped|not-started) rm -rf -- "$run" ;;
*) exit 78 ;;
esac`
