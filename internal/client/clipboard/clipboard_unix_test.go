//go:build !windows

package clipboard

import (
	"reflect"
	"testing"
)

// TestSetAsksTheTerminal: off Windows, Set hands the text to Bubble Tea's
// OSC 52 command, which writes the escape sequence to the outer terminal.
func TestSetAsksTheTerminal(t *testing.T) {
	cmd := Set("héllo 漢\nline two")
	if cmd == nil {
		t.Fatal("Set returned no command")
	}
	msg := cmd()
	v := reflect.ValueOf(msg)
	if v.Kind() != reflect.String || v.String() != "héllo 漢\nline two" {
		t.Fatalf("command produced %#v, want Bubble Tea's set-clipboard message holding the text", msg)
	}
}
