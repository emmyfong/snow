package gitpane

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/pkg/api"
)

func TestNoteShowsInPaneRow(t *testing.T) {
	m := model(api.GitState{Branch: "main"}, 80, 24)
	m.Key(key("s"))
	if note := m.Note(); note != "" {
		t.Errorf("status-line note %q", note)
	}
	first := strings.Join(m.View(), "\n")
	if row := ansi.Strip(m.View()[22]); !strings.Contains(row, "No file to stage.") {
		t.Fatalf("note not above buttons: %q", row)
	}
	if second := strings.Join(m.View(), "\n"); second != first {
		t.Fatal("View consumed the note")
	}
}

func TestNoteClearsOnNextKey(t *testing.T) {
	for _, click := range []bool{false, true} {
		m := model(api.GitState{Branch: "main"}, 80, 24)
		m.Key(key("s"))
		if !strings.Contains(strings.Join(m.View(), "\n"), "No file to stage.") {
			t.Fatal("note was never shown")
		}
		if click {
			m.Click(0, 0)
		} else {
			m.Key(key("down"))
		}
		if strings.Contains(strings.Join(m.View(), "\n"), "No file to stage.") {
			t.Fatalf("next input (click %v) kept note", click)
		}
	}
}

func TestNotePrecedence(t *testing.T) {
	st := changes()
	st.Notice = "Older notice"
	m := model(st, 80, 24)
	m.Key(key("s"))
	view := ansi.Strip(strings.Join(m.View(), "\n"))
	if !strings.Contains(view, "already staged") || strings.Contains(view, "Older notice") {
		t.Fatalf("notice hid note:\n%s", view)
	}
	st.Error = "Server refused the operation"
	m.SetState(st)
	view = ansi.Strip(strings.Join(m.View(), "\n"))
	if !strings.Contains(view, st.Error) || strings.Contains(view, "already staged") {
		t.Fatalf("note hid server error:\n%s", view)
	}
	st.Error = ""
	m.SetState(st)
	m.Key(key("down"))
	if !strings.Contains(strings.Join(m.View(), "\n"), st.Notice) {
		t.Fatal("clearing note did not reveal notice")
	}
}

func TestNoteClickUsesDrawnRows(t *testing.T) {
	st := api.GitState{Branch: "main"}
	for i := 0; i < 12; i++ {
		st.Changes = append(st.Changes, api.GitChange{Path: fmt.Sprintf("file-%d.go", i), Index: "A"})
	}
	m := model(st, 80, 8)
	m.Key(tea.Key{Code: tea.KeyEnd})
	m.Key(key("s"))
	l := m.layout()
	for y, hit := range l.hits {
		if hit != hitEntry {
			continue
		}
		want := l.index[y]
		m.Click(5, y)
		if m.selIndex != want || m.note != "" {
			t.Fatalf("click selected %d, want drawn entry %d; note %q", m.selIndex, want, m.note)
		}
		return
	}
	t.Fatal("no clickable entry")
}

func TestNoteDoesNotRepeatReadError(t *testing.T) {
	m := model(api.GitState{Error: "git status: fatal: unsafe repository"}, 80, 24)
	m.Key(key("s"))
	view := ansi.Strip(strings.Join(m.View(), "\n"))
	if n := strings.Count(view, "unsafe repository"); n != 1 {
		t.Fatalf("read error shown %d times:\n%s", n, view)
	}
}
