package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

func TestStatusBar(t *testing.T) {
	out := StatusBar(theme.Default(), StatusOptions{
		Width: 50, Session: "api",
		Tabs:  []Tab{{Index: 1, Name: "dev", Active: true}, {Index: 2, Name: "logs", Zoomed: true}},
		Right: "sh  14:02",
	})
	got := ansi.Strip(out)
	for _, want := range []string{"[api]", "1:dev*", "2:logsZ", "sh  14:02"} {
		if !strings.Contains(got, want) {
			t.Errorf("status %q lacks %q", got, want)
		}
	}
	if w := ansi.StringWidth(out); w != 50 {
		t.Fatalf("width %d, want 50", w)
	}
}

func TestStatusBarMessageReplacesRight(t *testing.T) {
	got := ansi.Strip(StatusBar(theme.Default(), StatusOptions{Width: 40, Session: "a", Right: "sh 14:02", Message: "no room to split"}))
	if !strings.Contains(got, "no room to split") || strings.Contains(got, "14:02") {
		t.Fatalf("status %q, want the message instead of the clock", got)
	}
}

func TestStatusBarIsWidthSafe(t *testing.T) {
	for _, w := range []int{0, 1, 5, 12} {
		out := StatusBar(theme.Default(), StatusOptions{Width: w, Session: "a-long-session-name", Tabs: []Tab{{Index: 1, Name: "PowerShell", Active: true}}, Right: "PowerShell 14:02"})
		if got := ansi.StringWidth(out); got != w {
			t.Errorf("width %d: got %d", w, got)
		}
	}
}

func TestStatusBarKeepsMessageWhenNarrow(t *testing.T) {
	tabs := []Tab{{Index: 1, Name: "dev", Active: true}, {Index: 2, Name: "logs"}, {Index: 3, Name: "build"}}
	out := StatusBar(theme.Default(), StatusOptions{Width: 30, Session: "api", Tabs: tabs, Right: "sh 14:02", Message: "no room"})
	got := ansi.Strip(out)
	if !strings.Contains(got, "no room") {
		t.Fatalf("narrow status %q dropped the message", got)
	}
	if w := ansi.StringWidth(out); w != 30 {
		t.Fatalf("width %d, want 30", w)
	}
}

func TestStatusBarUsesThemeGlyphs(t *testing.T) {
	th := theme.Default()
	th.Glyphs.ActiveTab, th.Glyphs.SessionLeft, th.Glyphs.SessionRight = "!", "<", ">"
	got := ansi.Strip(StatusBar(th, StatusOptions{Width: 30, Session: "api", Tabs: []Tab{{Index: 1, Name: "dev", Active: true}}}))
	if !strings.Contains(got, "<api>") || !strings.Contains(got, "1:dev!") {
		t.Fatalf("status %q ignores the theme's glyphs", got)
	}
}

// TestStatusBarPromptTakesTheBar: a question replaces the whole bar, in its
// own colors, as tmux does, so it cannot be missed.
func TestStatusBarPromptTakesTheBar(t *testing.T) {
	th := theme.Default()
	opts := StatusOptions{Width: 50, Session: "api", Tabs: []Tab{{Index: 0, Name: "dev", Active: true}}, Right: "14:02", Message: "no room", Prompt: "close this pane? y/n"}
	out := StatusBar(th, opts)
	got := ansi.Strip(out)
	if strings.TrimSpace(got) != "close this pane? y/n" {
		t.Fatalf("status %q, want only the question", got)
	}
	if w := ansi.StringWidth(out); w != 50 {
		t.Fatalf("width %d, want 50", w)
	}
	opts.Prompt = ""
	if plain := StatusBar(th, opts); strings.Contains(out, plain[:8]) && out[:8] == plain[:8] {
		t.Fatal("the question uses the normal bar colors")
	}
}
