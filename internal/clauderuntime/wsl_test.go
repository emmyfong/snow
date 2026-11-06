package clauderuntime

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/shell"
)

// TestConnectionEnvironmentNoFallback: a connection runs only with Claude
// from its own environment. A native connection refuses a WSL program, and
// a WSL connection refuses a native one or another distribution's.
func TestConnectionEnvironmentNoFallback(t *testing.T) {
	root := t.TempDir()
	native := shell.Profile{Kind: shell.KindClaude, Path: "/usr/bin/claude"}
	ubuntu := shell.Profile{Kind: shell.KindClaude, Path: "wsl.exe", Args: []string{"-d", "Ubuntu"}, Distro: "Ubuntu"}
	debian := shell.Profile{Kind: shell.KindClaude, Path: "wsl.exe", Args: []string{"-d", "Debian"}, Distro: "Debian"}
	tests := []struct {
		name    string
		distro  string
		program shell.Profile
		ok      bool
	}{
		{"native with native", "", native, true},
		{"native with WSL", "", ubuntu, false},
		{"WSL with native", "Ubuntu", native, false},
		{"WSL with another distro", "Ubuntu", debian, false},
		{"WSL with its distro", "Ubuntu", ubuntu, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ProfileFor(root, Connection{ID: "c1", Auth: Subscription, Distro: tt.distro})
			if err != nil {
				t.Fatal(err)
			}
			for _, op := range []string{"status", "login", "logout"} {
				_, err := p.AuthLaunch(tt.program, "", "", op)
				if tt.ok != (err == nil) || (!tt.ok && !errors.Is(err, ErrProfile)) {
					t.Errorf("%s: err = %v, want ok %v", op, err, tt.ok)
				}
			}
			if _, err := p.ChatLaunch(tt.program, "", "", RunOptions{}); tt.ok != (err == nil) {
				t.Errorf("chat: err = %v, want ok %v", err, tt.ok)
			}
		})
	}
}

// TestWSLLaunchKeepsLoginInDistro: a WSL connection starts Claude through
// wsl.exe in its own distribution, with a config folder under the Linux
// home, and forwards only Snow's own variables through a WSLENV of its own.
// The key never appears in the arguments.
func TestWSLLaunchKeepsLoginInDistro(t *testing.T) {
	p, err := ProfileFor(t.TempDir(), Connection{ID: "c1", Auth: APIKey, Distro: "Ubuntu"})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	program := shell.Profile{Kind: shell.KindClaude, Path: "wsl.exe", Args: []string{"-d", "Ubuntu"}, Distro: "Ubuntu"}
	l, err := p.AuthLaunch(program, "/home/u/work", "sk-secret-value", "status")
	if err != nil {
		t.Fatal(err)
	}
	if l.Path != "wsl.exe" || !slices.Equal(l.Args[:4], []string{"-d", "Ubuntu", "--cd", "/home/u/work"}) {
		t.Fatalf("launch does not enter the distribution: %q %q", l.Path, l.Args)
	}
	if !slices.Contains(l.Args, "c1") || !slices.Contains(l.Args, "status") || !slices.Contains(l.Args, "--json") {
		t.Fatalf("launch lost the connection or the command: %q", l.Args)
	}
	if strings.Contains(strings.Join(l.Args, " "), "sk-secret-value") {
		t.Fatal("key appears in the arguments")
	}
	var wslenv string
	for _, e := range l.Env {
		if strings.HasPrefix(e, "CLAUDE_CONFIG_DIR=") || strings.HasPrefix(e, "ANTHROPIC_API_KEY=") {
			t.Errorf("variable not wrapped for the script: %q", e)
		}
		if v, ok := strings.CutPrefix(e, "WSLENV="); ok {
			wslenv = v
		}
	}
	if !slices.Contains(l.Env, wslVar+"ANTHROPIC_API_KEY=sk-secret-value") {
		t.Errorf("key not forwarded to the script: %q", l.Env)
	}
	for _, name := range strings.Split(wslenv, ":") {
		if !strings.HasPrefix(name, wslVar) {
			t.Errorf("WSLENV forwards a non-Snow variable: %q", wslenv)
		}
	}
	if !slices.Contains(l.Hide, "WSLENV") {
		t.Error("the user's WSLENV is inherited")
	}
	// Only WSLENV names cross into the distribution, and the script drops
	// every Claude and Anthropic variable there before it restores Snow's.
	script := l.Args[slices.Index(l.Args, "-c")+1]
	if strings.Index(script, "unset \"$name\"") > strings.Index(script, wslVar) {
		t.Error("the script restores Snow's variables before it clears inherited ones")
	}
}

// TestWSLConfigDirMatch: a WSL status answer counts only when Claude reports
// this connection's folder under some Linux home.
func TestWSLConfigDirMatch(t *testing.T) {
	p, err := ProfileFor(t.TempDir(), Connection{ID: "c1", Auth: Subscription, Distro: "Ubuntu"})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		dir  string
		want bool
	}{
		{"/home/u/.local/state/snow/runtimes/c1/claude", true},
		{"/home/u/.local/state/snow/runtimes/c1/claude/", true},
		{"/home/u/.local/state/snow/runtimes/c2/claude", false},
		{"/home/u/.claude", false},
		{".local/state/snow/runtimes/c1/claude", false},
		{"/.local/state/snow/runtimes/c1/claude", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := p.configDirMatches(tt.dir); got != tt.want {
			t.Errorf("configDirMatches(%q) = %v, want %v", tt.dir, got, tt.want)
		}
	}
}

// TestWSLChatReachesPaneTools: Claude in a distribution starts Snow's pane
// tools through the Windows snow.exe by its /mnt path. The pane and socket
// values stay as they are: snow.exe reads them on the Windows side.
func TestWSLChatReachesPaneTools(t *testing.T) {
	p, err := ProfileFor(t.TempDir(), Connection{ID: "c1", Auth: Subscription, Distro: "Ubuntu"})
	if err != nil {
		t.Fatal(err)
	}
	program := shell.Profile{Kind: shell.KindClaude, Path: "wsl.exe", Args: []string{"-d", "Ubuntu"}, Distro: "Ubuntu"}
	l, err := p.ChatLaunch(program, "/home/u/work", "", RunOptions{MCP: map[string]MCPServer{"snow": {
		Command: `C:\Program Files\snow\snow.exe`, Args: []string{"mcp"},
		Env: map[string]string{"SNOW_PANE": "3", "SNOW_SOCKET": `C:\Users\u\snow.sock`}}}})
	if err != nil {
		t.Fatal(err)
	}
	config := l.Args[slices.Index(l.Args, "--mcp-config")+1]
	var got struct {
		Servers map[string]MCPServer `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(config), &got); err != nil {
		t.Fatal(err)
	}
	snow := got.Servers["snow"]
	if snow.Command != "/mnt/c/Program Files/snow/snow.exe" || snow.Env["SNOW_SOCKET"] != `C:\Users\u\snow.sock` {
		t.Fatalf("pane tools in WSL: %+v", snow)
	}
}
