package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
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

// TestMaskedInput: a masked value shows one glyph per character and never
// the value.
func TestMaskedInput(t *testing.T) {
	th := theme.Default()
	const secret = "sk-secret-value"
	got := ansi.Strip(Input(th, InputOptions{Value: secret, Prompt: "> ", Width: 30, Focused: true, Mask: true}))
	if strings.Contains(got, "secret") || strings.Contains(got, "sk-") {
		t.Fatalf("masked Input = %q, leaks the value", got)
	}
	if n := strings.Count(got, th.Glyphs.Mask); n != len(secret) {
		t.Fatalf("masked Input = %q, shows %d glyphs, want %d", got, n, len(secret))
	}
	if ansi.StringWidth(got) != 30 {
		t.Fatalf("masked Input width = %d, want 30", ansi.StringWidth(got))
	}
}

// TestMaskedInputGolden draws a masked field, so a change to the glyph or
// its style shows.
func TestMaskedInputGolden(t *testing.T) {
	got := Input(theme.Default(), InputOptions{Value: "sk-ant-example", Prompt: "$ ", Width: 24, Focused: true, Mask: true})
	golden.RequireEqual(t, got)
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

func TestListHeadingsAndRows(t *testing.T) {
	th := theme.Default()
	items := []ListItem{
		{Heading: "Model"},
		{Cols: []string{"/model"}},
		{Heading: "Snow"},
		{Cols: []string{"/help"}},
	}
	out, rows := ListRows(th, ListOptions{Items: items, Selected: 3, Width: 20})
	lines := strings.Split(ansi.Strip(out), "\n")
	if len(lines) != 4 || !strings.Contains(lines[0], "Model") || !strings.Contains(lines[2], "Snow") {
		t.Fatalf("ListRows =\n%s", ansi.Strip(out))
	}
	if len(rows) != 4 || rows[0] != 0 || rows[1] != 1 || rows[2] != 2 || rows[3] != 3 {
		t.Fatalf("rows = %v, want one per item", rows)
	}
	if strings.HasPrefix(lines[0], th.Glyphs.Pointer) || strings.HasPrefix(lines[2], th.Glyphs.Pointer) {
		t.Fatalf("a heading is marked as selected:\n%s", ansi.Strip(out))
	}
	if !strings.HasPrefix(lines[3], th.Glyphs.Pointer) {
		t.Fatalf("the selected item is not marked:\n%s", ansi.Strip(out))
	}
}

func TestListWindow(t *testing.T) {
	tests := []struct{ n, selected, height, first, last int }{
		{5, 0, 0, 0, 5},
		{8, 6, 3, 4, 7},
		{8, 0, 3, 0, 3},
		{3, 2, 8, 0, 3},
	}
	for _, tt := range tests {
		first, last := ListWindow(tt.n, tt.selected, tt.height)
		if first != tt.first || last != tt.last {
			t.Errorf("ListWindow(%d, %d, %d) = %d, %d; want %d, %d", tt.n, tt.selected, tt.height, first, last, tt.first, tt.last)
		}
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

// TestReadableOnAnyBackground: Snow keeps the terminal's background, which
// may be light or dark. The wordmark uses the terminal's own text color, and
// a selected row sets its text color along with its background.
func TestReadableOnAnyBackground(t *testing.T) {
	th := theme.Default()
	if w := Wordmark(th, 80); strings.Contains(w, "38;") {
		t.Fatalf("the wordmark sets a text color: %q", w)
	}
	list := List(th, ListOptions{Items: []ListItem{{Cols: []string{"api"}}}, Selected: 0, Width: 20})
	if !strings.Contains(list, "38;") {
		t.Fatalf("the selected row leaves its text to the terminal: %q", list)
	}
}

func TestWordmarkWidthMatchesThePixels(t *testing.T) {
	if len(wordmarkPixels[0]) != WordmarkWidth {
		t.Fatalf("WordmarkWidth is %d, the pixels are %d wide", WordmarkWidth, len(wordmarkPixels[0]))
	}
}

// TestOverlayKeepsTheWidthAcrossWideCharacters: a wide character cut by
// the box's edge becomes a space, so the row keeps its width.
func TestOverlayKeepsTheWidthAcrossWideCharacters(t *testing.T) {
	base := "漢字漢字漢字漢字漢字" // 20 cells
	for x := 0; x <= 18; x++ {
		got := Overlay(base, "ab", x, 0) // ends at x+2: an odd x cuts a character on both sides
		if w := ansi.StringWidth(got); w != 20 {
			t.Fatalf("Overlay at x=%d: %q is %d cells, want 20", x, got, w)
		}
		if !strings.Contains(got, "ab") {
			t.Fatalf("Overlay at x=%d lost the box: %q", x, got)
		}
	}
}

// TestChoiceBoxFieldDefaults: each field option left empty keeps its own
// default, whatever the others hold.
func TestChoiceBoxFieldDefaults(t *testing.T) {
	th := theme.Default()
	got := ansi.Strip(ChoiceBox(th, ChoiceOptions{Width: 60, Title: "t", Reason: true, ReasonPlaceholder: "type here"}))
	if !strings.Contains(got, "Reason: ") || !strings.Contains(got, "type here") || !strings.Contains(got, "answer no") {
		t.Fatalf("box:\n%s", got)
	}
	got = ansi.Strip(ChoiceBox(th, ChoiceOptions{Width: 60, Title: "t", Reason: true, ReasonPrompt: " Answer: "}))
	if !strings.Contains(got, "Answer: ") || !strings.Contains(got, "optional") {
		t.Fatalf("box:\n%s", got)
	}
}
