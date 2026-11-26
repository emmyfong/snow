package gitpane

import (
	"fmt"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
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
		"ChangeWords": func() *Model {
			st := changes()
			st.Changes = []api.GitChange{
				{Path: "internal/git/status.go", Index: "A"},
				{Path: "internal/server/gitpane.go", Work: "M"},
				{Path: "old.go", Work: "D"},
				{Path: "docs/new.md", Orig: "docs/old.md", Index: "R"},
				{Path: "copy.go", Index: "C"},
				{Path: "main.go", Index: "U", Work: "U", Conflict: true},
				{Path: "notes/todo.txt", Work: "?"},
				{Path: "界/very/long/path/that/keeps/the/filename.go", Work: "T"},
			}
			return model(st, 0, 0)
		},
		"Empty": func() *Model {
			return model(api.GitState{Folder: "/home/u/snow", Branch: "main"}, 0, 0)
		},
		"NotRepo": func() *Model { return model(api.GitState{Folder: "/home/u/notes", NotRepo: true}, 0, 0) },
		"GitError": func() *Model {
			return model(api.GitState{Folder: "/home/u/snow", Error: "git status: fatal: unsafe repository"}, 0, 0)
		},
		"LongGitError": func() *Model {
			return model(api.GitState{Folder: "/home/u/snow", Error: "git commit: " + strings.Repeat("reason ", 7) + "unmerged files remain; " + strings.Repeat("later ", 6)}, 0, 0)
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
		"CommitOmitted": func() *Model {
			st := changes()
			st.Omitted = 2000
			m := model(st, 0, 0)
			m.Key(key("c"))
			m.Paste("Commit the staged files")
			return m
		},
		"CommitMany": func() *Model {
			st := changes()
			st.Changes = nil
			for i := 1; i <= 7; i++ {
				st.Changes = append(st.Changes, api.GitChange{Path: fmt.Sprintf("internal/git/staged-%d.go", i), Index: "A"})
			}
			m := model(st, 0, 0)
			m.Key(key("c"))
			m.Paste("Commit the staged files")
			return m
		},
		"CommitEmpty": func() *Model {
			m := model(api.GitState{Folder: "/home/u/snow", Branch: "main"}, 0, 0)
			m.Key(key("c"))
			return m
		},
		"CommitReadError": func() *Model {
			m := model(api.GitState{Folder: "/home/u/snow", Error: "git status: fatal: unsafe repository"}, 0, 0)
			m.Key(key("c"))
			return m
		},
		"CommitLongError": func() *Model {
			st := changes()
			st.Error = "git commit: " + strings.Repeat("reason ", 7) + "unmerged files remain; " + strings.Repeat("later ", 6)
			m := model(st, 0, 0)
			m.Key(key("c"))
			m.Paste("Add the git pane\nKeep a useful body\nand the final line")
			return m
		},
		"CommitUnfocused": func() *Model {
			m := model(changes(), 0, 0)
			m.Key(key("c"))
			m.Paste("Keep the draft")
			m.SetFocused(false)
			return m
		},
		"Worktrees": func() *Model { return worktreeModel(0, 0) },
		"RemoveWorktree": func() *Model {
			st := api.GitState{Folder: "/home/u/snow", Branch: "main", Worktrees: []api.GitWorktree{
				{Path: "/home/u/snow", Branch: "main", Main: true},
				{Path: "/home/u/snow/.worktrees/clean", Branch: "clean"},
			}}
			m := model(st, 0, 0)
			m.Key(key("down"))
			m.Key(key("x"))
			return m
		},
		"History": func() *Model { return model(history(), 0, 0) },
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
		for _, size := range [][2]int{{80, 24}, {120, 40}, {30, 20}, {30, 8}} {
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

func TestGitPaneNoColorWords(t *testing.T) {
	st := api.GitState{Folder: "/src/snow", Branch: "main"}
	for i, code := range []string{"A", "M", "T", "D", "R", "C", "U", "?", "X"} {
		st.Changes = append(st.Changes, api.GitChange{Path: fmt.Sprintf("file-%d", i), Work: code})
	}
	m := model(st, 80, 24)
	m.SetTheme(testTheme().For(colorprofile.ASCII))
	view := ansi.Strip(strings.Join(m.View(), "\n"))
	for _, want := range []string{"added", "modified", "deleted", "renamed", "copied", "conflict", "untracked", "changed"} {
		if !strings.Contains(view, want) {
			t.Errorf("no-color view lacks %q:\n%s", want, view)
		}
	}
}
