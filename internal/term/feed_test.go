package term

import (
	"strings"
	"testing"
)

// TestFeedKeepsCharactersWhole: output is fed to the emulator in pieces,
// and the emulator ends a character at the end of each write. A piece edge
// inside "e" + combining accent, or inside a multi-byte rune, must not
// break the character.
func TestFeedKeepsCharactersWhole(t *testing.T) {
	tests := []struct {
		name, char string
	}{
		{"combining accent", "é"},
		{"joined emoji", "\U0001F468\u200d\U0001F469"},
		{"wide rune", "漢"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, f := newTestPane(t)
			// The character starts one byte before the first piece edge.
			f.shellWrites(strings.Repeat("x", feedPiece-1) + tt.char + "z\r\n")
			eventually(t, "the line", func() bool { return strings.Contains(p.Text(), "z") })
			if !strings.Contains(p.Text(), tt.char+"z") {
				t.Fatalf("%q was broken at a piece edge", tt.char)
			}
		})
	}
}
