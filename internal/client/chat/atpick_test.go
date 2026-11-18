package chat

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// pickerFiles are the folder's files in walk order, so a test can tell the
// walk order from the filter order.
var pickerFiles = []string{
	"README.md",
	"docs/design.md",
	"internal/client/chat/chat.go",
	"internal/server/chat.go",
	"internal/server/files.go",
}

// pickerChatWith is a chat whose folder's files the server has listed.
func pickerChatWith(t *testing.T, w, h int, files []string, cut bool) *Model {
	t.Helper()
	m, _ := newChat(w, h)
	m.Snapshot(&api.ChatSnapshot{Pane: 2, Agent: "c1", Conversation: "c1", Model: "anthropic/claude-sonnet-5-5",
		Mode: "ask", State: api.ChatStateIdle, Folder: "/work/project"})
	if r := m.FileRequest(); r == nil || r.Pane != 2 {
		t.Fatalf("file request %+v", r)
	}
	m.Files(&api.FileList{Pane: 2, Folder: "/work/project", Files: files, Cut: cut})
	return m
}

// pickerChat is pickerChatWith for the whole list, uncut.
func pickerChat(t *testing.T, w, h int) *Model {
	t.Helper()
	return pickerChatWith(t, w, h, pickerFiles, false)
}

// TestAtPickerFilters: typing after @ filters the files, Enter inserts the
// @path, and a typed :line suffix stays.
func TestAtPickerFilters(t *testing.T) {
	t.Run("filter and insert", func(t *testing.T) {
		m := pickerChat(t, 100, 30)
		typeText(m, "@server")
		if !m.atOpen {
			t.Fatal("the @ picker did not open")
		}
		got := m.atMatches()
		want := []string{"internal/server/chat.go", "internal/server/files.go"}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("matches %q, want %q", got, want)
		}
		if in := m.Key(key(tea.KeyEnter, 0, "")); in != nil {
			t.Fatalf("Enter in the picker sent %+v", in)
		}
		if got := string(m.input); got != "@internal/server/chat.go" {
			t.Fatalf("input %q, want @internal/server/chat.go", got)
		}
	})
	t.Run("line suffix", func(t *testing.T) {
		m := pickerChat(t, 100, 30)
		typeText(m, "@files.go:12")
		m.Key(key(tea.KeyEnter, 0, ""))
		if got := string(m.input); got != "@internal/server/files.go:12" {
			t.Fatalf("input %q, want @internal/server/files.go:12", got)
		}
	})
	t.Run("ctrl-o opens", func(t *testing.T) {
		m := pickerChat(t, 100, 30)
		typeText(m, "@files.go:12")
		if in := m.Key(key('o', tea.ModCtrl, "")); in != nil {
			t.Fatalf("Ctrl+o sent %+v", in)
		}
		p, line := m.AskedFile()
		if p != "internal/server/files.go" || line != 12 {
			t.Fatalf("asked for %q:%d, want internal/server/files.go:12", p, line)
		}
	})
	t.Run("case folds", func(t *testing.T) {
		m := pickerChat(t, 100, 30)
		typeText(m, "@SERVER")
		if got := m.atMatches(); len(got) != 2 {
			t.Fatalf("matches %q, want two server files", got)
		}
	})
	t.Run("cut row", func(t *testing.T) {
		m := pickerChatWith(t, 100, 30, pickerFiles, true)
		typeText(m, "@")
		v, ok := m.atView()
		if !ok {
			t.Fatal("the picker did not open")
		}
		rows := ansi.Strip(strings.Join(v.lines, "\n"))
		if !strings.Contains(rows, "Only the first 5 files are listed.") {
			t.Fatalf("the picker rows lack the cut note:\n%s", rows)
		}
	})
}

// TestAtPickerGolden pins the picker rows and its keys.
func TestAtPickerGolden(t *testing.T) {
	for _, th := range []struct {
		name  string
		theme theme.Theme
	}{{"Dark", theme.Default()}, {"Light", lightTheme()}} {
		for _, size := range [][2]int{{100, 30}, {60, 20}} {
			t.Run(fmt.Sprintf("%s%dx%d", th.name, size[0], size[1]), func(t *testing.T) {
				m := pickerChat(t, size[0], size[1])
				m.SetTheme(th.theme)
				typeText(m, "@")
				golden.RequireEqual(t, strings.Join(m.View(), "\n"))
			})
		}
	}
}
