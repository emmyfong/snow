package session

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/pkg/api"
)

// pathChat is a chat pane whose answer names a file of the folder, and
// whose folder's files the server listed.
func pathChat(t *testing.T) (*Model, *recorder) {
	t.Helper()
	m, r := chatScreen(t, 100, 30)
	m.Update(&api.ChatSnapshot{Pane: 2, Agent: "c1", Conversation: "c1", Model: "anthropic/claude-sonnet-5-5",
		Mode: "ask", State: api.ChatStateIdle, Folder: "/work/project", Entries: []api.ChatEntry{
			{Kind: api.ChatEntryAssistant, Text: "The fix is in internal/server/chat.go:12."},
		}})
	m.Update(&api.FileList{Pane: 2, Folder: "/work/project", Files: []string{"internal/server/chat.go"}})
	return m, r
}

// screenText returns the screen cell of the first text on screen.
func screenText(t *testing.T, m *Model, text string) (x, y int) {
	t.Helper()
	for i, row := range strings.Split(ansi.Strip(content(m)), "\n") {
		if j := strings.Index(row, text); j >= 0 {
			return ansi.StringWidth(row[:j]) + 1, i
		}
	}
	t.Fatalf("%q is not on screen:\n%s", text, content(m))
	return 0, 0
}

// TestPathClickOpensFile: a click on a path in a reply asks for the viewer
// at that path and line.
func TestPathClickOpensFile(t *testing.T) {
	m, r := pathChat(t)
	x, y := screenText(t, m, "internal/server/chat.go:12")
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m.Update(tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
	c := lastCommandOf(r, api.OpOpenFile)
	if c == nil || c.Target != 2 || c.Args[api.ArgPath] != "internal/server/chat.go" || c.Args[api.ArgLine] != "12" {
		t.Fatalf("sent %+v, want open-file internal/server/chat.go:12", c)
	}
}

// TestDragSelectsNotClicks: a left press that moves over a path is a text
// selection, so it sends no open-file.
func TestDragSelectsNotClicks(t *testing.T) {
	m, r := pathChat(t)
	x, y := screenText(t, m, "internal/server/chat.go:12")
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m.Update(tea.MouseMotionMsg{X: x + 4, Y: y, Button: tea.MouseLeft})
	m.Update(tea.MouseReleaseMsg{X: x + 4, Y: y, Button: tea.MouseLeft})
	if c := lastCommandOf(r, api.OpOpenFile); c != nil {
		t.Fatalf("a drag sent %+v", c)
	}
}

// TestPathClickUnlistedShowsNote: a click on a file-like path the server did
// not list shows a status-bar note and opens nothing.
func TestPathClickUnlistedShowsNote(t *testing.T) {
	m, r := pathChat(t)
	m.Update(&api.ChatSnapshot{Pane: 2, Agent: "c1", Conversation: "c1", Model: "anthropic/claude-sonnet-5-5",
		Mode: "ask", State: api.ChatStateIdle, Folder: "/work/project", Entries: []api.ChatEntry{
			{Kind: api.ChatEntryAssistant, Text: "It moved to pkg/gone.go."},
		}})
	x, y := screenText(t, m, "pkg/gone.go")
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m.Update(tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
	if c := lastCommandOf(r, api.OpOpenFile); c != nil {
		t.Fatalf("an unlisted path sent %+v", c)
	}
	if !strings.Contains(m.message, "pkg/gone.go") {
		t.Fatalf("the status message %q does not name the path", m.message)
	}
}
