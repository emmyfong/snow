package term

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// printLines plays a shell printing lines, one per row, and waits for the
// last. Line i is then line i of the history.
func printLines(t *testing.T, p *Pane, f *fakePTY, lines []string) {
	t.Helper()
	f.shellWrites(strings.Join(lines, "\r\n") + "\r\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	eventually(t, last, func() bool { return strings.Contains(p.Text(), last) })
}

// searchText is 40 lines: 30 of filler, then lines that reach the screen, so
// matches fall in the scrollback and on the screen.
func searchText() []string {
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = fmt.Sprintf("filler %d", i)
	}
	lines[3] = "error: disk full"
	lines[5] = "an error then another error"
	lines[9] = "Error with a capital"
	lines[12] = "漢字 error after wide text"
	lines[30] = "last error on the screen"
	return lines
}

func TestSearchFindsAll(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []Match
	}{
		{"every match in order, ignoring case", "error", []Match{
			{3, 0, 5}, {5, 3, 5}, {5, 22, 5}, {9, 0, 5}, {12, 5, 5}, {30, 5, 5},
		}},
		{"an upper-case letter makes it exact", "Error", []Match{{9, 0, 5}}},
		{"no match", "warning", nil},
		{"empty query", "", nil},
		{"matches do not overlap", "rr", []Match{
			{3, 1, 2}, {5, 4, 2}, {5, 23, 2}, {9, 1, 2}, {12, 6, 2}, {30, 6, 2},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, f := newTestPane(t)
			printLines(t, p, f, searchText())
			if p.ScrollbackLen() < 10 {
				t.Fatalf("scrollback is %d lines: the test needs matches above the screen", p.ScrollbackLen())
			}
			got, truncated, _ := p.Search(tt.query)
			if truncated {
				t.Fatal("truncated")
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Search(%q) =\n %v\nwant\n %v", tt.query, got, tt.want)
			}
		})
	}
}

// TestSearchLines: each match comes with the plain text of its own line,
// read with the search, so a caller never has to look the line up again.
func TestSearchLines(t *testing.T) {
	p, f := newTestPane(t)
	printLines(t, p, f, searchText())
	got := p.SearchLines("error")
	want := []string{
		"error: disk full",
		"an error then another error",
		"an error then another error",
		"Error with a capital",
		"漢字 error after wide text",
		"last error on the screen",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SearchLines = %q\nwant %q", got, want)
	}
}

func TestSearchTruncates(t *testing.T) {
	p, f := newTestPane(t)
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = strings.Repeat("x", 80)
	}
	// Each batch ends with its own line: printLines waits for it, and a
	// line every batch shares would be on screen before the batch is.
	for i := range MaxMatches/len(lines)/80 + 2 {
		printLines(t, p, f, append(lines, fmt.Sprintf("end of batch %d", i)))
	}
	got, truncated, _ := p.Search("x")
	if len(got) != MaxMatches || !truncated {
		t.Fatalf("got %d matches, truncated=%v; want %d and true", len(got), truncated, MaxMatches)
	}
	// The newest matches are kept, so ? from the bottom finds the nearest.
	_, rows := p.Size()
	if last := got[len(got)-1].Line; last < historyLen(p)-rows {
		t.Fatalf("last kept match is on line %d of %d: the newest matches were dropped", last, historyLen(p))
	}
	for i := 1; i < len(got); i++ {
		if got[i].Line < got[i-1].Line || (got[i].Line == got[i-1].Line && got[i].Col <= got[i-1].Col) {
			t.Fatalf("matches %d and %d are out of order: %v, %v", i-1, i, got[i-1], got[i])
		}
	}
}
