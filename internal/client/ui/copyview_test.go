package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

func TestCopyViewSizes(t *testing.T) {
	th := theme.Default()
	lines := []string{"\x1b[31mred\x1b[0m line", "漢字 wide", "a very long row that must be cut at the right edge of the pane"}
	tests := []struct {
		name          string
		width, height int
		indicator     string
	}{
		{"typical", 20, 4, "[3/9]"},
		{"no indicator", 20, 4, ""},
		{"indicator wider than the row", 4, 3, "[1234/5678]"},
		{"one column", 1, 3, "[1/2]"},
		{"zero width", 0, 3, "[1/2]"},
		{"zero height", 10, 0, "[1/2]"},
		{"wide characters", 7, 2, "[1/2]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyView(th, CopyViewOptions{Width: tt.width, Height: tt.height, Lines: lines, Indicator: tt.indicator})
			if tt.width <= 0 || tt.height <= 0 {
				if got != "" {
					t.Fatalf("got %q, want empty for an empty size", got)
				}
				return
			}
			rows := strings.Split(got, "\n")
			if len(rows) != tt.height {
				t.Fatalf("%d rows, want %d", len(rows), tt.height)
			}
			for i, r := range rows {
				if w := ansi.StringWidth(r); w != tt.width {
					t.Errorf("row %d is %d cells wide, want %d: %q", i, w, tt.width, r)
				}
			}
		})
	}
}

func TestCopyViewIndicatorSitsInTheCorner(t *testing.T) {
	got := CopyView(theme.Default(), CopyViewOptions{Width: 20, Height: 2, Lines: []string{"hello"}, Indicator: "[3/9]"})
	first := ansi.Strip(strings.Split(got, "\n")[0])
	if first != "hello          [3/9]" {
		t.Fatalf("first row %q, want the indicator at the right end", first)
	}
}
