package gitpane

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/pkg/api"
)

func TestCommitBoxListsStaged(t *testing.T) {
	m := model(changes(), 80, 24)
	m.Key(key("c"))
	view := ansi.Strip(strings.Join(m.View(), "\n"))
	for _, want := range []string{"Commit 3 staged files", "added", "modified", "renamed", "internal/git/status.go", "internal/server/gitpane.go", "docs/old.md -> docs/new.md"} {
		if !strings.Contains(view, want) {
			t.Errorf("commit box lacks %q:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"Staged (", "Changes (", "README.md", "notes/todo.txt"} {
		if strings.Contains(view, unwanted) {
			t.Errorf("commit box includes %q:\n%s", unwanted, view)
		}
	}
}

func TestCommitBoxCapsStaged(t *testing.T) {
	st := api.GitState{Branch: "main"}
	for i := 1; i <= 7; i++ {
		st.Changes = append(st.Changes, api.GitChange{Path: fmt.Sprintf("staged-%d.go", i), Index: "A"})
	}
	m := model(st, 80, 24)
	m.Key(key("c"))
	view := ansi.Strip(strings.Join(m.View(), "\n"))
	for _, want := range []string{"Commit 7 staged files", "and 2 more", "staged-1.go", "staged-5.go"} {
		if !strings.Contains(view, want) {
			t.Errorf("commit view lacks %q:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"staged-6.go", "staged-7.go"} {
		if strings.Contains(view, unwanted) {
			t.Errorf("commit cap includes %q", unwanted)
		}
	}
	// Short panes keep the title, message box, and buttons before file rows.
	m.SetSize(30, 8)
	view = ansi.Strip(strings.Join(m.View(), "\n"))
	for _, want := range []string{"Commit 7 staged files", "and 6 more", "Message:", "Clear"} {
		if !strings.Contains(view, want) {
			t.Errorf("short commit view lacks %q:\n%s", want, view)
		}
	}
	if _, y, ok := m.Cursor(); !ok || y >= 7 {
		t.Fatalf("short commit cursor y=%d ok=%v", y, ok)
	}
}

func TestCommitBoxButtons(t *testing.T) {
	for _, tt := range []struct {
		label string
		key   tea.Key
	}{
		{"Commit", key("enter")}, {"Clear", tea.Key{Code: 'u', Mod: tea.ModCtrl}}, {"Close, keep draft", key("esc")},
	} {
		t.Run(tt.label, func(t *testing.T) {
			makeModel := func() *Model { m := model(changes(), 80, 24); m.Key(key("c")); m.Paste("Keep the draft"); return m }
			click, press := makeModel(), makeModel()
			row := ansi.Strip(click.View()[23])
			x := strings.Index(row, tt.label)
			if x < 0 {
				t.Fatalf("no %s button: %q", tt.label, row)
			}
			got, want := click.Click(x, 23), press.Key(tt.key)
			if !reflect.DeepEqual(got, want) || click.mode != press.mode || click.message() != press.message() || click.committing != press.committing || !reflect.DeepEqual(click.st, press.st) {
				t.Fatalf("click does not match %s", tt.key.String())
			}
			if tt.label == "Clear" && (click.message() != "" || !click.staged() || click.mode != modeCommit) {
				t.Fatal("Clear changed staged files or closed the box")
			}
			if tt.label == "Close, keep draft" && (click.message() != "Keep the draft" || click.mode != modeList) {
				t.Fatal("Close did not keep the draft")
			}
		})
	}
}

func TestCommitButtonsFocusedAndNarrow(t *testing.T) {
	m := model(changes(), 30, 20)
	m.Key(key("c"))
	row := ansi.Strip(m.View()[19])
	if !strings.Contains(row, "Commit") || !strings.Contains(row, "Clear") || strings.Contains(row, "Close") {
		t.Fatalf("narrow buttons: %q", row)
	}
	m.SetFocused(false)
	if row := ansi.Strip(m.View()[19]); strings.Contains(row, "Clear") || strings.Contains(row, "Close") {
		t.Fatalf("unfocused commit buttons: %q", row)
	}
	if !strings.Contains(ansi.Strip(strings.Join(m.View(), "\n")), "Commit 3 staged files") {
		t.Fatal("unfocused draft lost staged contents")
	}
}

func TestCommitMessageLimitUnchanged(t *testing.T) {
	m := model(changes(), 80, 24)
	m.Key(key("c"))
	m.Paste(strings.Repeat("x", api.MaxCommitMessage+1))
	if msg := m.Key(key("enter")); msg != nil || !strings.Contains(strings.Join(m.View(), "\n"), "too long") {
		t.Fatalf("oversized message: %#v", msg)
	}
}

func TestCommitNewlineKeys(t *testing.T) {
	for _, k := range []tea.Key{{Code: tea.KeyEnter, Mod: tea.ModAlt}, {Code: tea.KeyEnter, Mod: tea.ModShift}, {Code: 'j', Mod: tea.ModCtrl}} {
		t.Run(k.String(), func(t *testing.T) {
			m := model(changes(), 30, 8)
			m.Key(key("c"))
			m.Paste("first")
			if msg := m.Key(k); msg != nil {
				t.Fatalf("newline sent %#v", msg)
			}
			m.Paste("second")
			if m.message() != "first\nsecond" {
				t.Fatalf("message %q", m.message())
			}
			_, y, ok := m.Cursor()
			if !ok || y >= 7 || y < m.layout().boxTop {
				t.Fatalf("cursor y=%d ok=%v", y, ok)
			}
		})
	}
}

func TestCommitReadErrorStaysVisible(t *testing.T) {
	m := model(api.GitState{Folder: "/src/snow", Error: "git status: fatal: unsafe repository"}, 30, 8)
	m.Key(key("c"))
	view := ansi.Strip(strings.Join(m.View(), "\n"))
	if !strings.Contains(strings.Join(strings.Fields(view), " "), "unsafe repository") {
		t.Fatalf("commit mode hid the read error:\n%s", view)
	}
}

func TestCommitOmittedChanges(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {30, 20}, {30, 8}} {
		st := changes()
		st.Omitted = 2000
		m := model(st, size[0], size[1])
		m.Key(key("c"))
		view := strings.Join(strings.Fields(ansi.Strip(strings.Join(m.View(), "\n"))), " ")
		for _, want := range []string{"Commit 3 staged files shown", "2000 more changes not shown"} {
			if !strings.Contains(view, want) {
				t.Errorf("size %v lacks %q:\n%s", size, want, view)
			}
		}
	}
}
