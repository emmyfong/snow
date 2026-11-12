package claudecode

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMCPHelperCLI(t *testing.T) {
	separator := -1
	for i, arg := range os.Args {
		if arg == "--snow-mcp-test" {
			separator = i
			break
		}
	}
	if separator < 0 {
		return
	}
	args := os.Args[separator+1:]
	mode, log := args[0], args[1]
	f, err := os.OpenFile(log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := fmt.Fprintln(f, strings.Join(args[2:], " "))
	if err := errors.Join(writeErr, f.Close()); err != nil {
		t.Fatal(err)
	}
	write := func(out io.Writer, message string) {
		if _, err := fmt.Fprintln(out, message); err != nil {
			t.Fatal(err)
		}
	}
	switch mode {
	case "missing":
		write(os.Stderr, "No MCP server named \"snow\". Run `claude mcp add` to add one.")
	case "missing-legacy":
		write(os.Stderr, "No MCP server found with name: snow")
	case "missing-user":
		write(os.Stderr, `No MCP server named "snow" in user scope`)
	case "timeout":
		write(os.Stderr, "No MCP server found with name: snow")
		time.Sleep(time.Minute)
	case "corrupt":
		write(os.Stderr, `{"apiKey":"secret-not-for-errors"}`)
		write(os.Stderr, "Claude config is corrupt: unexpected token")
		os.Exit(7)
	case "unknown":
		write(os.Stderr, "Unknown CLI failure")
	case "other-name":
		write(os.Stderr, "No MCP server found with name: other")
	case "misleading":
		write(os.Stderr, "Claude config is corrupt: No MCP server settings can be read")
	case "user":
		write(os.Stdout, "snow:\n  Scope: User config (available in all your projects)")
		os.Exit(0)
	case "project":
		write(os.Stdout, "snow:\n  Scope: Project config (shared via .mcp.json)")
		os.Exit(0)
	default:
		t.Fatalf("unknown helper mode %q", mode)
	}
	os.Exit(1)
}

func TestMCPCLITimeout(t *testing.T) {
	m, _ := mcpCLI(t, "timeout")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := m.outputContext(ctx, "mcp", "get", serverName)
	if !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrNoServer) {
		t.Fatalf("output = %v, want deadline exceeded", err)
	}
}

func TestMCPCLIStartFailure(t *testing.T) {
	m := NativeMCP(filepath.Join(t.TempDir(), "missing-claude"), nil, "/opt/snow")
	set, err := m.Status()
	if set || err == nil || errors.Is(err, ErrNoServer) {
		t.Fatalf("Status = %v, %v; want executable-start failure", set, err)
	}
	_, pathError := errors.AsType[*os.PathError](err)
	_, execError := errors.AsType[*exec.Error](err)
	if !pathError && !execError {
		t.Fatalf("Status lost executable-start error: %v", err)
	}
}

func TestMCPMissingRegistrationCLI(t *testing.T) {
	for _, mode := range []string{"missing", "missing-legacy"} {
		t.Run(mode, func(t *testing.T) {
			m, _ := mcpCLI(t, mode)
			var out bytes.Buffer
			if err := reportMCP(m, &out); err != nil || !strings.Contains(out.String(), "MCP server is not set") {
				t.Fatalf("report = %q, %v", out.String(), err)
			}
			out.Reset()
			if err := registerMCP(m, answer("n\n"), &out); err != nil || !strings.Contains(out.String(), "Register it?") {
				t.Fatalf("register = %q, %v", out.String(), err)
			}
		})
	}
}

func TestMCPRemoveCLI(t *testing.T) {
	for _, mode := range []string{"missing", "missing-legacy", "missing-user", "misleading", "corrupt"} {
		t.Run(mode, func(t *testing.T) {
			m, _ := mcpCLI(t, mode)
			err := m.Remove()
			wantError := mode == "misleading" || mode == "corrupt"
			if (err != nil) != wantError {
				t.Fatalf("Remove = %v, want error=%v", err, wantError)
			}
		})
	}
}

func mcpCLI(t *testing.T, mode string) (MCP, string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "calls")
	return MCP{
		Name:    "Test Claude",
		Program: exe,
		Args:    []string{"-test.run=^TestMCPHelperCLI$", "--", "--snow-mcp-test", mode, log},
		Exe:     "/opt/snow",
	}, log
}

func TestMCPStatusCLI(t *testing.T) {
	for _, tt := range []struct {
		mode string
		set  bool
		fail bool
	}{
		{mode: "missing"},
		{mode: "missing-legacy"},
		{mode: "corrupt", fail: true},
		{mode: "unknown", fail: true},
		{mode: "other-name", fail: true},
		{mode: "misleading", fail: true},
		{mode: "user", set: true},
		{mode: "project"},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			m, _ := mcpCLI(t, tt.mode)
			set, err := m.Status()
			if set != tt.set || (err != nil) != tt.fail {
				t.Fatalf("Status = %v, %v; want set=%v, failure=%v", set, err, tt.set, tt.fail)
			}
			if tt.fail {
				if _, ok := errors.AsType[*exec.ExitError](err); !ok {
					t.Fatalf("Status lost the CLI exit error: %v", err)
				}
				if tt.mode == "corrupt" && !strings.Contains(err.Error(), "Claude config is corrupt: unexpected token") {
					t.Fatalf("Status lost the diagnostic: %v", err)
				}
				if strings.Contains(err.Error(), "secret-not-for-errors") {
					t.Fatalf("Status exposed configuration data: %v", err)
				}
			}
		})
	}
}

func TestMCPLookupFailureStopsWrites(t *testing.T) {
	for _, operation := range []string{"report", "setup", "add"} {
		t.Run(operation, func(t *testing.T) {
			m, log := mcpCLI(t, "corrupt")
			var out bytes.Buffer
			var err error
			switch operation {
			case "report":
				err = Report(FileTarget{Path: filepath.Join(t.TempDir(), "settings.json")}, m, &out)
			case "setup":
				err = Setup(FileTarget{Path: filepath.Join(t.TempDir(), "settings.json"), Cmd: snowCmd}, m, answer("n\ny\n"), &out)
			case "add":
				err = m.Add()
			}
			if err == nil || !strings.Contains(err.Error(), "Claude config is corrupt: unexpected token") {
				t.Fatalf("%s = %v; output %q", operation, err, out.String())
			}
			calls, readErr := os.ReadFile(log)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(calls) != "mcp get snow\n" {
				t.Fatalf("lookup failure caused more CLI calls: %q", calls)
			}
			if strings.Contains(out.String(), "Register it?") || strings.Contains(out.String(), "MCP server is not set") {
				t.Fatalf("lookup failure prompted setup or claimed absence: %q", out.String())
			}
		})
	}
}
