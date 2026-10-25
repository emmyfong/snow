package textdiff

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func show(lines []Line) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString([]string{" ", "+", "-"}[l.Op] + l.Text + "\n")
	}
	return b.String()
}

func TestLines(t *testing.T) {
	tests := []struct {
		name, old, new, want string
	}{
		{"same", "a\nb\n", "a\nb\n", " a\n b\n"},
		{"new file", "", "a\nb", "+a\n+b\n"},
		{"emptied", "a\n", "", "-a\n"},
		{"one line changed", "a\nb\nc\n", "a\nB\nc\n", " a\n-b\n+B\n c\n"},
		{"added in the middle", "a\nc\n", "a\nb\nc\n", " a\n+b\n c\n"},
		{"crlf", "a\r\nb\r\n", "a\nb\n", " a\n b\n"},
		{"moved", "x\na\nb\n", "a\nb\nx\n", "-x\n a\n b\n+x\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := show(Lines(tt.old, tt.new)); got != tt.want {
				t.Fatalf("got\n%swant\n%s", got, tt.want)
			}
		})
	}
}

// TestLinesRebuildsBoth: whatever the input, the kept and removed lines
// are the old text and the kept and added lines the new text.
func TestLinesRebuildsBoth(t *testing.T) {
	big := func(n, every int) string {
		var b strings.Builder
		for i := range n {
			if every > 0 && i%every == 0 {
				fmt.Fprintf(&b, "changed %d\n", i)
				continue
			}
			fmt.Fprintf(&b, "line %d\n", i)
		}
		return b.String()
	}
	pairs := [][2]string{
		{big(200, 0), big(200, 7)},
		{big(3000, 0), big(3000, 2)}, // past maxCells
		{"a\nb\nc\nd\n", "d\nc\nb\na\n"},
	}
	for i, p := range pairs {
		lines := Lines(p[0], p[1])
		var before, after []string
		for _, l := range lines {
			if l.Op != Add {
				before = append(before, l.Text)
			}
			if l.Op != Remove {
				after = append(after, l.Text)
			}
		}
		if !slices.Equal(before, split(p[0])) || !slices.Equal(after, split(p[1])) {
			t.Fatalf("pair %d does not rebuild", i)
		}
	}
	if a, r := Counts(Lines(big(200, 0), big(200, 7))); a != 29 || r != 29 {
		t.Fatalf("counts +%d -%d", a, r)
	}
}
