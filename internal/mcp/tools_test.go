package mcp

import (
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

// TestFormatRead pins the text an agent reads for one pane.
func TestFormatRead(t *testing.T) {
	tests := []struct {
		name string
		read *api.ReadPane
		want string
	}{
		{
			name: "screen and scrollback",
			read: &api.ReadPane{
				Pane: 2, Session: "work", Profile: "bash", Folder: "/src", Cols: 80, Rows: 3,
				CursorX: 4, CursorY: 2, CursorVisible: true,
				Screen: []string{"$ ls", "a.txt", "$ "}, Scrollback: []string{"tmp", "src"}, Omitted: 7,
			},
			want: "Session \"work\", pane 2: bash, /src, 80x3, cursor at column 5, row 3.\n" +
				"Screen, top row first:\n$ ls\na.txt\n$ \n" +
				"\nScrollback, the 2 newest lines above the screen, oldest first:\ntmp\nsrc" +
				"\n\n7 older lines were left out.",
		},
		{
			name: "quiet ended pane",
			read: &api.ReadPane{Pane: 1, Session: "api", Profile: "Claude Code", Cols: 40, Rows: 2, Screen: []string{"done"}, Ended: true},
			want: "Session \"api\", pane 1: Claude Code, no folder, 40x2, the program ended, no cursor.\n" +
				"Screen, top row first:\ndone",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatRead(tt.read); got != tt.want {
				t.Errorf("formatRead:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

// TestFormatPanes pins the text an agent reads for its session.
func TestFormatPanes(t *testing.T) {
	tests := []struct {
		name  string
		panes *api.Panes
		want  string
	}{
		{
			name: "two panes",
			panes: &api.Panes{Session: "work", Caller: 1, Panes: []api.PaneSummary{
				{ID: 1, Window: 0, WindowName: "sh", Shown: true, Profile: "bash", Folder: "/src", Rect: api.Rect{W: 80, H: 24}, Focused: true},
				{ID: 2, Window: 1, WindowName: "dev", Profile: "Claude Code", Rect: api.Rect{W: 60, H: 20}, Agent: true, AgentState: api.AgentWorking},
			}},
			want: "Session \"work\", your pane is 1.\n" +
				"2 terminal pane(s):\n" +
				"pane 1: window 0 \"sh\", bash, /src, 80x24, focused, this is you\n" +
				"pane 2: window 1 \"dev\", Claude Code, no folder, 60x20, in a window that is not shown, agent: working",
		},
		{
			name: "hidden ended pane",
			panes: &api.Panes{Session: "api", Caller: 3, Panes: []api.PaneSummary{
				{ID: 3, Window: 2, WindowName: "tests", Profile: "zsh", Folder: "/a\nb", Rect: api.Rect{W: 10, H: 4}, Ended: true},
			}},
			want: "Session \"api\", your pane is 3.\n" +
				"1 terminal pane(s):\n" +
				"pane 3: window 2 \"tests\", zsh, /a b, 10x4, in a window that is not shown, ended, this is you",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatPanes(tt.panes); got != tt.want {
				t.Errorf("formatPanes:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}
