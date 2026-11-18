package files

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// tree is the state the tests draw: a small project with git marks.
func tree() api.FilesState {
	return api.FilesState{
		Folder:   "/home/u/snow",
		Expanded: []string{"", "internal", "internal/server"},
		Folders: []api.FilesFolder{
			{Path: "", Entries: []api.FilesEntry{
				{Name: "internal", Path: "internal", Dir: true},
				{Name: "pkg", Path: "pkg", Dir: true},
				{Name: "README.md", Path: "README.md", Mark: "M"},
				{Name: "go.mod", Path: "go.mod"},
			}},
			{Path: "internal", Entries: []api.FilesEntry{
				{Name: "client", Path: "internal/client", Dir: true},
				{Name: "server", Path: "internal/server", Dir: true},
				{Name: "files", Path: "internal/files", Dir: true},
			}},
			{Path: "internal/server", Entries: []api.FilesEntry{
				{Name: "chat.go", Path: "internal/server/chat.go"},
				{Name: "files.go", Path: "internal/server/files.go", Mark: "A"},
				{Name: "gone.go", Path: "internal/server/gone.go", Mark: "D", Deleted: true},
			}},
		},
		Selected: "internal/server/files.go",
	}
}

func model(st api.FilesState, width, height int) *Model {
	m := New(7, theme.Default())
	m.SetSize(width, height)
	m.SetState(st)
	return m
}

func key(text string) tea.Key {
	switch text {
	case "enter":
		return tea.Key{Code: tea.KeyEnter}
	case "up":
		return tea.Key{Code: tea.KeyUp}
	case "down":
		return tea.Key{Code: tea.KeyDown}
	case "right":
		return tea.Key{Code: tea.KeyRight}
	case "left":
		return tea.Key{Code: tea.KeyLeft}
	}
	return tea.Key{Text: text}
}

// TestFilesEnterOpensFile: Enter on a file asks the server to open it.
func TestFilesEnterOpensFile(t *testing.T) {
	m := model(tree(), 80, 24)
	cmd, ok := m.Key(key("enter")).(*api.Command)
	if !ok || cmd.Op != api.OpOpenFile || cmd.Target != 7 || cmd.Args[api.ArgPath] != "internal/server/files.go" {
		t.Fatalf("Enter sent %+v, want open-file for the selected file", cmd)
	}
}

// TestFilesEnterTogglesFolder: Enter on a folder opens it, and Enter again
// closes it.
func TestFilesEnterTogglesFolder(t *testing.T) {
	closed := tree()
	closed.Expanded = []string{""}
	m := model(closed, 80, 24)
	m.move(m.index("internal"))
	a, ok := m.Key(key("enter")).(*api.SidePaneAction)
	if !ok || a.Action != api.FilesActExpand || a.Args[api.ArgPath] != "internal" {
		t.Fatalf("Enter on a closed folder sent %+v, want expand", a)
	}
	st := tree()
	st.Expanded = []string{"", "internal"}
	m.SetState(st)
	m.move(m.index("internal"))
	a, ok = m.Key(key("enter")).(*api.SidePaneAction)
	if !ok || a.Action != api.FilesActCollapse {
		t.Fatalf("Enter on an open folder sent %+v, want collapse", a)
	}
}

// TestFilesLeftGoesToParent: Left on a file selects the folder that holds
// it.
func TestFilesLeftGoesToParent(t *testing.T) {
	m := model(tree(), 80, 24)
	m.Key(key("left"))
	if m.sel != "internal/server" {
		t.Fatalf("selection %q after Left, want internal/server", m.sel)
	}
	a, ok := m.Key(key("left")).(*api.SidePaneAction)
	if !ok || a.Action != api.FilesActCollapse || a.Args[api.ArgPath] != "internal/server" {
		t.Fatalf("Left on an open folder sent %+v, want collapse", a)
	}
	st := tree()
	st.Expanded = []string{"", "internal"}
	m.SetState(st)
	m.move(m.index("internal/server"))
	m.Key(key("left"))
	if m.sel != "internal" {
		t.Fatalf("selection %q after Left on a closed folder, want internal", m.sel)
	}
}

// TestFilesCollapsedFolderHidesChildren: a folder not in Expanded has no
// rows of its own.
func TestFilesCollapsedFolderHidesChildren(t *testing.T) {
	st := tree()
	st.Expanded = []string{""}
	m := model(st, 80, 24)
	for _, r := range m.rows() {
		if strings.HasPrefix(r.entry.Path, "internal/") {
			t.Fatalf("row %q is visible below a closed folder", r.entry.Path)
		}
	}
}

// TestFilesServerSelectionReveals: a Selected path from the server moves
// the selection once, and later local moves stay.
func TestFilesServerSelectionReveals(t *testing.T) {
	m := model(tree(), 80, 24)
	m.move(m.index("README.md"))
	st := tree()
	st.Selected = "internal/server/chat.go"
	m.SetState(st)
	if m.sel != "internal/server/chat.go" {
		t.Fatalf("selection %q, want the server's reveal", m.sel)
	}
	m.move(m.index("README.md"))
	m.SetState(st) // the same reveal must not move it again
	if m.sel != "README.md" {
		t.Fatalf("selection %q after moving, want README.md", m.sel)
	}
}

// TestFilesClickSelectsThenOpens: one click selects, the next opens.
func TestFilesClickSelectsThenOpens(t *testing.T) {
	m := model(tree(), 80, 24)
	m.move(m.index("internal"))
	row := -1
	for i, r := range m.rows() {
		if r.entry.Path == "internal/server/files.go" {
			row = i
		}
	}
	if row < 0 {
		t.Fatal("the file has no row")
	}
	y := row - m.top + 1
	if got := m.Click(0, y); got != nil {
		t.Fatalf("first click sent %+v, want selection only", got)
	}
	cmd, ok := m.Click(0, y).(*api.Command)
	if !ok || cmd.Args[api.ArgPath] != "internal/server/files.go" {
		t.Fatalf("second click sent %+v, want open-file", cmd)
	}
}

// TestFilesPaneGolden draws the tree at both sizes in every builtin theme.
func TestFilesPaneGolden(t *testing.T) {
	for _, th := range theme.Builtin() {
		for _, size := range [][2]int{{80, 24}, {120, 40}} {
			t.Run(fmt.Sprintf("%s_%dx%d", th.Name, size[0], size[1]), func(t *testing.T) {
				m := New(7, th)
				m.SetSize(size[0], size[1])
				m.SetState(tree())
				golden.RequireEqual(t, strings.Join(m.View(), "\n"))
			})
		}
	}
}
