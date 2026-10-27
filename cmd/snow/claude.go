package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/emmyfong/snow/internal/claudecode"
	"github.com/emmyfong/snow/internal/shell"
)

const claudeUsage = `usage:
  snow claude setup    add Snow's hooks to Claude Code's settings (asks first)
  snow claude remove   take Snow's hooks out of Claude Code's settings
  snow claude status   say whether the hooks are set

The hooks run snow hook, which tells Snow when a Claude Code pane works,
waits for you, or is idle. Snow changes only its own hooks in
~/.claude/settings.json (or in CLAUDE_CONFIG_DIR), keeps a copy of the
file first, and never reads your Claude Code login. On Windows it offers
each WSL distribution that has claude too.
`

// claudeAction is what snow claude does.
type claudeAction string

const (
	claudeSetup  claudeAction = "setup"
	claudeRemove claudeAction = "remove"
	claudeStatus claudeAction = "status"
)

// parseClaudeCommand reads the arguments after "claude": help, or an
// action.
func parseClaudeCommand(args []string) (command, error) {
	if len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		return command{kind: kindClaudeHelp}, nil
	}
	a, err := parseClaude(args)
	if err != nil {
		return command{kind: kindClaudeHelp}, err
	}
	return command{kind: kindClaude, claude: a}, nil
}

// parseClaude reads the arguments after "claude".
func parseClaude(args []string) (claudeAction, error) {
	if len(args) != 1 {
		return "", errors.New("claude needs setup, remove, or status")
	}
	switch a := claudeAction(args[0]); a {
	case claudeSetup, claudeRemove, claudeStatus:
		return a, nil
	}
	return "", fmt.Errorf("unknown claude command %s", args[0])
}

// snowPath is where this snow program is, for the hook command, and
// claudeDistros lists the WSL distributions that have claude. Tests
// replace them: a test binary is not named snow, and a CI machine's WSL is
// not the user's.
var (
	snowPath      = os.Executable
	claudeDistros = shell.ClaudeDistros
)

// claudeTargets lists the settings files snow claude changes: this
// machine's, and on Windows each WSL distribution's that has claude.
// skipped names the distributions that gave no answer.
func claudeTargets() (targets []claudecode.Target, skipped error, err error) {
	exe, err := snowPath()
	if err != nil {
		return nil, nil, fmt.Errorf("find snow's own path: %w", err)
	}
	if err := claudecode.CheckProgram(exe); err != nil {
		return nil, nil, err
	}
	path, err := claudecode.SettingsPath()
	if err != nil {
		return nil, nil, err
	}
	targets = []claudecode.Target{claudecode.FileTarget{Path: path, Cmd: claudecode.HookCommand(exe)}}
	distros, skipped := claudeDistros()
	for _, p := range distros {
		targets = append(targets, claudecode.WSLTarget{WSL: p.Path, Distro: p.Distro, Cmd: claudecode.HookCommand(shell.WSLPath(exe))})
	}
	return targets, skipped, nil
}

// runClaude is snow claude: it changes or reports each settings file in
// turn. A file it cannot change does not stop the others.
func runClaude(a claudeAction, in io.Reader, stdout, stderr io.Writer) int {
	targets, skipped, err := claudeTargets()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "snow:", err)
		return 1
	}
	if skipped != nil {
		_, _ = fmt.Fprintf(stderr, "snow: left out, could not ask: %v\n", skipped)
	}
	answers := bufio.NewReader(in)
	code := 0
	for _, t := range targets {
		switch a {
		case claudeSetup:
			err = claudecode.Setup(t, answers, stdout)
		case claudeRemove:
			err = claudecode.Remove(t, answers, stdout)
		default:
			err = claudecode.Report(t, stdout)
		}
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "snow:", err)
			code = 1
		}
	}
	return code
}
