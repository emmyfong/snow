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

// ErrMCPUnverifiable means Claude Code's output does not identify a
// registration that Snow can safely compare or change.
var ErrMCPUnverifiable = errors.New("claudecode: cannot verify Snow's MCP registration; run claude mcp get snow directly for details")

type mcpState uint8

const (
	mcpMissing mcpState = iota
	mcpCurrent
	mcpStale
	mcpOtherScope
)

// Status reports whether the user-scope registration runs the intended Snow
// executable with its MCP argument. An unknown output format is an error.
func (m MCP) Status() (bool, error) {
	state, err := m.status()
	return state == mcpCurrent, err
}

func (m MCP) status() (mcpState, error) {
	if !m.Installed() {
		return mcpMissing, nil
	}
	out, err := m.output("mcp", "get", serverName)
	if errors.Is(err, ErrNoServer) {
		return mcpMissing, nil
	}
	if err != nil {
		return mcpMissing, err
	}
	return m.registration(out)
}

func (m MCP) registration(out []byte) (mcpState, error) {
	fields := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		key, value, ok := strings.Cut(strings.TrimLeft(strings.TrimSuffix(line, "\r"), " \t"), ":")
		if !ok || (key != "Scope" && key != "Type" && key != "Command" && key != "Args") {
			continue
		}
		if _, duplicate := fields[key]; duplicate {
			return mcpMissing, ErrMCPUnverifiable
		}
		fields[key] = strings.TrimPrefix(value, " ")
	}
	switch fields["Scope"] {
	case "Project config (shared via .mcp.json)", "Local config (private to you in this project)":
		return mcpOtherScope, nil
	case "User config (available in all your projects)":
	default:
		return mcpMissing, ErrMCPUnverifiable
	}
	if fields["Type"] != "stdio" || fields["Command"] == "" {
		return mcpMissing, ErrMCPUnverifiable
	}
	if _, ok := fields["Args"]; !ok {
		return mcpMissing, ErrMCPUnverifiable
	}
	// Claude prints the command separately from its joined arguments. Treat
	// the whole field as a path, preserving spaces, quotes and backslashes.
	if fields["Command"] == m.Exe && fields["Args"] == "mcp" {
		return mcpCurrent, nil
	}
	return mcpStale, nil
}

// Add registers Snow's MCP server at user scope. An unchanged registration
// is left alone. A stale user registration is replaced and verified. Other
// scopes stay intact because every write explicitly selects user scope.
func (m MCP) Add() error {
	if !m.Installed() {
		return fmt.Errorf("%s: %w", m.displayName(), ErrNoServer)
	}
	state, err := m.status()
	if err != nil || state == mcpCurrent {
		return err
	}
	if state == mcpStale {
		if _, err := m.output("mcp", "remove", "--scope", "user", serverName); err != nil && !errors.Is(err, ErrNoServer) {
			return err
		}
	}
	if _, err := m.output("mcp", "add", "--scope", "user", serverName, "--", m.Exe, "mcp"); err != nil {
		return err
	}
	if state == mcpStale {
		current, err := m.Status()
		if err != nil {
			return err
		}
		if !current {
			return fmt.Errorf("%s: %w", m.displayName(), ErrMCPUnverifiable)
		}
	}
	return nil
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
	return m.outputContext(ctx, args...)
}

func (m MCP) outputContext(ctx context.Context, args ...string) ([]byte, error) {
	argv := append(append([]string{}, m.Args...), args...)
	cmd := exec.CommandContext(ctx, m.Program, argv...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("%s: claude %s: %w", m.displayName(), strings.Join(args, " "), ctx.Err())
	}
	// Only Claude's named-server diagnostics prove absence. Configuration
	// failures and unknown output must not authorize a registration write.
	if _, exited := errors.AsType[*exec.ExitError](err); exited {
		if missingServer(args, stderr.String()) {
			return nil, ErrNoServer
		}
	}
	return nil, fmt.Errorf("%s: claude %s: %w; %s; run this command directly for details",
		m.displayName(), strings.Join(args, " "), err, mcpDiagnostic(stderr.String()))
}

func missingServer(args []string, diagnostic string) bool {
	if len(args) < 3 || args[0] != "mcp" || (args[1] != "get" && args[1] != "remove") || args[len(args)-1] != serverName {
		return false
	}
	if args[1] == "remove" && strings.TrimSpace(diagnostic) == `No MCP server named "snow" in user scope` {
		return len(args) == 5 && args[2] == "--scope" && args[3] == "user"
	}
	switch strings.TrimSpace(diagnostic) {
	case `No MCP server named "snow". Run ` + "`claude mcp add`" + ` to add one.`,
		"No MCP server found with name: snow":
		return true
	}
	return false
}

// isGet reports whether args ask claude for one MCP server.
func isGet(args []string) bool { return len(args) >= 2 && args[0] == "mcp" && args[1] == "get" }

// mcpDiagnostic uses fixed summaries because CLI errors can contain config
// values, request headers, or credentials anywhere in their text.
func mcpDiagnostic(stderr string) string {
	s := strings.ToLower(stderr)
	switch {
	case strings.Contains(s, "config") && (strings.Contains(s, "corrupt") || strings.Contains(s, "parse") || strings.Contains(s, "unexpected token") || strings.Contains(s, "invalid")):
		return "Claude reported invalid configuration"
	case strings.Contains(s, "permission denied") || strings.Contains(s, "access is denied"):
		return "Claude reported a permission error"
	case strings.Contains(s, "unknown command") || strings.Contains(s, "unknown option"):
		return "Claude does not support this command or option"
	}
	return "Claude command failed"
}
