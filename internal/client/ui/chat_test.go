package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

func rowsOf(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// TestTranscriptFits: at any width, with wide characters, no row is wider
// than the width, and labels show.
func TestTranscriptFits(t *testing.T) {
	th := theme.Default()
	entries := []TranscriptEntry{
		{Kind: EntryUser, Text: "fix the 名前 bug in rename.go please, it breaks on wide names"},
		{Kind: EntryThinking, Text: "look at rename.go"},
		{Kind: EntryAgent, Text: "Done.\nThe 名前名前名前名前名前名前 case works now.", Cut: true},
		{Kind: EntryNote, Text: "switched to fake/model-2"},
		{Kind: EntryError, Text: "rate limited"},
		{Kind: EntryQueued, Text: "queued: and the tests"},
	}
	for _, w := range []int{1, 5, 13, 20, 40, 80, 120} {
		out := Transcript(th, TranscriptOptions{Width: w, Entries: entries})
		for i, r := range rowsOf(out) {
			if got := ansi.StringWidth(r); got > w {
				t.Fatalf("width %d: row %d is %d wide: %q", w, i, got, ansi.Strip(r))
			}
		}
		if w >= 13 {
			text := ansi.Strip(out)
			for _, want := range []string{"you", "snow", "error", "(cut off)"} {
				if !strings.Contains(text, want) {
					t.Fatalf("width %d: no %q in\n%s", w, want, text)
				}
			}
		}
	}
	if Transcript(th, TranscriptOptions{Width: 0, Entries: entries}) != "" || Transcript(th, TranscriptOptions{Width: 10}) != "" {
		t.Fatal("zero width or no entries should draw nothing")
	}
}

// TestTranscriptPlainText: escape sequences and control characters in a
// reply do not reach the terminal.
func TestTranscriptPlainText(t *testing.T) {
	out := Transcript(theme.Default(), TranscriptOptions{Width: 40, Entries: []TranscriptEntry{
		{Kind: EntryAgent, Text: "a\x1b[2Jb\x1b]0;title\x07c\x08d\te"},
	}})
	text := ansi.Strip(out)
	if !strings.Contains(text, "abcd    e") || strings.Contains(out, "\x1b[2J") || strings.Contains(out, "\x07") {
		t.Fatalf("got %q", out)
	}
}

// TestActivityWidth: the row is exactly the width, and the hint goes
// first when short.
func TestActivityWidth(t *testing.T) {
	th := theme.Default()
	for _, w := range []int{1, 10, 30, 80} {
		out := Activity(th, ActivityOptions{Width: w, Frame: 3, Text: "working 12s", Hint: "Esc interrupt"})
		if got := ansi.StringWidth(out); got != w {
			t.Fatalf("width %d: got %d: %q", w, got, ansi.Strip(out))
		}
	}
	if s := ansi.Strip(Activity(th, ActivityOptions{Width: 20, Text: "working 12s", Hint: "Esc interrupt"})); strings.Contains(s, "Esc") {
		t.Fatalf("hint kept when short: %q", s)
	}
	if Activity(th, ActivityOptions{Width: 0}) != "" {
		t.Fatal("zero width")
	}
}

// TestTextArea: rows are exactly the width, the cursor follows the text,
// and a long value scrolls to the cursor.
func TestTextArea(t *testing.T) {
	th := theme.Default()
	tests := []struct {
		name         string
		o            TextAreaOptions
		rows, cx, cy int
	}{
		{"empty", TextAreaOptions{Width: 20, Height: 3, Prompt: "> ", Placeholder: "type"}, 1, 2, 0},
		{"short", TextAreaOptions{Width: 20, Height: 3, Prompt: "> ", Value: "hello", Cursor: 5}, 1, 7, 0},
		{"cursor inside", TextAreaOptions{Width: 20, Height: 3, Prompt: "> ", Value: "hello", Cursor: 1}, 1, 3, 0},
		{"line break", TextAreaOptions{Width: 20, Height: 3, Prompt: "> ", Value: "ab\ncd", Cursor: 5}, 2, 4, 1},
		{"wraps", TextAreaOptions{Width: 7, Height: 3, Prompt: "> ", Value: "abcdefgh", Cursor: 8}, 2, 5, 1},
		{"full row", TextAreaOptions{Width: 7, Height: 3, Prompt: "> ", Value: "abcde", Cursor: 5}, 2, 2, 1},
		{"wide", TextAreaOptions{Width: 8, Height: 3, Prompt: "> ", Value: "名前名前", Cursor: 4}, 2, 4, 1},
		{"scrolls", TextAreaOptions{Width: 10, Height: 2, Prompt: "> ", Value: "1\n2\n3\n4", Cursor: 7}, 2, 3, 1},
		{"scrolls up", TextAreaOptions{Width: 10, Height: 2, Prompt: "> ", Value: "1\n2\n3\n4", Cursor: 0}, 2, 2, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := TextArea(th, tt.o)
			rows := rowsOf(out)
			if len(rows) != tt.rows || TextAreaRows(tt.o) != tt.rows {
				t.Fatalf("rows %d (TextAreaRows %d), want %d:\n%s", len(rows), TextAreaRows(tt.o), tt.rows, ansi.Strip(out))
			}
			for _, r := range rows {
				if ansi.StringWidth(r) != tt.o.Width {
					t.Fatalf("row %q is %d wide", ansi.Strip(r), ansi.StringWidth(r))
				}
			}
			if x, y := TextAreaCursor(tt.o); x != tt.cx || y != tt.cy {
				t.Fatalf("cursor %d,%d, want %d,%d:\n%s", x, y, tt.cx, tt.cy, ansi.Strip(out))
			}
		})
	}
}

