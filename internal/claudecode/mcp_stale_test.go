package claudecode

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupUpdatesStaleMCP(t *testing.T) {
	for _, confirm := range []bool{false, true} {
		t.Run(map[bool]string{false: "decline", true: "confirm"}[confirm], func(t *testing.T) {
			r := &recorder{user: true, command: "/old/deleted/snow"}
			path := filepath.Join(t.TempDir(), "settings.json")
			original := []byte(`{"theme":"dark"}`)
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			input := "n\nn\n"
			if confirm {
				input = "n\ny\n"
			}
			var out bytes.Buffer
			if err := Setup(FileTarget{Path: path, Cmd: snowCmd}, r.mcp(), answer(input), &out); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), "Update it? [y/N]") {
				t.Fatalf("setup did not offer an update: %s", &out)
			}
			want := "/old/deleted/snow"
			if confirm {
				want = "/opt/snow"
			}
			if r.command != want {
				t.Fatalf("command = %q, want %q", r.command, want)
			}
			settings, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(settings, original) {
				t.Fatalf("unrelated settings changed: %s, %v", settings, err)
			}
			if !confirm && len(r.calls) != 1 {
				t.Fatalf("decline caused writes: %q", r.joined())
			}
			if confirm {
				out.Reset()
				r.calls = nil
				if err := Setup(FileTarget{Path: path, Cmd: snowCmd}, r.mcp(), answer("n\n"), &out); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(out.String(), "Update it?") || len(r.calls) != 1 {
					t.Fatalf("unchanged registration prompted or wrote: %q, %s", r.joined(), &out)
				}
			}
		})
	}
}

func TestMCPRegistrationCommand(t *testing.T) {
	for _, tt := range []struct {
		name, exe, command, args string
		current                  bool
	}{
		{"plain", "/opt/snow", "/opt/snow", "mcp", true},
		{"spaces and apostrophe", "/opt/Snow's app/snow", "/opt/Snow's app/snow", "mcp", true},
		{"literal quotes", `/opt/"Snow"/snow`, `/opt/"Snow"/snow`, "mcp", true},
		{"windows", `C:\Program Files\Snow\snow.exe`, `C:\Program Files\Snow\snow.exe`, "mcp", true},
		{"old executable", "/opt/snow", "/old/snow", "mcp", false},
		{"shell quoting is not a path", "/opt/Snow app/snow", `"/opt/Snow app/snow"`, "mcp", false},
		{"extra argument", "/opt/snow", "/opt/snow", "mcp --debug", false},
		{"wrong argument", "/opt/snow", "/opt/snow", "hook", false},
		{"empty arguments", "/opt/snow", "/opt/snow", "", false},
		{"empty last argument", "/opt/snow", "/opt/snow", "mcp ", false},
		{"empty first argument", "/opt/snow", "/opt/snow", " mcp", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := NativeMCP("claude", nil, tt.exe)
			m.run = func([]string) ([]byte, error) {
				return []byte("snow:\r\n  Scope: User config (available in all your projects)\r\n  Type: stdio\r\n  Command: " + tt.command + "\r\n  Args: " + tt.args + "\r\n"), nil
			}
			set, err := m.Status()
			if err != nil || set != tt.current {
				t.Fatalf("Status = %v, %v, want %v", set, err, tt.current)
			}
			var out bytes.Buffer
			if err := reportMCP(m, &out); err != nil {
				t.Fatal(err)
			}
			want := "is stale"
			if tt.current {
				want = "is set"
			}
			if !strings.Contains(out.String(), want) {
				t.Fatalf("report = %q, want %q", out.String(), want)
			}
		})
	}
}

