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
  snow claude setup    add Snow's hooks and MCP server to Claude Code (asks first)
  snow claude remove   take them out of Claude Code
  snow claude status   say whether they are set

The hooks run snow hook, which tells Snow when a Claude Code pane works,
waits for you, or is idle. The MCP server runs snow mcp, which offers the
terminal panes of the same session to the agent.
Snow adds only its own hooks to ~/.claude/settings.json (or in
CLAUDE_CONFIG_DIR), keeps a copy of the file first, and never reads your
Claude Code login. It registers its MCP server through Claude Code's own
claude mcp command. On Windows it offers each WSL distribution that has
claude too.
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
// claudeDistros lists the WSL distributions that have claude. claudeProgram
// finds this machine's claude. Tests replace them: a test binary is not
// named snow, and a CI machine's WSL is not the user's.
var (
	snowPath      = os.Executable
	claudeDistros = shell.ClaudeDistros
	claudeProgram = shell.ClaudeProgram
)

// claudeInstall is one Claude Code: the settings snow claude changes and the
// MCP server it registers with the same program.
type claudeInstall struct {
	target claudecode.Target
	mcp    claudecode.MCP
}

// claudeInstalls lists the Claude Code installations snow claude changes:
// this machine's, and on Windows each WSL distribution's that has claude.
// skipped names the distributions that gave no answer.
func claudeInstalls() (installs []claudeInstall, skipped error, err error) {
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
	native := claudeInstall{
		target: claudecode.FileTarget{Path: path, Cmd: claudecode.HookCommand(exe)},
		mcp:    claudecode.MCP{Name: "Claude Code", Exe: exe},
	}
	if p, ok := claudeProgram(); ok {
		native.mcp = claudecode.NativeMCP(p.Path, p.Args, exe)
	}
	installs = []claudeInstall{native}
	distros, skipped := claudeDistros()
	for _, p := range distros {
		wslExe := shell.WSLPath(exe)
		installs = append(installs, claudeInstall{
			target: claudecode.WSLTarget{WSL: p.Path, Distro: p.Distro, Cmd: claudecode.HookCommand(wslExe)},
			mcp:    claudecode.WSLMCP(p.Path, p.Distro, wslExe),
		})
	}
	return installs, skipped, nil
}

// runClaude is snow claude: it changes or reports each installation in
// turn. An installation it cannot change does not stop the others.
func runClaude(a claudeAction, in io.Reader, stdout, stderr io.Writer) int {
	installs, skipped, err := claudeInstalls()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "snow:", err)
		return 1
	}
	if skipped != nil {
		_, _ = fmt.Fprintf(stderr, "snow: left out, could not ask: %v\n", skipped)
	}
	answers := bufio.NewReader(in)
	code := 0
	for _, install := range installs {
		var err error
		switch a {
		case claudeSetup:
			err = claudecode.Setup(install.target, install.mcp, answers, stdout)
		case claudeRemove:
			err = claudecode.Remove(install.target, install.mcp, answers, stdout)
		default:
			err = claudecode.Report(install.target, install.mcp, stdout)
		}
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "snow:", err)
			code = 1
		}
	}
	return code
}
