package session

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

// claudeScreen is a session screen with one WSL pane in a project folder.
func claudeScreen(cols, rows int) (*Model, *recorder) {
	r := &recorder{}
	m := New(r.send)
	m.now = fixedClock
	m.SetSize(cols, rows)
	p := []api.PaneInfo{{ID: 1, Rect: api.Rect{W: cols, H: rows - 1}, Profile: "Ubuntu", Folder: "/home/emmy/api", Focused: true}}
	m.Update(&api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "Ubuntu", Panes: p}}})
	fill(m, p)
	return m, r
}

// TestCtrlBAOpensClaudeCode: Ctrl+b a asks the server for a Claude Code
// pane beside the focused one, in its folder. When claude is missing, the
// server's install hint shows in the status bar.
func TestCtrlBAOpensClaudeCode(t *testing.T) {
	for _, size := range []struct{ cols, rows int }{{80, 24}, {120, 40}} {
		m, r := claudeScreen(size.cols, size.rows)
		m.Update(ctrlB)
		m.Update(press('a', 0, "a"))
		c := lastCommand(t, r)
		want := map[string]string{api.ArgDir: api.DirRight, api.ArgAgent: api.AgentClaude, api.ArgFolder: "/home/emmy/api"}
		if c.Op != api.OpSplit || c.Target != 1 || len(c.Args) != len(want) {
			t.Fatalf("sent %+v, want a split of pane 1 with %v", c, want)
		}
		for k, v := range want {
			if c.Args[k] != v {
				t.Fatalf("arg %s = %q, want %q", k, c.Args[k], v)
			}
		}
		m.Update(&api.Error{Code: api.CodeCommandFailed, Message: "Claude Code not found in Ubuntu. Get it: claude.com/claude-code"})
		t.Run(fmt.Sprintf("%dx%d", size.cols, size.rows), func(t *testing.T) { golden.RequireEqual(t, m.View().Content) })
	}
}