func TestMCPUnverifiableStopsWrites(t *testing.T) {
	for _, output := range []string{
		"unknown output",
		"Scope: Unknown\nType: stdio\nCommand: /opt/snow\nArgs: mcp\n",
		"Scope: User config (available in all your projects)\n",
		"Scope: User config (available in all your projects)\nType: stdio\nCommand: /opt/snow\n",
		"Scope: User config (available in all your projects)\nType: http\nCommand: /opt/snow\nArgs: mcp\n",
		"Scope: User config (available in all your projects)\nType: stdio\nCommand: /opt/snow\nCommand: /old/snow\nArgs: mcp\n",
	} {
		t.Run(output, func(t *testing.T) {
			calls := 0
			m := NativeMCP("claude", nil, "/opt/snow")
			m.run = func(args []string) ([]byte, error) {
				calls++
				if !isGet(args) {
					t.Fatalf("unverifiable output caused a write: %q", args)
				}
				return []byte(output), nil
			}
			var out bytes.Buffer
			if err := registerMCP(m, answer("y\n"), &out); !errors.Is(err, ErrMCPUnverifiable) {
				t.Fatalf("setup = %v", err)
			}
			if err := m.Add(); !errors.Is(err, ErrMCPUnverifiable) {
				t.Fatalf("add = %v", err)
			}
			if err := reportMCP(m, &out); !errors.Is(err, ErrMCPUnverifiable) {
				t.Fatalf("report = %v", err)
			}
			if err := unregisterMCP(m, answer("y\n"), &out); !errors.Is(err, ErrMCPUnverifiable) {
				t.Fatalf("remove = %v", err)
			}
			if calls != 4 || strings.Contains(out.String(), "[y/N]") || strings.Contains(out.String(), "is set") {
				t.Fatalf("unverifiable output gave misleading status or prompt: %d, %q", calls, &out)
			}
		})
	}
}

func TestMCPStaleUpdateVerifies(t *testing.T) {
	r := &recorder{user: true, command: "/old/snow"}
	m := r.mcp()
	m.run = func(args []string) ([]byte, error) {
		out, err := r.run(args)
		// Simulate a CLI that reports success without persisting the update.
		r.command = "/old/snow"
		return out, err
	}
	var out bytes.Buffer
	if err := registerMCP(m, answer("y\n"), &out); !errors.Is(err, ErrMCPUnverifiable) {
		t.Fatalf("unverified update = %v, %s", err, &out)
	}
	if strings.Contains(out.String(), "Done.") {
		t.Fatalf("unverified update claimed success: %s", &out)
	}
}

func TestRemoveStaleMCP(t *testing.T) {
	r := &recorder{user: true, command: "/old/snow"}
	if err := unregisterMCP(r.mcp(), answer("y\n"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if r.user {
		t.Fatal("stale registration was not removed")
	}
}

func TestSetupStaleMCPCLI(t *testing.T) {
	m, log := mcpCLI(t, "stale-registration")
	m.Exe = `/new/Snow's "app"/snow`
	var out bytes.Buffer
	if err := registerMCP(m, answer("y\n"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Update it?") || !strings.Contains(out.String(), "Done.") {
		t.Fatalf("update output = %q", &out)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	want := "mcp get snow\nmcp get snow\nmcp remove --scope user snow\nmcp add --scope user snow -- " + m.Exe + " mcp\nmcp get snow\n"
	if string(calls) != want {
		t.Fatalf("calls = %q, want %q", calls, want)
	}
	out.Reset()
	if err := registerMCP(m, answer(""), &out); err != nil {
		t.Fatal(err)
	}
	calls, err = os.ReadFile(log)
	if err != nil || string(calls) != want+"mcp get snow\n" || !strings.Contains(out.String(), "already set") {
		t.Fatalf("second setup = %q, %v, %q", calls, err, &out)
	}
}

func TestMCPUnverifiableCLI(t *testing.T) {
	m, log := mcpCLI(t, "unverifiable")
	var out bytes.Buffer
	if err := registerMCP(m, answer("y\n"), &out); !errors.Is(err, ErrMCPUnverifiable) {
		t.Fatalf("setup = %v, %s", err, &out)
	}
	calls, err := os.ReadFile(log)
	if err != nil || string(calls) != "mcp get snow\n" {
		t.Fatalf("unknown output caused further calls: %q, %v", calls, err)
	}
}
