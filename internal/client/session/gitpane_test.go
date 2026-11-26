package session

import (
	tea "charm.land/bubbletea/v2"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

// gitScreen is a session with a shell on the left and a focused git pane
// on the right, showing one modified file.
func gitScreen(t *testing.T, cols, rows int) (*Model, *recorder) {
	t.Helper()
	r := &recorder{}
	m := New(r.send)
	m.now = fixedClock
	m.SetSize(cols, rows)
	left := cols / 2
	m.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "sh", Panes: []api.PaneInfo{
		{ID: 1, Kind: api.PaneTerminal, Rect: api.Rect{Y: 1, W: left, H: rows - 3}, Profile: "sh"},
		{ID: 2, Kind: api.PaneGit, Rect: api.Rect{X: left + 1, Y: 1, W: cols - left - 1, H: rows - 3}, Focused: true},
	}}}})
	m.Update(&api.PaneUpdate{Pane: 1, Lines: []api.LineUpdate{{Row: 0, Text: "$ git status"}}})
	m.Update(gitState(t, 1, api.GitState{Folder: "/home/u/snow", Branch: "main", Changes: []api.GitChange{{Path: "README.md", Work: "M"}}}))
	return m, r
}

func TestGitNoteStaysOffStatusLine(t *testing.T) {
	m, _ := gitScreen(t, 80, 24)
	git := m.own[2].(gitPane)
	if _, note := git.Key(tea.Key{Code: 'u', Text: "u"}); note != "" {
		t.Errorf("git returned status-line note %q", note)
	}
	if !strings.Contains(strings.Join(git.View(), "\n"), "Select a staged file to unstage.") {
		t.Fatal("git pane lost its note")
	}
	m.Update(press('u', 0, "u"))
	rows := strings.Split(content(m), "\n")
	if strings.Contains(rows[len(rows)-1], "staged file") {
		t.Fatalf("git note reached the session status line: %q", rows[len(rows)-1])
	}
}

func gitState(t *testing.T, rev uint64, st api.GitState) *api.SidePaneState {
	t.Helper()
	msg, err := api.NewSidePaneState(2, rev, &st)
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

func gitPaneGolden(t *testing.T, cols, rows int) {
	m, _ := gitScreen(t, cols, rows)
	golden.RequireEqual(t, m.View().Content)
}

func TestGitPaneGolden80x24(t *testing.T)  { gitPaneGolden(t, 80, 24) }
func TestGitPaneGolden120x40(t *testing.T) { gitPaneGolden(t, 120, 40) }

// TestGitPaneKeys: a key in the focused git pane goes to the server as the
// pane's action.
func TestGitPaneKeys(t *testing.T) {
	m, r := gitScreen(t, 80, 24)
	m.Update(press('s', 0, "s"))
	if len(r.sent) == 0 {
		t.Fatal("s sent nothing")
	}
	a, ok := r.sent[len(r.sent)-1].(*api.SidePaneAction)
	if !ok || a.Pane != 2 || a.Action != api.GitActStage || a.Args[api.ArgGitPath] != "README.md" {
		t.Fatalf("sent %+v, want stage README.md on pane 2", r.sent[len(r.sent)-1])
	}
}

// TestGitPaneOlderState: a state older than the one shown is dropped, and
// a state this client cannot read shows as an error.
func TestGitPaneOlderState(t *testing.T) {
	m, _ := gitScreen(t, 80, 24)
	m.Update(gitState(t, 3, api.GitState{Folder: "/home/u/snow", Branch: "newer"}))
	m.Update(gitState(t, 2, api.GitState{Folder: "/home/u/snow", Branch: "older"}))
	if v := m.View().Content; !strings.Contains(v, "newer") || strings.Contains(v, "older") {
		t.Fatal("an older state replaced a newer one")
	}
	m.Update(&api.SidePaneState{Pane: 2, Kind: api.PaneGit, Rev: 4, State: json.RawMessage(`{"ahead":-1}`)})
	if v := m.View().Content; !strings.Contains(v, "cannot read") {
		t.Fatal("a bad state does not say so")
	}
}

func TestGitNoteGolden(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m, _ := gitScreen(t, size[0], size[1])
			m.Update(press('u', 0, "u"))
			golden.RequireEqual(t, content(m))
		})
	}
}
