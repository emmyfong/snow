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
