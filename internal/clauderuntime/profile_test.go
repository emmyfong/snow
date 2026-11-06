package clauderuntime

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/shell"
)

func TestConnectionDirectories(t *testing.T) {
	root := t.TempDir()
	a, err := NewProfile(root, "connection-a", Subscription)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewProfile(root, "connection-b", APIKey)
	if err != nil {
		t.Fatal(err)
	}
	if a.ConfigDir == b.ConfigDir || a.CredentialID() == b.CredentialID() {
		t.Fatal("connections share state")
	}
	if err := a.Prepare(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.ConfigDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(b.ConfigDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preparing A changed B: %v", err)
	}
	for _, id := range []string{"", "../standalone", "a/b", `a\b`, "A", strings.Repeat("x", 65)} {
		if _, err := NewProfile(root, id, Subscription); !errors.Is(err, ErrProfile) {
			t.Errorf("accepted identity %q: %v", id, err)
		}
	}
	if _, err := NewProfile(filepath.Base(root), "a", Subscription); !errors.Is(err, ErrProfile) {
		t.Fatalf("relative state directory: %v", err)
	}
}

func TestConnectionCannotInheritBillingRoute(t *testing.T) {
	for _, mode := range []AuthMode{Subscription, APIKey} {
		t.Run(string(mode), func(t *testing.T) {
			p, err := NewProfile(t.TempDir(), "selected", mode)
			if err != nil {
				t.Fatal(err)
			}
			key := ""
			if mode == APIKey {
				key = "selected-key"
			}
			inherited := []string{"ANTHROPIC_API_KEY=host-key", "ANTHROPIC_BASE_URL=https://host.invalid",
				"CLAUDE_CODE_OAUTH_TOKEN=host-token", "CLAUDE_CONFIG_DIR=standalone", "CLAUDE_CODE_USE_BEDROCK=1", "CLAUDECODE=1", "PATH=/bin"}
			launch, err := p.Launch(shell.Launch{Env: []string{"CLAUDE_CONFIG_DIR=other", "ANTHROPIC_AUTH_TOKEN=other", "SNOW_PANE=1"}}, inherited, key)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range inherited[:len(inherited)-1] {
				name, _, _ := strings.Cut(entry, "=")
				if !slices.Contains(launch.Hide, name) || slices.Contains(launch.Env, entry) {
					t.Errorf("inherited connection override %s survives", name)
				}
			}
			if !slices.Contains(launch.Env, "CLAUDE_CONFIG_DIR="+p.ConfigDir) || !slices.Contains(launch.Env, "SNOW_PANE=1") {
				t.Fatal("selected connection or Snow context was lost")
			}
			for _, entry := range launch.Env {
				if strings.HasPrefix(entry, "ANTHROPIC_") && entry != "ANTHROPIC_API_KEY="+key {
					t.Fatal("unselected Anthropic override remains")
				}
			}
			if mode == Subscription && slices.Contains(launch.Env, "ANTHROPIC_API_KEY=") {
				t.Fatal("subscription gained API-key route")
			}
			if mode == APIKey && !slices.Contains(launch.Env, "ANTHROPIC_API_KEY="+key) {
				t.Fatal("selected API key was not supplied")
			}
		})
	}
}

func TestConnectionProcessUsesSelectedAccount(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "standalone-key")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "standalone-token")
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
	t.Setenv("CLAUDE_CONFIG_DIR", "standalone-directory")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []AuthMode{Subscription, APIKey} {
		t.Run(string(mode), func(t *testing.T) {
			p, err := NewProfile(t.TempDir(), "selected", mode)
			if err != nil {
				t.Fatal(err)
			}
			key := ""
			if mode == APIKey {
				key = "selected-key"
			}
			launch, err := p.Launch(shell.Launch{Path: exe,
				Args: []string{"-test.run=^TestConnectionEnvironmentHelper$"},
				Env:  []string{"SNOW_PROFILE_TEST=" + p.ConfigDir, "SNOW_PROFILE_TEST_KEY=" + key}}, os.Environ(), key)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if code, err := launch.Run(ctx, io.Discard); err != nil || code != 0 {
				t.Fatalf("child received an unselected account: exit %d, %v", code, err)
			}
		})
	}
	if os.Getenv("ANTHROPIC_API_KEY") != "standalone-key" || os.Getenv("CLAUDE_CONFIG_DIR") != "standalone-directory" {
		t.Fatal("launch changed the host connection")
	}
}

func TestConnectionEnvironmentHelper(_ *testing.T) {
	dir := os.Getenv("SNOW_PROFILE_TEST")
	if dir == "" {
		return
	}
	if os.Getenv("CLAUDE_CONFIG_DIR") != dir || os.Getenv("ANTHROPIC_API_KEY") != os.Getenv("SNOW_PROFILE_TEST_KEY") ||
		os.Getenv("ANTHROPIC_AUTH_TOKEN") != "" || os.Getenv("CLAUDE_CODE_USE_BEDROCK") != "" {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestConnectionRejectsWrongCredential(t *testing.T) {
	for _, tc := range []struct {
		mode AuthMode
		key  string
	}{
		{Subscription, "api-key"}, {APIKey, ""}, {APIKey, "key\ninjected=value"},
	} {
		p, err := NewProfile(t.TempDir(), "selected", tc.mode)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.Launch(shell.Launch{}, nil, tc.key); !errors.Is(err, ErrCredential) {
			t.Errorf("route %s accepted incompatible credential: %v", tc.mode, err)
		}
	}
}

// TestConnectionKeepsGitBashPath: the Git Bash path that native Windows
// Claude Code needs passes through; every other Claude variable is hidden.
func TestConnectionKeepsGitBashPath(t *testing.T) {
	p, err := NewProfile(t.TempDir(), "selected", Subscription)
	if err != nil {
		t.Fatal(err)
	}
	l, err := p.Launch(shell.Launch{Path: "claude"}, []string{"CLAUDE_CODE_GIT_BASH_PATH=C:\\Git\\bin\\bash.exe", "CLAUDE_CODE_USE_BEDROCK=1"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(l.Hide, "CLAUDE_CODE_GIT_BASH_PATH") || !slices.Contains(l.Hide, "CLAUDE_CODE_USE_BEDROCK") {
		t.Fatalf("hidden = %q, want the Git Bash path kept and the gateway switch hidden", l.Hide)
	}
}
