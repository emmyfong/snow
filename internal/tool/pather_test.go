package tool

import "testing"

// TestToolsThatNamePathsArePathers: every built-in tool whose input has a
// path declares it, or the folder check could not see it.
func TestToolsThatNamePathsArePathers(t *testing.T) {
	for _, name := range []string{"read", "list", "search", "edit", "write"} {
		tl, _ := Builtin().Get(name)
		if _, ok := tl.(Pather); !ok {
			t.Errorf("%s does not implement Pather", name)
		}
	}
}
