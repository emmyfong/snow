package clauderuntime

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/emmyfong/snow/internal/shell"
)

func TestChatLaunchUsesOnlySelectedSettingsAndMCP(t *testing.T) {
	p, err := NewProfile(t.TempDir(), "selected", Subscription)
	if err != nil {
		t.Fatal(err)
	}
	l, err := p.ChatLaunch(shell.Profile{Path: "/bin/claude"}, "/workspace", "", RunOptions{
		Resume: "--unsafe-flag", MCP: map[string]MCPServer{"snow": {Command: "snow", Args: []string{"mcp"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"--setting-sources", "--strict-mcp-config", "--permission-prompt-tool", "--resume=--unsafe-flag"} {
		if !slices.Contains(l.Args, required) {
			t.Errorf("missing %s", required)
		}
	}
	if i := slices.Index(l.Args, "--setting-sources"); i < 0 || l.Args[i+1] != "" {
		t.Fatal("inherited settings were enabled")
	}
	i := slices.Index(l.Args, "--mcp-config")
	var config struct {
		Servers map[string]MCPServer `json:"mcpServers"`
	}
	if i < 0 || json.Unmarshal([]byte(l.Args[i+1]), &config) != nil || len(config.Servers) != 1 || config.Servers["snow"].Command != "snow" {
		t.Fatal("Snow MCP configuration was lost")
	}
	if l.Dir != "/workspace" || slices.Contains(l.Args, "--fallback-model") || slices.Contains(l.Args, "--dangerously-skip-permissions") {
		t.Fatal("launch changed workspace, billing or permission policy")
	}
}

func TestAuthLaunchSharesChatProfile(t *testing.T) {
	p, err := NewProfile(t.TempDir(), "selected", Subscription)
	if err != nil {
		t.Fatal(err)
	}
	program := shell.Profile{Path: "/bin/claude"}
	chat, err := p.ChatLaunch(program, "/workspace", "", RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	chat.Env = slices.DeleteFunc(chat.Env, func(value string) bool { return value == "CLAUDE_CODE_SDK_READS_SESSION_STATE=1" })
	for _, operation := range []string{"login", "status", "logout"} {
		launch, err := p.AuthLaunch(program, "/workspace", "", operation)
		if err != nil || !slices.Equal(launch.Env, chat.Env) || !slices.Equal(launch.Hide, chat.Hide) {
			t.Fatalf("%s changed connection environment: %v", operation, err)
		}
		if operation == "login" && !slices.Contains(launch.Args, "--claudeai") {
			t.Fatal("subscription login did not select Claude account flow")
		}
	}
	p.Auth = APIKey
	if _, err := p.AuthLaunch(program, "", "explicit-key", "login"); !errors.Is(err, ErrCredential) {
		t.Fatalf("API-key route opened browser login: %v", err)
	}
}

func TestRuntimeRejectsShellWrappers(t *testing.T) {
	p, err := NewProfile(t.TempDir(), "selected", Subscription)
	if err != nil {
		t.Fatal(err)
	}
	for _, program := range []shell.Profile{
		{}, {Path: "claude.cmd"}, {Path: "cmd.exe", CmdLine: "claude.cmd"},
		{Path: "wsl.exe", Distro: "Ubuntu"},
	} {
		if _, err := p.ChatLaunch(program, "", "", RunOptions{}); !errors.Is(err, ErrExecutable) {
			t.Errorf("unsafe native executable accepted: %v", err)
		}
	}
}
