package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

func TestStatusBar(t *testing.T) {
	out := StatusBar(theme.Default(), StatusOptions{
		Width: 50, Session: "api",
		Right: "sh  14:02",
	})
	got := ansi.Strip(out)
	for _, want := range []string{"api", "sh  14:02"} {
		if !strings.Contains(got, want) {
			t.Errorf("status %q lacks %q", got, want)
		}
	}
	if w := ansi.StringWidth(out); w != 50 {
		t.Fatalf("width %d, want 50", w)
	}
}

// TestStatusBarDividesMessage: a dark cell sits before the error strip in
// every theme, because the strip can share the bar's color (Clay).
func TestStatusBarDividesMessage(t *testing.T) {
	for _, th := range theme.Builtin() {
		divider := lipgloss.NewStyle().Background(th.BarFG).Render(" ")
		for _, w := range []int{30, 60} {
			out := StatusBar(th, StatusOptions{Width: w, Session: "api", Message: "no room"})
			if !strings.Contains(out, divider) || ansi.StringWidth(out) != w {
				t.Errorf("%s at %d: want a divider and width %d: %q", th.Name, w, w, out)
			}
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
		out := StatusBar(theme.Default(), StatusOptions{Width: w, Session: "a-long-session-name", Right: "PowerShell 14:02"})
		if got := ansi.StringWidth(out); got != w {
			t.Errorf("width %d: got %d", w, got)
		}
	}
}

func TestStatusBarKeepsMessageWhenNarrow(t *testing.T) {
	out := StatusBar(theme.Default(), StatusOptions{Width: 30, Session: "api", Right: "sh 14:02", Message: "no room"})
	got := ansi.Strip(out)
	if !strings.Contains(got, "no room") {
		t.Fatalf("narrow status %q dropped the message", got)
	}
	if w := ansi.StringWidth(out); w != 30 {
		t.Fatalf("width %d, want 30", w)
	}
}

// TestStatusBarKeepsNotice: a notice stays when the line is short, as a
// message does, and a notice wider than the line is cut. It is not drawn in
// the error color (#238).
func TestStatusBarKeepsNotice(t *testing.T) {
	th := theme.Default()
	const notice = "Resumed with the approval mode ask. Type /mode auto to stop the questions."
	for _, width := range []int{20, 40, 80, 100} {
		o := StatusOptions{Width: width, Session: "api", Right: "sh 14:02", Notice: notice}
		out := StatusBar(th, o)
		if w := ansi.StringWidth(out); w != width {
			t.Fatalf("width %d: bar is %d cells", width, w)
		}
		got := ansi.Strip(out)
		if want := ansi.Truncate(notice, max(width-2, 0), "…"); width < 70 && !strings.Contains(got, want) {
			t.Fatalf("width %d: bar %q lacks the cut notice %q", width, got, want)
		}
		if width >= 80 && !strings.Contains(got, "Type /mode auto") {
			t.Fatalf("width %d: bar %q dropped the notice", width, got)
		}
		o.Notice, o.Message = "", notice
		if out == StatusBar(th, o) {
			t.Fatalf("width %d: a notice is drawn like an error", width)
		}
	}
}

// TestStatusBarPromptTakesTheBar: a question replaces the whole bar, in its
// own colors, as tmux does, so it cannot be missed.
func TestStatusBarPromptTakesTheBar(t *testing.T) {
	th := theme.Default()
	opts := StatusOptions{Width: 50, Session: "api", Right: "14:02", Message: "no room", Prompt: "close this pane? y/n"}
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

// TestStatusBarCutsLongMessage: a message wider than the bar is cut to
// fit, never dropped; it is the only feedback the user gets.
func TestStatusBarCutsLongMessage(t *testing.T) {
	msg := "Claude Code is not installed in Ubuntu. Install it there: https://claude.com/claude-code"
	for _, width := range []int{80, 40, 12, 4} {
		o := StatusOptions{Width: width, Session: "api", Right: "14:02", Message: msg}
		out := StatusBar(theme.Default(), o)
		if w := ansi.StringWidth(out); w != width {
			t.Fatalf("width %d: bar is %d wide", width, w)
		}
		got := ansi.Strip(out)
		if width >= 12 && (!strings.Contains(got, "Claude") || !strings.Contains(got, "…")) {
			t.Fatalf("width %d: bar %q, want the start of the message, cut", width, got)
		}
	}
}

func TestStatusBarFolderSeparator(t *testing.T) {
	for _, folder := range []string{"Projects/snow", ""} {
		got := strings.TrimSpace(ansi.Strip(StatusBar(theme.Default(), StatusOptions{Width: 80, Session: "work", Folder: folder})))
		want := "work"
		if folder != "" {
			want += " · " + folder
		}
		if got != want {
			t.Fatalf("bar %q, want %q", got, want)
		}
	}
}

func TestStatusBarHint(t *testing.T) {
	for _, tt := range []struct {
		width           int
		message, notice string
		want            bool
	}{
		{80, "", "", true}, {16, "", "", false}, {80, "no room", "", false}, {80, "", "copied", false},
	} {
		o := StatusOptions{Width: tt.width, Session: "work", Folder: "Projects/snow", Right: "2 need you", Hint: "Esc: cancel", Message: tt.message, Notice: tt.notice}
		out := StatusBar(theme.Default(), o)
		if ansi.StringWidth(out) != tt.width || strings.Contains(ansi.Strip(out), o.Hint) != tt.want {
			t.Fatalf("options %+v: %q", o, out)
		}
	}
}

func TestStatusLineKeepsErrors(t *testing.T) {
	th := theme.Default()
	for _, width := range []int{1, 2, 4, 12, 80} {
		o := StatusOptions{Width: width, Session: "work", Folder: "Projects/snow", Right: "2 need you", Message: "no room to split"}
		got := StatusBar(th, o)
		if ansi.StringWidth(got) != width {
			t.Fatalf("width %d: %q", width, got)
		}
		if strings.Contains(ansi.Strip(got), "need you") {
			t.Fatalf("message lost: %q", got)
		}
		if width >= 12 && !strings.Contains(ansi.Strip(got), "no room") {
			t.Fatalf("error lost: %q", got)
		}
		errorStyle := lipgloss.NewStyle().Foreground(th.ErrorFG).Background(th.Error).Bold(true)
		message := layoutStatus(th, o).right
		if message != "" && !strings.Contains(got, errorStyle.Render(message)) {
			t.Fatalf("error lacks error colors: %q", got)
		}
	}
}

func TestStatusBarSanitizesLabels(t *testing.T) {
	o := StatusOptions{Width: 80, Session: "work\n\x1b[2J", Folder: "Projects/\x1b[31msnow", Right: "1 need you\r"}
	got := ansi.Strip(StatusBar(theme.Default(), o))
	if strings.ContainsAny(got, "\n\r\x1b") || !strings.Contains(got, "Projects/snow") {
		t.Fatalf("unsafe status row: %q", got)
	}
}
