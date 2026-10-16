package term

import (
	"strings"
	"testing"
)

func TestHistoryIsScrollbackThenScreen(t *testing.T) {
	p, f := newTestPane(t)
	writeLines(t, p, f, 40)
	kept := p.ScrollbackLen()
	_, rows := p.Size()

	lines, total, _ := p.History(0, historyLen(p))
	if total != kept+rows || len(lines) != total {
		t.Fatalf("History() = %d lines, total %d; want %d (scrollback %d + screen %d)", len(lines), total, kept+rows, kept, rows)
	}
	first := lineNumber(t, ansiStripped(lines[0]))
	for i := range kept + 17 { // the screen's rows hold lines up to 39, then the cursor row
		if got := lineNumber(t, ansiStripped(lines[i])); got != first+i {
			t.Fatalf("history line %d is line %d, want %d: the screen must follow the scrollback with no gap", i, got, first+i)
		}
	}
}

func historyLen(p *Pane) int {
	_, n, _ := p.History(0, 1)
	return n
}

func TestHistoryBounds(t *testing.T) {
	p, f := newTestPane(t)
	writeLines(t, p, f, 40)
	n := historyLen(p)
	tests := []struct {
		name    string
		from, n int
		want    int
	}{
		{"all", 0, n, n},
		{"across the scrollback and screen", p.ScrollbackLen() - 2, 5, 5},
		{"clipped at the end", n - 2, 10, 2},
		{"past the end", n, 5, 0},
		{"negative start", -1, 3, 0},
		{"zero count", 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, got, _ := p.History(tt.from, tt.n)
			if len(lines) != tt.want || got != n {
				t.Fatalf("History(%d, %d) = %d lines, total %d; want %d lines, total %d", tt.from, tt.n, len(lines), got, tt.want, n)
			}
		})
	}
}

func TestHistoryKeepsStyles(t *testing.T) {
	p, f := newTestPane(t)
	f.shellWrites("\x1b[31mred text\x1b[0m\r\n")
	writeLines(t, p, f, 5)
	lines, _, _ := p.History(0, 5)
	if got := strings.TrimSpace(ansiStripped(lines[0])); got != "red text" || !strings.Contains(lines[0], "\x1b[31m") {
		t.Fatalf("line 0 = %q, want styled red text", lines[0])
	}
}
