package resumepick

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/convrow"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

var now = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

func items() []api.ConversationInfo {
	return []api.ConversationInfo{
		{ID: "c3", Title: "Fix the rename bug", Folder: "/home/u/snow", Model: "anthropic/claude-opus-5-5", LastActivity: now.Add(-5 * time.Minute), Turns: 7, Open: true},
		{ID: "c2", Title: "Explain the picker", Folder: "/home/u/snow", Model: "openai/gpt-6.1", LastActivity: now.Add(-30 * time.Hour), Turns: 1},
		{ID: "c1", Folder: "/home/u/docs", Model: "anthropic/claude-opus-5-5", LastActivity: now.Add(-72 * time.Hour)},
		{ID: "c0", Err: "line 1: damaged", Folder: "/home/u/old", LastActivity: now.Add(-100 * time.Hour)},
	}
}

func opened() *Model {
	m := New(4, func() time.Time { return now })
	m.Request()
	m.SetList(&api.ConversationList{Pane: 4, Items: items()})
	return m
}

func TestAsksForThisFolderFirst(t *testing.T) {
	m := New(4, func() time.Time { return now })
	if r := m.Request(); r == nil || r.Pane != 4 || r.All {
		t.Fatalf("first request %+v", r)
	}
	if m.Request() != nil {
		t.Fatal("a request came twice")
	}
}

func TestCtrlAAsksForEveryFolderAndBack(t *testing.T) {
	m := opened()
	ctrlA := tea.Key{Code: 'a', Mod: tea.ModCtrl}
	if r := m.Key(ctrlA); r != picker.Open {
		t.Fatalf("result %v", r)
	}
	if r := m.Request(); r == nil || !r.All || r.Pane != 4 {
		t.Fatalf("request %+v", r)
	}
	if _, ok := m.Choice(); ok {
		t.Fatal("old rows stayed while the new list loads")
	}
	m.SetList(&api.ConversationList{Pane: 4, All: false, Items: items()}) // a reply that crossed
	if _, ok := m.Choice(); ok {
		t.Fatal("a reply for the other scope was shown")
	}
	m.SetList(&api.ConversationList{Pane: 4, All: true, Items: items()})
	if it, ok := m.Choice(); !ok || it.ID != "c3" {
		t.Fatalf("choice %+v, %v", it, ok)
	}
	m.Key(ctrlA)
	if r := m.Request(); r == nil || r.All {
		t.Fatalf("back request %+v", r)
	}
}

func TestChoose(t *testing.T) {
	m := opened()
	m.Key(tea.Key{Code: tea.KeyDown})
	if r := m.Key(tea.Key{Code: tea.KeyEnter}); r != picker.Accepted {
		t.Fatalf("result %v", r)
	}
	if it, _ := m.Choice(); it.ID != "c2" {
		t.Fatalf("chose %q, want c2", it.ID)
	}
}

// TestRowsHoldNoEscapeSequences: text from a conversation file never
// reaches the screen with a control character in it.
func TestRowsHoldNoEscapeSequences(t *testing.T) {
	m := New(4, func() time.Time { return now })
	m.SetList(&api.ConversationList{Pane: 4, Items: []api.ConversationInfo{{
		ID: "c1", Title: "a\x1b]52;c;AAAA\x07b", Model: "m\x1b[2Jx", Folder: "/f\x1b[H",
		LastActivity: now, Turns: 1,
	}}})
	m.Key(tea.Key{Code: 'a', Mod: tea.ModCtrl})
	m.SetList(&api.ConversationList{Pane: 4, All: true, Items: []api.ConversationInfo{{
		ID: "c1", Title: "a\x1b]52;c;AAAA\x07b", Model: "m\x1b[2Jx", Folder: "/f\x1b[H",
		LastActivity: now, Turns: 1,
	}}})
	for _, c := range convrow.Columns(m.items[0], now, true) {
		if strings.ContainsAny(c, "\x1b\x07") {
			t.Fatalf("cell %q holds a control character", c)
		}
	}
}

// TestLongTitleStaysInTheBox: a title longer than the box is cut, and the
// box keeps its width.
func TestLongTitleStaysInTheBox(t *testing.T) {
	m := New(4, func() time.Time { return now })
	m.SetList(&api.ConversationList{Pane: 4, Items: []api.ConversationInfo{
		{ID: "c1", Title: strings.Repeat("long title ", 30), Model: "anthropic/claude-opus-5-5", LastActivity: now, Turns: 2},
	}})
	box, _, _ := m.View(theme.Default(), 100, 24)
	w := 0
	for _, l := range strings.Split(box, "\n") {
		if lw := ansi.StringWidth(l); lw != 0 && w != 0 && lw != w {
			t.Fatalf("lines differ in width: %d and %d", w, lw)
		} else if lw != 0 {
			w = lw
		}
	}
	if !strings.Contains(ansi.Strip(box), "long title") {
		t.Fatal("the title is missing")
	}
}

// TestTruncatedListSaysSo: conversations the server left out are counted
// in the title, and a new request clears the count.
func TestTruncatedListSaysSo(t *testing.T) {
	m := New(4, func() time.Time { return now })
	m.SetList(&api.ConversationList{Pane: 4, Items: items(), Truncated: 12})
	box, _, _ := m.View(theme.Default(), 100, 24)
	if !strings.Contains(ansi.Strip(box), "12 older not shown") {
		t.Fatalf("no count in:\n%s", ansi.Strip(box))
	}
	m.Key(tea.Key{Code: 'a', Mod: tea.ModCtrl})
	box, _, _ = m.View(theme.Default(), 100, 24)
	if strings.Contains(ansi.Strip(box), "not shown") {
		t.Fatal("the count stayed after a new request")
	}
}

func TestOtherPaneListIsIgnored(t *testing.T) {
	m := New(4, func() time.Time { return now })
	m.SetList(&api.ConversationList{Pane: 9, Items: items()})
	if _, ok := m.Choice(); ok {
		t.Fatal("took the list of another pane")
	}
}

// TestResumePickerGolden: the list for this folder, every folder, no
// conversations, and the wait for the server.
func TestResumePickerGolden(t *testing.T) {
	th := theme.Default()
	ctrlA := tea.Key{Code: 'a', Mod: tea.ModCtrl}
	tests := []struct {
		name string
		make func() *Model
	}{
		{"ThisFolder", opened},
		{"EveryFolder", func() *Model {
			m := opened()
			m.Key(ctrlA)
			m.SetList(&api.ConversationList{Pane: 4, All: true, Items: items()})
			return m
		}},
		{"Empty", func() *Model {
			m := New(4, func() time.Time { return now })
			m.SetList(&api.ConversationList{Pane: 4, Items: []api.ConversationInfo{}})
			return m
		}},
		{"Loading", func() *Model { return New(4, func() time.Time { return now }) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			box, _, _ := tt.make().View(th, 100, 24)
			golden.RequireEqual(t, box)
		})
	}
}
