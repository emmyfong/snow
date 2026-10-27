package shell

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// ClaudeName is the name of the Claude Code profile that runs on this
// machine. A profile that runs it inside a WSL distribution adds the
// distribution's name in parentheses.
const ClaudeName = "Claude Code"

func claudeName(distro string) string {
	if distro == "" {
		return ClaudeName
	}
	return ClaudeName + " (" + distro + ")"
}

// detectClaude returns the Claude Code profiles: the one on this machine's
// PATH, then one for each running distribution among distros that has
// claude. A distribution that does not run is not asked, because asking
// starts it, which takes seconds.
func detectClaude(sys system, distros []string) []Profile {
	var out []Profile
	if p, ok := nativeClaude(sys); ok {
		out = append(out, p)
	}
	if sys.goos != "windows" || len(distros) == 0 || sys.wslRunning == nil || sys.claudeIn == nil {
		return out
	}
	wsl, err := sys.lookPath("wsl.exe")
	if err != nil {
		return out
	}
	running, err := sys.wslRunning()
	if err != nil {
		return out
	}
	// Each probe runs the user's startup files; asking the distributions
	// together keeps the server's start as short as the slowest one.
	found := make([]bool, len(distros))
	var wg sync.WaitGroup
	for i, d := range distros {
		if slices.Contains(running, d) {
			wg.Go(func() {
				// A distribution that does not answer is left out; Ctrl+b a
				// asks it again.
				found[i], _ = sys.claudeIn(wsl, d)
			})
		}
	}
	wg.Wait()
	for i, d := range distros {
		if found[i] {
			out = append(out, wslClaude(wsl, d))
		}
	}
	return out
}

// nativeClaude finds claude on this machine's PATH. On Windows the npm
// install is a .cmd script, which runs through cmd.
func nativeClaude(sys system) (Profile, bool) {
	if sys.lookPath == nil {
		return Profile{}, false
	}
	exes := []string{"claude"}
	if sys.goos == "windows" {
		exes = []string{"claude.exe", "claude.cmd"}
	}
	for _, exe := range exes {
		path, err := sys.lookPath(exe)
		if err != nil {
			continue
		}
		p := Profile{Name: ClaudeName, Kind: KindClaude, Path: path}
		if strings.EqualFold(filepath.Ext(path), ".cmd") {
			cmd, err := sys.lookPath("cmd.exe")
			if err != nil {
				continue
			}
			// /S strips only the outer quotes, so a path with spaces,
			// parentheses, or & stays one quoted word.
			p.Path, p.Args = cmd, []string{"/D", "/S", "/C", `"` + path + `"`}
			p.CmdLine = `"` + cmd + `" /D /S /C ""` + path + `""`
		}
		return p, true
	}
	return Profile{}, false
}

func wslClaude(wsl, distro string) Profile {
	return Profile{Name: claudeName(distro), Kind: KindClaude, Path: wsl, Args: []string{"-d", distro}, Distro: distro}
}

// claudeWhich prints the path of claude as the user's interactive login
// shell finds it, so a PATH set in .profile or .bashrc (nvm, ~/.local/bin)
// counts. Startup files may print other lines; the caller looks for the
// path.
const claudeWhich = `shell=$(getent passwd "$(id -un)" 2>/dev/null | cut -d: -f7)
[ -n "$shell" ] || shell=${SHELL:-/bin/sh}
exec "$shell" -lic 'command -v claude' </dev/null 2>/dev/null`

// claudeProbeTimeout bounds asking a distribution for claude. The ask may
// start the distribution and runs the user's startup files, such as nvm's,
// which can take many seconds.
const claudeProbeTimeout = 20 * time.Second

// claudeInDistro reports whether claude is on the user's PATH inside
// distro. A shell that runs but finds no claude is a plain no; an error
// means the distribution did not answer in time, or wsl.exe failed.
func claudeInDistro(wsl, distro string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), claudeProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, wsl, "-d", distro, "-e", "sh", "-c", claudeWhich)
	hideWindow(cmd)
	out, err := cmd.Output()
	if _, exited := errors.AsType[*exec.ExitError](err); exited && ctx.Err() == nil {
		return foundClaude(string(out)), nil
	}
	if err != nil {
		return false, fmt.Errorf("ask %s for claude: %w", distro, err)
	}
	return foundClaude(string(out)), nil
}

// foundClaude reports whether the output of claudeWhich names a claude
// program.
func foundClaude(out string) bool {
	for line := range strings.Lines(out) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "/") && strings.HasSuffix(line, "/claude") {
			return true
		}
	}
	return false
}

// Find returns the profile called name; an empty name is the first.
func Find(profiles []Profile, name string) (Profile, bool) {
	for _, p := range profiles {
		if name == "" || p.Name == name {
			return p, true
		}
	}
	return Profile{}, false
}

// ClaudeFor returns the Claude Code profile for a new pane next to a pane
// running host: the one inside host's WSL distribution for a WSL pane, or
// else the one on this machine. A Claude Code pane gets its own profile.
// A WSL pane never gets the Windows one: its folder is a Linux path.
func ClaudeFor(profiles []Profile, host string) (Profile, bool) {
	h, ok := Find(profiles, host)
	if !ok {
		h = Profile{}
	}
	if h.Kind == KindClaude {
		return h, true
	}
	return Find(profiles, claudeName(h.Distro))
}

// FindClaude looks for Claude Code again, for a pane next to host: inside
// host's WSL distribution, or on this machine. It runs wsl.exe for a WSL
// host, so it can take seconds; never call it where blocking matters.
func FindClaude(host Profile) (Profile, bool) {
	sys := currentSystem()
	if host.Distro == "" {
		return nativeClaude(sys)
	}
	wsl := host.Path
	if wsl == "" {
		var err error
		if wsl, err = sys.lookPath("wsl.exe"); err != nil {
			return Profile{}, false
		}
	}
	if ok, err := sys.claudeIn(wsl, host.Distro); !ok || err != nil {
		return Profile{}, false
	}
	return wslClaude(wsl, host.Distro), true
}

// HostShell returns the shell that commands for a pane running p go
// through: p itself for a shell, and for a Claude Code profile the WSL
// profile of its distribution, or the first shell on this machine.
func HostShell(profiles []Profile, p Profile) (Profile, bool) {
	if !p.IsAgent() {
		return p, true
	}
	for _, s := range profiles {
		if s.IsAgent() {
			continue
		}
		if p.Distro == "" && s.Kind != KindWSL || p.Distro != "" && s.Kind == KindWSL && s.Distro == p.Distro {
			return s, true
		}
	}
	return Profile{}, false
}
