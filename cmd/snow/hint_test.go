package main

import (
	"runtime"
	"strings"
	"testing"
)

// TestUnsetHintFitsTheShell: the escape hatch names a command the user's
// shells have.
func TestUnsetHintFitsTheShell(t *testing.T) {
	want := []string{"unset SNOW"}
	if runtime.GOOS == "windows" {
		want = []string{"set SNOW=", "Remove-Item Env:SNOW"}
	}
	for _, w := range want {
		if !strings.Contains(unsetHint(), w) {
			t.Errorf("hint %q lacks %q", unsetHint(), w)
		}
	}
}
