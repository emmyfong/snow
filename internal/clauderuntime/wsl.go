package clauderuntime

import (
	"path"
	"slices"
	"strings"

	"github.com/emmyfong/snow/internal/shell"
)

// wslConfigBase is a WSL connection's folder under the distribution user's
// home. Linux permissions keep it private there, and the Windows side never
// holds the login.
const wslConfigBase = ".local/state/snow/runtimes"

// wslVar prefixes the variables Snow forwards into a WSL distribution. The
// script turns each back into its real name only after it removed every
// inherited Claude and Anthropic variable, so nothing the user's Windows
// environment or WSLENV holds reaches the runtime.
const wslVar = "SNOW_RT_"

// wslScript starts Claude inside the distribution. It finds claude the way
// the user's login shell does (nvm, ~/.local/bin), but runs it from plain
// sh, so startup files that export a key or a helper never apply, and
// their output never mixes into the protocol. Only the folder is made
// owner-only: Claude keeps the user's umask for the files it writes. A
// missing claude exits with notInstalledCode, 127.
// WSLENV lets the Windows snow.exe that Claude starts for pane tools see
// its pane and socket. $1 is the connection ID, checked by validID; the
// rest are Claude's arguments.
const wslScript = `set -eu
id=$1
shift
claude=$(` + shell.ClaudeWhich + ` | tail -n 1)
case $claude in
/*) ;;
*) echo "snow: claude is not installed in this WSL distribution" >&2; exit 127 ;;
esac
dir="$HOME/` + wslConfigBase + `/$id/claude"
(umask 077; mkdir -p "$dir")
for name in $(env | sed -n -e 's/^\(CLAUDE[A-Za-z0-9_]*\)=.*/\1/p' -e 's/^\(ANTHROPIC[A-Za-z0-9_]*\)=.*/\1/p' -e 's/^\(` + bedrockKey + `\)=.*/\1/p'); do
	[ "$name" = ` + gitBashVar + ` ] || unset "$name"
done
for name in $(env | sed -n 's/^` + wslVar + `\([A-Za-z0-9_]*\)=.*/\1/p'); do
	eval "export $name=\"\$` + wslVar + `$name\""
	unset "` + wslVar + `$name"
done
export CLAUDE_CONFIG_DIR="$dir"
export WSLENV=SNOW_PANE:SNOW_SOCKET
exec "$claude" "$@"`

// wslCommand runs program's distribution through wsl.exe with the script.
// Env entries become forwarded variables listed in a WSLENV of Snow's own,
// replacing the user's, so the inherited list forwards nothing else.
func (p Profile) wslCommand(program shell.Profile, folder string, l shell.Launch) shell.Launch {
	args := slices.Clone(program.Args)
	if folder != "" {
		args = append(args, "--cd", folder)
	}
	args = append(args, "-e", "sh", "-c", wslScript, "snow-claude", p.ID)
	args = append(args, l.Args...)
	out := shell.Launch{Path: program.Path, Args: args, Hide: append(slices.Clone(l.Hide), "WSLENV")}
	var names []string
	for _, entry := range l.Env {
		name, value, _ := strings.Cut(entry, "=")
		if name == "CLAUDE_CONFIG_DIR" {
			// The script sets it inside the distribution, under $HOME.
			continue
		}
		out.Env = append(out.Env, wslVar+name+"="+value)
		names = append(names, wslVar+name)
	}
	out.Env = append(out.Env, "WSLENV="+strings.Join(names, ":"))
	return out
}

// configDirMatches reports whether dir, as Claude reports it, is this
// profile's folder. A WSL folder sits under a home only the distribution
// knows, so it matches by its fixed tail.
func (p Profile) configDirMatches(dir string) bool {
	if p.Distro == "" {
		return dir != "" && samePath(dir, p.ConfigDir)
	}
	if !path.IsAbs(dir) {
		return false
	}
	dir = path.Clean(dir)
	tail := "/" + path.Join(wslConfigBase, p.ID, "claude")
	return strings.HasSuffix(dir, tail) && len(dir) > len(tail)
}
