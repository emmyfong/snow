package chat

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// linkedChat is a chat with an answer that names a known file and a line.
func linkedChat(t *testing.T, w, h int) *Model {
	t.Helper()
	m, _ := newChat(w, h)
	m.Snapshot(&api.ChatSnapshot{Pane: 2, Agent: "c1", Conversation: "c1", Model: "anthropic/claude-sonnet-5-5",
		Mode: "ask", State: api.ChatStateIdle, Folder: "/work/project", Entries: []api.ChatEntry{
			{Kind: api.ChatEntryUser, Text: "Where is the list?"},
			{Kind: api.ChatEntryAssistant, Text: "The list is in internal/server/chat.go:12."},
		}})
	m.Files(&api.FileList{Pane: 2, Folder: "/work/project", Files: []string{"internal/server/chat.go"}})
	return m
}

// TestClickablePathGolden pins the link style on a path the server listed.
func TestClickablePathGolden(t *testing.T) {
	for _, th := range []struct {
		name  string
		theme theme.Theme
	}{{"Dark", theme.Default()}, {"Light", lightTheme()}} {
		t.Run(fmt.Sprintf("%s100x20", th.name), func(t *testing.T) {
			m := linkedChat(t, 100, 20)
			m.SetTheme(th.theme)
			golden.RequireEqual(t, strings.Join(m.View(), "\n"))
		})
	}
}
