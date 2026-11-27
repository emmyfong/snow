package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

func TestButtonRow(t *testing.T) {
	for _, tt := range []struct {
		name  string
		o     ButtonRowOptions
		spans []ButtonSpan
	}{
		{"all", ButtonRowOptions{Width: 30, Choices: []string{"Stage", "Diff"}, Right: "Commit..."}, []ButtonSpan{{Index: 0, X: 0, W: 7}, {Index: 1, X: 8, W: 6}, {Index: 2, X: 19, W: 11}}},
		{"drop last", ButtonRowOptions{Width: 22, Choices: []string{"Stage", "Diff"}, Right: "Commit..."}, []ButtonSpan{{Index: 0, X: 0, W: 7}, {Index: 2, X: 11, W: 11}}},
		{"right only", ButtonRowOptions{Width: 11, Choices: []string{"Stage"}, Right: "Commit..."}, []ButtonSpan{{Index: 1, X: 0, W: 11}}},
		{"right too wide", ButtonRowOptions{Width: 7, Choices: []string{"Stage"}, Right: "Commit..."}, []ButtonSpan{{Index: 0, X: 0, W: 7}}},
		{"wide label", ButtonRowOptions{Width: 12, Choices: []string{"界", "B"}}, []ButtonSpan{{Index: 0, X: 0, W: 4}, {Index: 1, X: 5, W: 3}}},
		{"zero", ButtonRowOptions{}, nil},
		{"negative", ButtonRowOptions{Width: -1, Choices: []string{"A"}}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := ButtonRowSpans(tt.o)
			if !slices.Equal(got, tt.spans) {
				t.Fatalf("spans %+v, want %+v", got, tt.spans)
			}
			row := ansi.Strip(ButtonRow(theme.Default(), tt.o))
			if strings.Contains(row, "\n") || ansi.StringWidth(row) != max(tt.o.Width, 0) {
				t.Fatalf("row %q has width %d", row, ansi.StringWidth(row))
			}
			for _, sp := range got {
				label := tt.o.Right
				if sp.Index < len(tt.o.Choices) {
					label = tt.o.Choices[sp.Index]
				}
				if text := ansi.Cut(row, sp.X, sp.X+sp.W); text != " "+label+" " {
					t.Errorf("span %+v covers %q, want %q", sp, text, label)
				}
			}
		})
	}
}
