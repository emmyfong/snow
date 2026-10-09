package term

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// writeLines plays a shell printing "line 0", "line 1", ... and waits until
// the last one is on screen, so every earlier line has been parsed.
func writeLines(t *testing.T, p *Pane, f *fakePTY, n int) {
	t.Helper()
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "line %d\r\n", i)
	}
	f.shellWrites(b.String())
	last := fmt.Sprintf("line %d", n-1)
	eventually(t, last, func() bool { return strings.Contains(p.Text(), last) })
}

func lineNumber(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(s), "line "))
	if err != nil {
		t.Fatalf("not a numbered line: %q", s)
	}
	return n
}

func TestScrollbackLimit(t *testing.T) {
	tests := []struct {
		name  string
		opts  Options
		write int
		want  int
	}{
		{"default keeps 2,000", Options{}, 2100, DefaultScrollback},
		{"configured limit", Options{Scrollback: 500}, 900, 500},
		{"under the limit keeps all", Options{Scrollback: 500}, 100, 100 - 23},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakePTY()
			p := newPane(f, 80, 24, tt.opts)
			t.Cleanup(func() { _ = p.Close() })
			writeLines(t, p, f, tt.write)
			if got := p.ScrollbackLen(); got != tt.want {
				t.Fatalf("ScrollbackLen() = %d, want %d", got, tt.want)
			}
			lines := p.ScrollbackLines(0, tt.want)
			first := lineNumber(t, ansiStripped(lines[0]))
			for i, l := range lines {
				if got := lineNumber(t, ansiStripped(l)); got != first+i {
					t.Fatalf("line %d is %q, want line %d (oldest first, no gaps)", i, l, first+i)
				}
			}
		})
	}
}

func TestScrollbackLinesKeepStyles(t *testing.T) {
	p, f := newTestPane(t)
	f.shellWrites("\x1b[31mred text\x1b[0m\r\n")
	writeLines(t, p, f, 30)
	lines := p.ScrollbackLines(0, 1)
	if len(lines) != 1 || !strings.Contains(lines[0], "red text") || !strings.Contains(lines[0], "\x1b[") {
		t.Fatalf("oldest line %q, want styled red text", lines)
	}
}

func TestScrollbackLinesBounds(t *testing.T) {
	p, f := newTestPane(t)
	writeLines(t, p, f, 40)
	total := p.ScrollbackLen()
	tests := []struct {
		name    string
		from, n int
		wantLen int
	}{
		{"all", 0, total, total},
		{"clipped at the end", total - 2, 10, 2},
		{"past the end", total, 5, 0},
		{"negative start", -3, 2, 0},
		{"zero count", 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := len(p.ScrollbackLines(tt.from, tt.n)); got != tt.wantLen {
				t.Fatalf("ScrollbackLines(%d, %d) returned %d lines, want %d", tt.from, tt.n, got, tt.wantLen)
			}
		})
	}
}
