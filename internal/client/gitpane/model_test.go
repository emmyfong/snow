package gitpane

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// changes is a state with a staged file, a file with staged and unstaged
// changes, a rename, a modified file, and an untracked one.
func changes() api.GitState {
	return api.GitState{
		Folder: "/home/u/snow", Branch: "feat/195-git-pane", Upstream: "origin/feat/195-git-pane", Ahead: 2, Behind: 1,
		Changes: []api.GitChange{
			{Path: "internal/git/status.go", Index: "A"},
			{Path: "internal/server/gitpane.go", Index: "M", Work: "M"},
			{Path: "docs/new.md", Orig: "docs/old.md", Index: "R"},
			{Path: "README.md", Work: "M"},
			{Path: "notes/todo.txt", Work: "?"},
		},
	}
}

func model(st api.GitState, w, h int) *Model {
	m := New(7, theme.Default())
	m.SetSize(w, h)
	m.SetState(st)
	return m
}

func key(s string) tea.Key {
	switch s {
	case "enter":
		return tea.Key{Code: tea.KeyEnter}
	case "esc":
		return tea.Key{Code: tea.KeyEscape}
	case "down":
		return tea.Key{Code: tea.KeyDown}
	case "up":
		return tea.Key{Code: tea.KeyUp}
	case "space":
		return tea.Key{Code: tea.KeySpace, Text: " "}
	case "ctrl+j":
		return tea.Key{Code: 'j', Mod: tea.ModCtrl}
	}
	r := []rune(s)[0]
	return tea.Key{Code: r, Text: s}
}

func wantAction(t *testing.T, got *api.SidePaneAction, action string, args map[string]string) {
	t.Helper()
	if got == nil || got.Pane != 7 || got.Action != action {
		t.Fatalf("action %+v, want %s on pane 7", got, action)
	}
	for k, v := range args {
		if got.Args[k] != v {
			t.Fatalf("action %+v, want %s=%q", got, k, v)
		}
	}
}

// TestListKeys: the rows run staged first, and each key sends its action
// for the selected row.
func TestListKeys(t *testing.T) {
	m := model(changes(), 80, 24)
	wantAction(t, m.Key(key("u")), api.GitActUnstage, map[string]string{api.ArgGitPath: "internal/git/status.go"})
	if a := m.Key(key("s")); a != nil {
		t.Fatalf("s on a staged-only file sent %+v", a)
	}
	if n := m.Note(); !strings.Contains(n, "already staged") {
		t.Errorf("note %q", n)
	}
	wantAction(t, m.Key(key("enter")), api.GitActDiff, map[string]string{api.ArgGitPath: "internal/git/status.go", api.ArgGitStaged: "true"})
	for range 3 {
		m.Key(key("down")) // past the staged gitpane.go and the rename, to the unstaged gitpane.go
	}
	wantAction(t, m.Key(key("space")), api.GitActStage, map[string]string{api.ArgGitPath: "internal/server/gitpane.go"})
	wantAction(t, m.Key(key("d")), api.GitActDiff, map[string]string{api.ArgGitStaged: "false"})
	wantAction(t, m.Key(key("a")), api.GitActStageAll, nil)
	wantAction(t, m.Key(key("r")), api.GitActRefresh, nil)
	if a := m.Key(key("u")); a == nil {
		t.Fatal("u on a file that also has a staged change sent nothing")
	}
}

// TestCommitBox: c opens the box, typed text and a new line go in, Enter
// sends the commit, and a commit that works clears the box.
func TestCommitBox(t *testing.T) {
	m := model(changes(), 80, 24)
	m.Key(key("c"))
	for _, s := range []string{"f", "i", "x"} {
		if a := m.Key(key(s)); a != nil {
			t.Fatalf("typing sent %+v", a)
		}
	}
	m.Key(key("ctrl+j"))
	m.Paste("body\r\n")
	wantAction(t, m.Key(key("enter")), api.GitActCommit, map[string]string{api.ArgGitMessage: "fix\nbody"})
	if a := m.Key(key("enter")); a != nil {
		t.Fatal("a second Enter sent another commit while the first waits")
	}
	done := changes()
	done.Changes = done.Changes[3:]
	done.Notice = "Committed abc1234"
	m.SetState(done)
	if m.mode != modeList || len(m.msg) != 0 {
		t.Errorf("after a commit that worked: mode %d, message %q", m.mode, string(m.msg))
	}
}

// TestCommitFailedKeepsMessage: a commit git refuses keeps the box and the
// message, and shows git's words.
func TestCommitFailedKeepsMessage(t *testing.T) {
	m := model(changes(), 80, 24)
	m.Key(key("c"))
	m.Key(key("x"))
	m.Key(key("enter"))
	failed := changes()
	failed.Error = "git commit: pre-commit hook failed"
	m.SetState(failed)
	if m.mode != modeCommit || string(m.msg) != "x" {
		t.Fatalf("after a failed commit: mode %d, message %q", m.mode, string(m.msg))
	}
	if !strings.Contains(strings.Join(m.View(), "\n"), "pre-commit hook failed") {
		t.Error("the pane does not show git's error")
	}
	m.Key(key("esc"))
	if m.mode != modeList || string(m.msg) != "x" {
		t.Errorf("Esc: mode %d, message %q; want the list and the message kept", m.mode, string(m.msg))
	}
}

