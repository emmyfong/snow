package claudecode

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ErrNoServer means Claude Code answered that it has no MCP server with the
// name Snow asked for.
var ErrNoServer = errors.New("claudecode: Claude Code has no such MCP server")

// serverName is what Claude Code calls Snow's MCP server.
const serverName = "snow"

// mcpTimeout bounds one claude command: inside WSL it may start the
// distribution.
const mcpTimeout = 30 * time.Second

// MCP registers snow mcp with one Claude Code installation. Snow runs Claude
// Code's own `claude mcp` command, so Claude Code writes its own user
// configuration. Snow never edits ~/.claude.json.
type MCP struct {
	// Name says which Claude Code this is, for people. Empty means this
	// machine's.
	Name string
	// Program and Args start claude. Args come before the mcp arguments,
	// for the cmd.exe wrapper on Windows and wsl.exe for a WSL install.
	Program string
	Args    []string
	// Exe is snow's program as this Claude Code runs it.
	Exe string
	// run replaces the claude command; tests set it.
	run func(args []string) ([]byte, error)
}

// NativeMCP returns the registration for the Claude Code that program and
// args start, with Snow's program at exe.
func NativeMCP(program string, args []string, exe string) MCP {
	return MCP{Name: "Claude Code", Program: program, Args: args, Exe: exe}
}

// WSLMCP returns the registration for the Claude Code inside distro. It runs
// wsl.exe and the user's interactive login shell, so claude and the PATH
// that nvm sets are found as in a WSL pane.
func WSLMCP(wsl, distro, exe string) MCP {
	return MCP{
		Name:    "Claude Code in " + distro,
		Program: wsl,
		Args:    []string{"-d", distro, "-e", "sh", "-c", wslClaude, "sh"},
		Exe:     exe,
	}
}

// wslClaude runs claude in the user's login shell, with the arguments
// wsl.exe passes after "sh".
const wslClaude = `shell=$(getent passwd "$(id -un)" 2>/dev/null | cut -d: -f7)
[ -n "$shell" ] || shell=${SHELL:-/bin/sh}
exec "$shell" -lic 'exec claude "$@"' sh "$@"`

// Installed reports whether Snow knows the claude program to run.
func (m MCP) Installed() bool { return m.Program != "" }

// displayName names the installation in messages.
func (m MCP) displayName() string {
	if m.Name != "" {
		return m.Name
	}
	return "Claude Code"
}

// Command is the command Claude Code runs for the server, for people.
func (m MCP) Command() string { return shellQuote(m.Exe) + " mcp" }

// Status reports whether Claude Code has Snow's MCP server registered at
// user scope. A server of the same name in another scope is not Snow's, so
// it does not count.
func (m MCP) Status() (bool, error) {
	if !m.Installed() {
		return false, nil
	}
	out, err := m.output("mcp", "get", serverName)
	switch {
	case err == nil:
		return userScope(out), nil
	case errors.Is(err, ErrNoServer):
		return false, nil
	}
	return false, err
}

// userScope reports whether `claude mcp get` found the server in the user
// scope, which Snow manages. A scope Snow does not know counts as the user's,
// so a setup does not add a second server that `get` cannot tell apart.
func userScope(out []byte) bool {
	for _, line := range strings.Split(string(out), "\n") {
		if after, ok := strings.CutPrefix(strings.TrimSpace(line), "Scope:"); ok {
			return strings.Contains(after, "User")
		}
	}
	return true
}

// Add registers Snow's MCP server at user scope. A server Snow added before
// is replaced, so a moved snow program is updated. A server of the same name
// in another scope stays: Claude Code's add puts Snow's own scope first.
func (m MCP) Add() error {
	if !m.Installed() {
		return fmt.Errorf("%s: %w", m.displayName(), ErrNoServer)
	}
	set, err := m.Status()
	if err != nil {
		return err
	}
	if set {
		if _, err := m.output("mcp", "remove", "--scope", "user", serverName); err != nil && !errors.Is(err, ErrNoServer) {
			return err
		}
	}
	_, err = m.output("mcp", "add", "--scope", "user", serverName, "--", m.Exe, "mcp")
	return err
}

// Remove unregisters Snow's MCP server from user scope.
func (m MCP) Remove() error {
	if !m.Installed() {
		return nil
	}
	_, err := m.output("mcp", "remove", "--scope", "user", serverName)
	if errors.Is(err, ErrNoServer) {
		return nil
	}
	return err
}

// output runs claude with args and returns its standard output.
func (m MCP) output(args ...string) ([]byte, error) {
	if !m.Installed() {
		return nil, fmt.Errorf("%s: claude is not on PATH", m.displayName())
	}
	if m.run != nil {
		return m.run(args)
	}
	ctx, cancel := context.WithTimeout(context.Background(), mcpTimeout)
	defer cancel()
	argv := append(append([]string{}, m.Args...), args...)
	cmd := exec.CommandContext(ctx, m.Program, argv...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}
	// `claude mcp get` exits nonzero when the server is not there, and
	// `claude mcp remove` says so when the user scope has none. Both are
	// answers, not failures.
	if _, exited := errors.AsType[*exec.ExitError](err); exited {
		if isGet(args) || strings.Contains(stderr.String(), "No MCP server") {
			return nil, fmt.Errorf("%w: %s", ErrNoServer, stderrLine(stderr.String()))
		}
	}
	if msg := stderrLine(stderr.String()); msg != "" {
		return nil, fmt.Errorf("%s: claude %s: %w: %s", m.displayName(), strings.Join(args, " "), err, msg)
	}
	return nil, fmt.Errorf("%s: claude %s: %w", m.displayName(), strings.Join(args, " "), err)
}

// isGet reports whether args ask claude for one MCP server.
func isGet(args []string) bool { return len(args) >= 2 && args[0] == "mcp" && args[1] == "get" }

// stderrLine returns the last non-empty line of a command's standard error,
// where claude says what went wrong.
func stderrLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[len(lines)-1])
}
