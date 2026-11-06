package clauderuntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/emmyfong/snow/internal/shell"
)

// ErrExecutable means a selected program cannot provide the structured runtime
// transport. A terminal pane can still use a CLI wrapper independently.
var ErrExecutable = errors.New("unsupported Claude runtime executable")

// MCPServer is an explicitly selected stdio integration. Snow supplies its own
// executable and pane identity; personal MCP settings are not imported.
type MCPServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// RunOptions selects explicit runtime features for a chat process.
type RunOptions struct {
	Resume string
	Model  string
	MCP    map[string]MCPServer
}

// ChatLaunch builds the official SDK's native streaming/control invocation.
// It loads no personal or workspace settings, which can contain auth helpers or
// alternate billing routes. Managed policy still applies. Project instructions
// must be supplied through Snow's selected-context path.
func (p Profile) ChatLaunch(program shell.Profile, folder, key string, options RunOptions) (shell.Launch, error) {
	l, err := p.command(program, folder, key)
	if err != nil {
		return shell.Launch{}, err
	}
	config := struct {
		Servers map[string]MCPServer `json:"mcpServers"`
	}{Servers: options.MCP}
	if config.Servers == nil {
		config.Servers = map[string]MCPServer{}
	}
	mcp, err := json.Marshal(config)
	if err != nil {
		return shell.Launch{}, fmt.Errorf("encode Snow MCP configuration: %w", err)
	}
	l.Args = append(l.Args, "--output-format", "stream-json", "--input-format", "stream-json", "--verbose",
		"--include-partial-messages", "--permission-prompt-tool", "stdio", "--permission-mode", "default",
		"--strict-mcp-config", "--mcp-config", string(mcp))
	if options.Resume != "" {
		l.Args = append(l.Args, "--resume="+options.Resume)
	}
	if options.Model != "" {
		l.Args = append(l.Args, "--model="+options.Model)
	}
	l.Env = append(l.Env, "CLAUDE_CODE_SDK_READS_SESSION_STATE=1")
	return p.finish(program, folder, l), nil
}

// AuthLaunch uses the same isolated native profile and setting sources as chat.
// Subscription login stays in Claude's official flow. Snow saves explicit API
// keys itself, so that route never opens a Console login as a substitute.
func (p Profile) AuthLaunch(program shell.Profile, folder, key, operation string) (shell.Launch, error) {
	if operation != "status" && operation != "login" && operation != "logout" {
		return shell.Launch{}, fmt.Errorf("%w: unknown authentication operation", ErrProfile)
	}
	if operation != "status" && p.Auth != Subscription {
		return shell.Launch{}, ErrCredential
	}
	l, err := p.command(program, folder, key)
	if err != nil {
		return shell.Launch{}, err
	}
	l.Args = append(l.Args, "auth", operation)
	switch operation {
	case "login":
		l.Args = append(l.Args, "--claudeai")
	case "status":
		l.Args = append(l.Args, "--json")
	}
	return p.finish(program, folder, l), nil
}

// finish turns a native launch into one for p's environment. It runs last,
// after every argument and variable is in place, so a WSL launch forwards
// all of them.
func (p Profile) finish(program shell.Profile, folder string, l shell.Launch) shell.Launch {
	if p.Distro == "" {
		return l
	}
	return p.wslCommand(program, folder, l)
}

func (p Profile) command(program shell.Profile, folder, key string) (shell.Launch, error) {
	if program.Distro != p.Distro {
		// A connection runs only in its own environment, never another's login.
		return shell.Launch{}, fmt.Errorf("%w: connection belongs to another environment", ErrProfile)
	}
	ext := strings.ToLower(filepath.Ext(program.Path))
	if program.Path == "" || program.CmdLine != "" || ext == ".cmd" || ext == ".bat" {
		// The official Python SDK also refuses Windows batch wrappers. Passing
		// structured JSON through cmd's expansion rules is not a safe argv path.
		return shell.Launch{}, ErrExecutable
	}
	l := shell.Launch{Path: program.Path, Args: slices.Clone(program.Args), Dir: folder}
	if p.Distro != "" {
		// finish puts wsl.exe and its arguments in front; Claude's start here.
		l.Args, l.Dir = nil, ""
	}
	l.Args = append(l.Args, "--setting-sources", "")
	return p.Launch(l, os.Environ(), key)
}