// TestTopBarDropsInOrder: as the row narrows, the cost goes first, then
// the gauge, the mode, and the title; the model always stays.
func TestTopBarDropsInOrder(t *testing.T) {
	th := theme.Default()
	o := TopBarOptions{Title: "Fix the rename bug", Gauge: "ctx 31%", Cost: "$0.42 est.", Model: "anthropic/claude-sonnet-5-5 high", Mode: "mode: ask"}
	steps := []struct {
		width int
		has   []string
		not   []string
	}{
		{120, []string{"Fix the rename bug", "ctx 31%", "$0.42 est.", "claude-sonnet-5-5 high", "mode: ask"}, nil},
		{80, []string{"Fix the rename bug", "ctx 31%", "claude-sonnet-5-5 high", "mode: ask"}, []string{"$0.42"}},
		{70, []string{"Fix the rename bug", "claude-sonnet-5-5 high", "mode: ask"}, []string{"$0.42", "ctx"}},
		{55, []string{"Fix the rename bug", "claude-sonnet-5-5 high"}, []string{"$0.42", "ctx", "mode"}},
		{40, []string{"anthropic/claude-sonnet-5-5 high"}, []string{"$0.42", "ctx", "mode", "Fix"}},
		{20, []string{"anthropic/claude"}, []string{"Fix"}},
	}
	for _, s := range steps {
		o.Width = s.width
		out := TopBar(th, o)
		if w := ansi.StringWidth(out); w != s.width {
			t.Fatalf("width %d: bar is %d wide", s.width, w)
		}
		text := ansi.Strip(out)
		for _, want := range s.has {
			if !strings.Contains(text, want) {
				t.Errorf("width %d: no %q in %q", s.width, want, text)
			}
		}
		for _, gone := range s.not {
			if strings.Contains(text, gone) {
				t.Errorf("width %d: %q kept in %q", s.width, gone, text)
			}
		}
	}
	for _, w := range []int{1, 2, 3} {
		o.Width = w
		if got := ansi.StringWidth(TopBar(th, o)); got != w {
			t.Fatalf("width %d: %d", w, got)
		}
	}
}

// TestTopBarSpans: every part reports the cells it covers, and the text
// there is the part's text, cut only when the row is too narrow.
func TestTopBarSpans(t *testing.T) {
	th := theme.Default()
	o := TopBarOptions{Title: "Fix the bug", Gauge: "ctx 31%", Cost: "$0.42 est.", Model: "anthropic/claude-sonnet-5-5 high", Mode: "mode: ask"}
	want := map[string]string{
		TopBarTitle: "Fix the bug",
		TopBarGauge: "ctx 31%",
		TopBarCost:  "$0.42 est.",
		TopBarModel: "anthropic/claude-sonnet-5-5 high",
		TopBarMode:  "mode: ask",
	}
	for _, w := range []int{120, 80, 70, 55, 40, 20, 3, 2, 1} {
		o.Width = w
		row, spans := TopBarRow(th, o)
		if got := ansi.StringWidth(row); got != w {
			t.Fatalf("width %d: row is %d wide", w, got)
		}
		text := []rune(ansi.Strip(row))
		seen := map[string]bool{}
		for _, s := range spans {
			if s.X < 0 || s.W <= 0 || s.X+s.W > len(text) {
				t.Fatalf("width %d: span %+v outside %q", w, s, string(text))
			}
			got := string(text[s.X : s.X+s.W])
			full := want[s.Kind]
			if !strings.HasPrefix(full, got) && !strings.HasPrefix(got, full) {
				t.Errorf("width %d: %s covers %q, want %q", w, s.Kind, got, full)
			}
			if seen[s.Kind] {
				t.Errorf("width %d: two %s spans", w, s.Kind)
			}
			seen[s.Kind] = true
		}
		if w >= 2 && !seen[TopBarModel] {
			t.Errorf("width %d: no model span", w)
		}
	}
}

// TestTopBarFitsEveryWidth: the bar is exactly Width cells wide at every
// width, with and without the optional parts.
func TestTopBarFitsEveryWidth(t *testing.T) {
	th := theme.Default()
	sets := []TopBarOptions{
		{Title: "Fix the rename bug", Gauge: "ctx 31%", Cost: "$0.42 est.", Model: "anthropic/claude-sonnet-5-5 high", Mode: "mode: ask"},
		{Model: "anthropic/claude-sonnet-5-5"},
		{Model: "no model", Mode: "mode: ask"},
		{Title: "修正する", Model: "模型/名前 high"},
	}
	for i, o := range sets {
		for w := 1; w <= 130; w++ {
			o.Width = w
			if got := ansi.StringWidth(TopBar(th, o)); got != w {
				t.Fatalf("set %d, width %d: bar is %d wide", i, w, got)
			}
		}
	}
}
