package gitpane

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

// goldenStates are the states the goldens draw.
func goldenStates() map[string]func() *Model {
	diff := func() api.GitState {
		st := changes()
		st.Diff = &api.GitDiff{Path: "internal/server/gitpane.go", Lines: []api.GitDiffLine{
			{Kind: api.GitLineHunk, Text: "@@ -12,6 +12,7 @@ func gitKind() sideKind {"},
			{Kind: api.GitLineKeep, Text: "\treturn sideKind{"},
			{Kind: api.GitLineRemove, Text: "\t\tName: \"git\","},
			{Kind: api.GitLineAdd, Text: "\t\tName: api.PaneGit,"},
			{Kind: api.GitLineAdd, Text: "\t\tTitle: \"Git\","},
			{Kind: api.GitLineKeep, Text: "\t}"},
		}}
		return st
	}
	return map[string]func() *Model{
		"Changes": func() *Model { return model(changes(), 0, 0) },
		"Empty": func() *Model {
			return model(api.GitState{Folder: "/home/u/snow", Branch: "main"}, 0, 0)
		},
		"NotRepo": func() *Model { return model(api.GitState{Folder: "/home/u/notes", NotRepo: true}, 0, 0) },
		"GitError": func() *Model {
			return model(api.GitState{Folder: "/home/u/snow", Error: "git status: fatal: unsafe repository"}, 0, 0)
		},
		"Loading": func() *Model {
			st := changes()
			st.Loading = true
			return model(st, 0, 0)
		},
		"Diff": func() *Model { return model(diff(), 0, 0) },
		"Commit": func() *Model {
			m := model(changes(), 0, 0)
			m.Key(key("c"))
			for _, s := range strings.Split("Add the git pane", "") {
				m.Key(key(s))
			}
			return m
		},
		"Committed": func() *Model {
			st := changes()
			st.Changes = st.Changes[3:]
			st.Notice = "Committed 3f9c2ab"
			return model(st, 0, 0)
		},
	}
}

// TestGitPaneGolden draws each state at a full window's size and at the
// size of a side pane.
func TestGitPaneGolden(t *testing.T) {
	for name, mk := range goldenStates() {
		for _, size := range [][2]int{{80, 24}, {120, 40}, {30, 20}} {
			t.Run(fmt.Sprintf("%s_%dx%d", name, size[0], size[1]), func(t *testing.T) {
				m := mk()
				m.SetSize(size[0], size[1])
				rows := m.View()
				if len(rows) != size[1] {
					t.Fatalf("%d rows, want %d", len(rows), size[1])
				}
				golden.RequireEqual(t, strings.Join(rows, "\n"))
			})
		}
	}
}
