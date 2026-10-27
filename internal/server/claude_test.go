package server

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// claudeOptions are test options whose panes keep the profile they were
// asked for, so a layout shows which one a split chose. claude and find
// stand in for the detected Claude Code profiles.
func claudeOptions(claude, find func(host string) (string, bool)) Options {
	opts := testOptions()
	opts.Spec = func(profile, dir string) (term.Spec, string, error) {
		spec, name, err := testOptions().Spec(profile, dir)
		if profile != "" {
			name = profile
		}
		return spec, name, err
	}
	opts.claudeProfile, opts.findClaude = claude, find
	return opts
}

// attachNoPrompt attaches a client to a new session and returns the first
// pane's ID, without waiting for a shell prompt.
func attachNoPrompt(t *testing.T, s *Server) (*apitest.Client, int) {
	t.Helper()
	c := connect(t, s, 81, 24)
	c.Send(&api.Attach{Session: "work", Create: true})
	l := nextLayout(t, c, func(*api.Layout) bool { return true })
	return c, l.Windows[0].Panes[0].ID
}

func claudeSplit(id int) *api.Command {
	return &api.Command{Op: api.OpSplit, Target: id, Args: map[string]string{api.ArgDir: api.DirRight, api.ArgAgent: api.AgentClaude}}
}

// TestClaudeSplitPicksHostProfile: Ctrl+b a asks for an agent, and the
// server picks the Claude Code profile that fits the focused pane.
func TestClaudeSplitPicksHostProfile(t *testing.T) {
	claude := func(host string) (string, bool) { return "Claude Code (" + host + ")", true }
	s := New(claudeOptions(claude, nil))
	t.Cleanup(s.Close)
	c, first := attachNoPrompt(t, s)
	c.Send(claudeSplit(first))
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	host := shownPanes(l)[0].Profile
	if got := shownPanes(l)[1]; got.Profile != "Claude Code ("+host+")" || !got.Focused {
		t.Fatalf("new pane %+v, want the Claude Code profile of %q, focused", got, host)
	}
}

// TestClaudeSplitLooksAgain: a claude installed after the server started is
// found when the user asks for it.
func TestClaudeSplitLooksAgain(t *testing.T) {
	none := func(string) (string, bool) { return "", false }
	found := func(string) (string, bool) { return "Claude Code", true }
	s := New(claudeOptions(none, found))
	t.Cleanup(s.Close)
	c, first := attachNoPrompt(t, s)
	c.Send(claudeSplit(first))
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	if got := shownPanes(l)[1].Profile; got != "Claude Code" {
		t.Fatalf("new pane runs %q, want Claude Code", got)
	}
}

// TestClaudeSplitHint: with no claude anywhere, the user learns how to
// install it, and the window does not change.
func TestClaudeSplitHint(t *testing.T) {
	none := func(string) (string, bool) { return "", false }
	s := New(claudeOptions(none, none))
	t.Cleanup(s.Close)
	c, first := attachNoPrompt(t, s)
	c.Send(claudeSplit(first))
	e := nextError(t, c)
	if e.Code != api.CodeCommandFailed || !strings.Contains(e.Message, "claude.com/claude-code") {
		t.Fatalf("error %+v, want a command-failed install hint", e)
	}
}

// TestAgentArgValidated: an agent the server does not know is a bad
// command.
func TestAgentArgValidated(t *testing.T) {
	cmd := claudeSplit(1)
	cmd.Args[api.ArgAgent] = "gemini"
	if err := cmd.Validate(); !errors.Is(err, api.ErrInvalidCommand) {
		t.Fatalf("Validate() = %v, want ErrInvalidCommand", err)
	}
}

var claudeProfiles = []shell.Profile{
	{Name: "pwsh", Kind: shell.KindPwsh, Path: "pwsh"},
	{Name: "Ubuntu", Kind: shell.KindWSL, Path: "wsl.exe", Args: []string{"-d", "Ubuntu"}, Distro: "Ubuntu"},
	{Name: "Claude Code", Kind: shell.KindClaude, Path: "claude"},
	{Name: "Claude Code (Ubuntu)", Kind: shell.KindClaude, Path: "wsl.exe", Args: []string{"-d", "Ubuntu"}, Distro: "Ubuntu"},
}

func TestLauncherClaude(t *testing.T) {
	l := detectedLauncher(claudeProfiles, shell.Hooks{})
	for host, want := range map[string]string{"pwsh": "Claude Code", "Ubuntu": "Claude Code (Ubuntu)", "": "Claude Code"} {
		if got, ok := l.claude(host); !ok || got != want {
			t.Errorf("claude(%q) = %q, %v; want %q", host, got, ok, want)
		}
	}
	if d, ok := l.distro("Claude Code (Ubuntu)"); !ok || d != "Ubuntu" {
		t.Errorf("distro of the WSL Claude Code profile = %q, %v; want Ubuntu", d, ok)
	}
	spec, _, err := l.spec("Claude Code (Ubuntu)", "/home/emmy/api")
	if err != nil || !strings.Contains(strings.Join(spec.Args, " "), "--cd /home/emmy/api") {
		t.Errorf("spec %+v, %v; want the Linux folder kept", spec, err)
	}
}

// TestChatFromClaudePaneUsesShell: a chat opened beside a Claude Code pane
// runs its commands in the shell under it, never in claude.
func TestChatFromClaudePaneUsesShell(t *testing.T) {
	l := detectedLauncher(claudeProfiles, shell.Hooks{})
	for host, want := range map[string]string{"Claude Code": "pwsh", "Claude Code (Ubuntu)": "Ubuntu"} {
		if got, ok := l.shell(host); !ok || got.Name != want {
			t.Errorf("shell(%q) = %q, %v; want %q", host, got.Name, ok, want)
		}
	}
}

// TestLauncherAddsFoundClaude: a profile found later joins the list, so the
// picker shows it and a restored pane finds it.
func TestLauncherAddsFoundClaude(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("looks for claude on PATH; the Windows names differ")
	}
	dir := t.TempDir()
	writeExe(t, dir, "claude")
	t.Setenv("PATH", dir)
	l := detectedLauncher(claudeProfiles[:1], shell.Hooks{})
	if _, ok := l.claude("pwsh"); ok {
		t.Fatal("found a Claude Code profile before looking")
	}
	if got, ok := l.findClaude("pwsh"); !ok || got != "Claude Code" {
		t.Fatalf("findClaude() = %q, %v; want Claude Code", got, ok)
	}
	if got, ok := l.claude("pwsh"); !ok || got != "Claude Code" {
		t.Fatalf("claude() after finding it = %q, %v", got, ok)
	}
	if names := l.profiles(); len(names) != 2 || names[1] != "Claude Code" {
		t.Fatalf("profiles %q, want pwsh then Claude Code", names)
	}
}

// writeExe writes an empty executable called name into dir.
func writeExe(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
}
