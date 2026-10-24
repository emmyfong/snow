package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/term"
)

// TestLoginHiddenPrompt runs `snow auth login` in a real terminal and types
// a key. The key must not show on the screen. It holds a space, so the
// format check stops it before any request goes out.
func TestLoginHiddenPrompt(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	p, err := term.Start(term.Spec{
		Path: exe, Args: []string{"auth", "login", "anthropic"},
		Env: []string{asSnow + "=1", "HOME=" + home, "USERPROFILE=" + home, "ANTHROPIC_API_KEY="},
	}, 120, 24, term.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()

	waitText(t, p, "Paste the key")
	p.Paste("sk-hidden SECRET-typed\r")
	waitText(t, p, "one word with no spaces")
	if text := p.Text(); strings.Contains(text, "SECRET-typed") {
		t.Fatalf("the typed key shows on the screen:\n%s", text)
	}
	select {
	case <-p.Done():
	case <-time.After(15 * time.Second):
		t.Fatal("snow auth login did not exit")
	}
}

// waitText waits until the pane's screen holds s.
func waitText(t *testing.T, p *term.Pane, s string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(p.Text(), s) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the screen never showed %q:\n%s", s, p.Text())
}