// TestCommitRefusedLocally: no message, or nothing staged, sends nothing
// and says why.
func TestCommitRefusedLocally(t *testing.T) {
	m := model(changes(), 80, 24)
	m.Key(key("c"))
	if a := m.Key(key("enter")); a != nil || !strings.Contains(m.Note(), "message") {
		t.Error("an empty message was sent")
	}
	st := changes()
	st.Changes = st.Changes[3:]
	m = model(st, 80, 24)
	m.Key(key("c"))
	m.Key(key("x"))
	if a := m.Key(key("enter")); a != nil || !strings.Contains(m.Note(), "Nothing is staged") {
		t.Error("a commit with nothing staged was sent")
	}
}

// TestSelectionStays: the selection follows its change when rows move, and
// takes the row at its place when the change goes.
func TestSelectionStays(t *testing.T) {
	m := model(changes(), 80, 24)
	m.Key(key("end"))
	m.Key(tea.Key{Code: tea.KeyEnd})
	if m.selPath != "notes/todo.txt" {
		t.Fatalf("selected %q", m.selPath)
	}
	st := changes()
	st.Changes = append([]api.GitChange{{Path: "a.go", Work: "M"}}, st.Changes...)
	m.SetState(st)
	if m.selPath != "notes/todo.txt" {
		t.Errorf("after a new row: selected %q", m.selPath)
	}
	st.Changes = st.Changes[:len(st.Changes)-1]
	m.SetState(st)
	if m.selPath != "README.md" {
		t.Errorf("after the row went: selected %q, want README.md", m.selPath)
	}
}

// TestDiffView: a state with a diff shows it; keys scroll it and go back.
func TestDiffView(t *testing.T) {
	st := changes()
	st.Diff = &api.GitDiff{Path: "README.md"}
	for i := range 50 {
		st.Diff.Lines = append(st.Diff.Lines, api.GitDiffLine{Kind: api.GitLineAdd, Text: strings.Repeat("x", i)})
	}
	m := model(st, 80, 10)
	if m.mode != modeDiff {
		t.Fatal("the diff did not open")
	}
	m.Key(tea.Key{Code: tea.KeyPgDown})
	if m.scroll == 0 {
		t.Error("PgDn did not scroll")
	}
	m.Key(tea.Key{Code: tea.KeyEnd})
	if m.scroll != m.maxScroll() {
		t.Errorf("End scrolled to %d, want %d", m.scroll, m.maxScroll())
	}
	wantAction(t, m.Key(key("s")), api.GitActStage, map[string]string{api.ArgGitPath: "README.md"})
	wantAction(t, m.Key(key("esc")), api.GitActCloseDiff, nil)
	m.SetState(changes())
	if m.mode != modeList {
		t.Error("the list did not come back when the diff closed")
	}
}

// TestClick: a click selects a row, a second click shows its diff, a
// click on a hint presses its key, and a click on the box opens it.
func TestClick(t *testing.T) {
	m := model(changes(), 80, 24)
	l := m.layout()
	row := -1
	for y, h := range l.hits {
		if h == hitEntry && l.index[y] == 3 {
			row = y
		}
	}
	if row < 0 {
		t.Fatal("no row for entry 3")
	}
	if a := m.Click(5, row); a != nil || m.selIndex != 3 {
		t.Fatalf("first click: %+v, selected %d", a, m.selIndex)
	}
	wantAction(t, m.Click(5, row), api.GitActDiff, nil)

	hints := len(l.rows) - 1
	if l.hits[hints] != hitHints {
		t.Fatalf("last row is %d, want the hints", l.hits[hints])
	}
	wantAction(t, m.Click(1, hints), api.GitActStage, nil) // "s stage" comes first, after one cell
	commit := strings.Index(ansi.Strip(l.rows[hints]), "c commit")
	m.Click(commit, hints)
	if m.mode != modeCommit {
		t.Error("a click on the commit hint did not open the box")
	}
}

// TestWidthSafe: no size, however small, makes a row wider than the pane
// or more rows than its height.
func TestWidthSafe(t *testing.T) {
	st := changes()
	st.Error = "git status: something long went wrong in the repository"
	for _, s := range []api.GitState{st, {NotRepo: true, Folder: "/x"}, {}} {
		for w := 0; w <= 40; w++ {
			for h := 0; h <= 8; h++ {
				m := model(s, w, h)
				m.Key(key("c"))
				rows := m.View()
				if len(rows) > h {
					t.Fatalf("%dx%d: %d rows", w, h, len(rows))
				}
				for _, r := range rows {
					if ansi.StringWidth(r) > w {
						t.Fatalf("%dx%d: row %q is %d wide", w, h, r, ansi.StringWidth(r))
					}
				}
			}
		}
	}
}

// TestCutLeft: a long path keeps its end.
func TestCutLeft(t *testing.T) {
	if got := cutLeft("internal/server/gitpane.go", 15); got != "...gitpane.go" && ansi.StringWidth(got) != 15 {
		t.Errorf("cutLeft = %q", got)
	}
	if got := cutLeft("abc", 10); got != "abc" {
		t.Errorf("short path changed to %q", got)
	}
}

// TestCursor: the cursor sits after the typed text in the commit box, and
// nowhere outside it.
func TestCursor(t *testing.T) {
	m := model(changes(), 80, 24)
	if _, _, ok := m.Cursor(); ok {
		t.Fatal("a cursor with the box closed")
	}
	m.Key(key("c"))
	m.Key(key("o"))
	m.Key(key("k"))
	x, y, ok := m.Cursor()
	if !ok || y != 22 || x != 1+len(boxPrompt)+2 {
		t.Errorf("cursor at %d,%d (%v), want %d,22", x, y, ok, 1+len(boxPrompt)+2)
	}
}
