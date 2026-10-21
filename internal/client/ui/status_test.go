package ui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
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
	for _, want := range []string{"[api]", " 1 dev* ", " 2 logsZ ", "sh  14:02"} {
		if !strings.Contains(got, want) {
			t.Errorf("status %q lacks %q", got, want)
		}
	}
	if w := ansi.StringWidth(out); w != 50 {
		t.Fatalf("width %d, want 50", w)
	}
}

// TestStatusBarMarksCurrentWindow: in every theme, the shown window is a
// block in the tab colors and the other windows are plain bar text.
func TestStatusBarMarksCurrentWindow(t *testing.T) {
	for _, th := range theme.Builtin() {
		out := StatusBar(th, StatusOptions{Width: 50, Session: "api", Tabs: []Tab{{Index: 1, Name: "dev", Active: true}, {Index: 2, Name: "logs"}}})
		active := lipgloss.NewStyle().Foreground(th.TabActive).Background(th.TabActiveBG).Bold(true)
		bar := lipgloss.NewStyle().Foreground(th.BarFG).Background(th.BarBG)
		if !strings.Contains(out, active.Render(" 1 dev* ")) || !strings.Contains(out, bar.Render(" 2 logs ")) {
			t.Errorf("%s: want the current window in the tab colors and the other plain: %q", th.Name, out)
		}
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
	if !strings.Contains(got, "<api>") || !strings.Contains(got, "1 dev!") {
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
	style := func(s string) string { return s[:strings.IndexByte(s, 'm')+1] }
	if plain := StatusBar(th, opts); style(out) == style(plain) {
		t.Fatal("the question uses the normal bar colors")
	}
}

func TestTabSpansMatchTheDrawnBar(t *testing.T) {
	tabs := []Tab{{Index: 0, Name: "dev"}, {Index: 1, Name: "logs", Active: true}, {Index: 2, Name: "漢字", Zoomed: true}}
	tests := []struct {
		name string
		o    StatusOptions
		want []string // the text under each returned span
	}{
		{"all fit", StatusOptions{Width: 80, Session: "api", Tabs: tabs, Right: "pwsh  14:02"}, []string{" 0 dev ", " 1 logs* ", " 2 漢字Z "}},
		{"cut by the width", StatusOptions{Width: 25, Session: "api", Tabs: tabs}, []string{" 0 dev ", " 1 logs* ", " 2"}},
		{"tab cut", StatusOptions{Width: 16, Session: "api", Tabs: tabs}, []string{" 0 dev ", " 1"}},
		{"tab hidden", StatusOptions{Width: 7, Session: "api", Tabs: tabs}, nil},
		{"cut by a message", StatusOptions{Width: 32, Session: "api", Tabs: tabs, Message: "no room to split"}, []string{" 0 dev "}},
		{"no tabs", StatusOptions{Width: 30, Session: "api"}, nil},
	}
	for _, tt := range tests {
		bar := ansi.Strip(StatusBar(theme.Default(), tt.o))
		spans := TabSpans(theme.Default(), tt.o)
		var got []string
		for _, sp := range spans {
			got = append(got, ansi.Cut(bar, sp.X, sp.X+sp.W))
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s: text under spans = %q, want %q (bar %q)", tt.name, got, tt.want, bar)
		}
	}
}

func TestTabSpansForPromptAndNoWidth(t *testing.T) {
	if got := TabSpans(theme.Default(), StatusOptions{Width: 40, Tabs: []Tab{{Name: "a"}}, Prompt: "close? y/n"}); got != nil {
		t.Errorf("a prompt leaves spans %v", got)
	}
	if got := TabSpans(theme.Default(), StatusOptions{Tabs: []Tab{{Name: "a"}}}); got != nil {
		t.Errorf("width 0 leaves spans %v", got)
	}
}
