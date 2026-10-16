package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// TestComponentsWidthSafe: every component renders within its width, at
// widths 1 to 200, with ordinary and zero-value options.
func TestComponentsWidthSafe(t *testing.T) {
	th := theme.Default()
	items := []ListItem{{Cols: []string{"api-server", "running", "2m ago"}}, {Cols: []string{"漢字のセッション", "saved", "yesterday"}}}
	hints := []Hint{{Key: "enter", Desc: "open or create"}, {Key: "↑↓", Desc: "select"}, {Key: "esc", Desc: "quit"}}
	for w := 1; w <= 200; w++ {
		renders := map[string]string{
			"Input":      Input(th, InputOptions{Value: "a-very-long-session-name-that-overflows", Prompt: "> ", Width: w, Focused: true}),
			"Input zero": Input(th, InputOptions{Width: w}),
			"List":       List(th, ListOptions{Items: items, Selected: 1, Width: w, Height: 5}),
			"List zero":  List(th, ListOptions{Width: w}),
			"KeyHints":   KeyHints(th, hints, w),
			"Panel":      Panel(th, PanelOptions{Title: "Recent sessions", Body: "line one\nline two is long", Width: w}),
			"Panel zero": Panel(th, PanelOptions{Width: w}),
			"Wordmark":   Wordmark(th, w),
			"Popup":      Popup(th, PopupOptions{Title: "t", Items: []string{"a", "b"}, Width: w}),
			"StatusBar":  StatusBar(th, StatusOptions{Width: w, Session: "s", Tabs: []Tab{{Index: 0, Name: "dev", Active: true}}, Right: "14:02"}),
		}
		for name, out := range renders {
			for _, row := range strings.Split(out, "\n") {
				if got := ansi.StringWidth(row); got > w {
					t.Fatalf("%s at width %d drew a row %d wide: %q", name, w, got, row)
				}
			}
		}
	}
}

func TestInput(t *testing.T) {
	th := theme.Default()
	got := ansi.Strip(Input(th, InputOptions{Value: "api", Prompt: "> ", Width: 20, Focused: true}))
	if !strings.HasPrefix(got, "> api") || ansi.StringWidth(got) != 20 {
		t.Fatalf("Input = %q, want the prompt and value padded to 20", got)
	}
	got = ansi.Strip(Input(th, InputOptions{Placeholder: "session name", Prompt: "> ", Width: 20}))
	if !strings.Contains(got, "session name") {
		t.Fatalf("empty Input = %q, want the placeholder", got)
	}
	// A long value keeps its end visible, where the user is typing.
	got = ansi.Strip(Input(th, InputOptions{Value: "abcdefghijklmnopqrstuvwxyz", Prompt: "> ", Width: 10, Focused: true}))
	if !strings.Contains(got, "xyz") {
		t.Fatalf("long Input = %q, want the end of the value visible", got)
	}
}

func TestListAlignsColumnsAndMarksSelection(t *testing.T) {
	th := theme.Default()
	out := ansi.Strip(List(th, ListOptions{Items: []ListItem{
		{Cols: []string{"api", "running", "2m ago"}},
		{Cols: []string{"dotfiles", "saved", "1h ago"}},
	}, Selected: 1, Width: 40}))
	rows := strings.Split(out, "\n")
	if len(rows) != 2 {
		t.Fatalf("%d rows, want 2", len(rows))
	}
	if strings.Index(rows[0], "running") != strings.Index(rows[1], "saved") {
		t.Fatalf("columns do not line up:\n%s", out)
	}
	if !strings.HasPrefix(rows[1], th.Glyphs.Pointer) || strings.HasPrefix(rows[0], th.Glyphs.Pointer) {
		t.Fatalf("selection marker wrong:\n%s", out)
	}
}

func TestListScrollsToTheSelection(t *testing.T) {
	th := theme.Default()
	var items []ListItem
	for _, n := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		items = append(items, ListItem{Cols: []string{n}})
	}
	out := ansi.Strip(List(th, ListOptions{Items: items, Selected: 6, Width: 10, Height: 3}))
	if rows := strings.Split(out, "\n"); len(rows) != 3 || !strings.Contains(out, "g") {
		t.Fatalf("List = %q, want 3 rows including the selected g", out)
	}
}

func TestKeyHintsDropWhatDoesNotFit(t *testing.T) {
	th := theme.Default()
	hints := []Hint{{Key: "enter", Desc: "open"}, {Key: "esc", Desc: "quit"}}
	if got := ansi.Strip(KeyHints(th, hints, 80)); !strings.Contains(got, "enter open") || !strings.Contains(got, "esc quit") {
		t.Fatalf("KeyHints = %q", got)
	}
	if got := ansi.Strip(KeyHints(th, hints, 12)); strings.Contains(got, "esc") {
		t.Fatalf("narrow KeyHints = %q, want only the hints that fit whole", got)
	}
}

func TestPanelFramesItsBody(t *testing.T) {
	out := ansi.Strip(Panel(theme.Default(), PanelOptions{Title: "Recent", Body: "a\nb", Width: 12}))
	rows := strings.Split(out, "\n")
	if len(rows) != 4 || !strings.Contains(rows[0], "Recent") || !strings.HasPrefix(rows[1], "│a") {
		t.Fatalf("Panel =\n%s", out)
	}
}

func TestWordmark(t *testing.T) {
	out := ansi.Strip(Wordmark(theme.Default(), 80))
	rows := strings.Split(out, "\n")
	if len(rows) != WordmarkHeight || !strings.Contains(out, "█") {
		t.Fatalf("Wordmark =\n%s", out)
	}
	if Wordmark(theme.Default(), 10) != "" {
		t.Fatal("Wordmark drew in a space too narrow for it")
	}
}
