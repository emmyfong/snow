package chat

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

func conv(id, title string, turns int, ago time.Duration) api.ConversationInfo {
	return api.ConversationInfo{ID: id, Title: title, Model: "anthropic/claude-sonnet-5-5", Folder: "/home/emmy/snow", LastActivity: start.Add(-ago), Turns: turns}
}

// sampleList is a list of three conversations in the chat's folder.
func sampleList() *api.ConversationList {
	return &api.ConversationList{Pane: 2, Items: []api.ConversationInfo{
		conv("c3", "fix the rename command", 4, 5*time.Minute),
		conv("c2", "why is the build slow", 2, 3*time.Hour),
		conv("c1", "explain the layout package", 1, 26*time.Hour),
	}}
}

// TestEmptyChatAsksForTheList: an empty chat asks once for the recent
// conversations of its folder, again when another conversation takes the
// pane, and never once it holds messages.
func TestEmptyChatAsksForTheList(t *testing.T) {
	m, _ := newChat(80, 24)
	if r := m.ListRequest(); r == nil || r.Pane != 2 || r.All {
		t.Fatalf("first request %+v, want this folder of pane 2", r)
	}
	if r := m.ListRequest(); r != nil {
		t.Fatalf("second request %+v, want none", r)
	}
	m.Snapshot(&api.ChatSnapshot{Pane: 2, Agent: "c2", Conversation: "c2", Model: "anthropic/claude-sonnet-5-5", State: api.ChatStateIdle})
	if r := m.ListRequest(); r == nil {
		t.Fatal("no request after /new put another conversation in the pane")
	}
	m.Snapshot(&api.ChatSnapshot{Pane: 2, Agent: "c3", Conversation: "c3", Model: "m", State: api.ChatStateIdle,
		Entries: []api.ChatEntry{{Kind: api.ChatEntryUser, Text: "hi"}}})
	if r := m.ListRequest(); r != nil {
		t.Fatalf("request %+v for a chat that has messages", r)
	}
}

// TestEmptyChatShowsTheList: the rows show newest first with the title, the
// model, the age, and the turns; the first is selected.
func TestEmptyChatShowsTheList(t *testing.T) {
	m, _ := newChat(100, 24)
	m.Conversations(sampleList())
	s := screen(m)
	for _, want := range []string{"Recent conversations in this folder", "fix the rename command", "claude-sonnet-5-5", "5m ago", "4 turns", "1 turn", "Enter"} {
		if !strings.Contains(s, want) {
			t.Fatalf("no %q in\n%s", want, s)
		}
	}
	if strings.Contains(s, "anthropic/claude-sonnet-5-5   ") {
		t.Fatalf("the rows show the provider:\n%s", s)
	}
	if strings.Index(s, "fix the rename") > strings.Index(s, "explain the layout") {
		t.Fatalf("rows are not newest first:\n%s", s)
	}
}

// TestEmptyChatResume: up and down select, and Enter with nothing typed
// resumes the selected conversation. It sends nothing with no list.
func TestEmptyChatResume(t *testing.T) {
	m, _ := newChat(100, 24)
	if in := m.Key(key(tea.KeyEnter, 0, "")); in != nil {
		t.Fatalf("Enter with no list sent %+v", in)
	}
	m.Conversations(sampleList())
	m.Key(key(tea.KeyDown, 0, ""))
	m.Key(key(tea.KeyDown, 0, ""))
	m.Key(key(tea.KeyDown, 0, "")) // past the last row
	m.Key(key(tea.KeyUp, 0, ""))
	in := m.Key(key(tea.KeyEnter, 0, ""))
	if in == nil || in.Kind != api.ChatInputResume || in.Conversation != "c2" || in.Pane != 2 || in.Agent != "c1" {
		t.Fatalf("Enter sent %+v, want a resume of c2", in)
	}
}

// TestEmptyChatTypeStartsNew: typing a message and Enter sends it as the
// first message, with the list in the way of nothing. The list is gone once
// the turn starts.
func TestEmptyChatTypeStartsNew(t *testing.T) {
	m, _ := newChat(100, 24)
	m.Conversations(sampleList())
	typeText(m, "hello")
	m.Key(key(tea.KeyUp, 0, "")) // with text typed, the arrows do not choose a row
	in := m.Key(key(tea.KeyEnter, 0, ""))
	if in == nil || in.Kind != api.ChatInputMessage || in.Text != "hello" {
		t.Fatalf("Enter sent %+v, want the message", in)
	}
	m.Event(ev(api.ChatEvent{Kind: api.ChatEventTurnStarted, Turn: 1, Text: "hello"}))
	if s := screen(m); strings.Contains(s, "Recent conversations") || strings.Contains(s, "fix the rename") {
		t.Fatalf("the list stayed after the first message:\n%s", s)
	}
	if r := m.ListRequest(); r != nil {
		t.Fatalf("request %+v after the first message", r)
	}
}

// TestEmptyChatAllFolders: Ctrl+a asks for every folder, shows the folder
// of each row, and asks for this folder again the next time. A reply for the
// scope the user left is ignored.
func TestEmptyChatAllFolders(t *testing.T) {
	m, _ := newChat(100, 24)
	m.ListRequest()
	m.Conversations(sampleList())
	m.Key(key('a', tea.ModCtrl, ""))
	r := m.ListRequest()
	if r == nil || !r.All || r.Pane != 2 {
		t.Fatalf("Ctrl+a asked %+v, want every folder", r)
	}
	m.Conversations(sampleList()) // the reply to the old question
	if s := screen(m); strings.Contains(s, "fix the rename") {
		t.Fatalf("a reply for this folder showed in the every-folder list:\n%s", s)
	}
	m.Conversations(&api.ConversationList{Pane: 2, All: true, Items: []api.ConversationInfo{conv("x", "in another folder", 3, time.Hour)}})
	s := screen(m)
	for _, want := range []string{"every folder", "in another folder", "snow"} {
		if !strings.Contains(s, want) {
			t.Fatalf("no %q in\n%s", want, s)
		}
	}
	m.Key(key('a', tea.ModCtrl, ""))
	if r := m.ListRequest(); r == nil || r.All {
		t.Fatalf("second Ctrl+a asked %+v, want this folder", r)
	}
}

// TestEmptyChatUnreadable: a file that cannot be read is a row that says so,
// and Enter on it sends nothing and shows the error.
func TestEmptyChatUnreadable(t *testing.T) {
	m, _ := newChat(100, 24)
	m.Conversations(&api.ConversationList{Pane: 2, Items: []api.ConversationInfo{
		{ID: "bad", Err: "line 3: unexpected end of JSON input", LastActivity: start},
		conv("c1", "fine one", 1, time.Hour),
	}})
	if s := screen(m); !strings.Contains(s, "cannot read this file") || !strings.Contains(s, "fine one") {
		t.Fatalf("rows:\n%s", s)
	}
	if in := m.Key(key(tea.KeyEnter, 0, "")); in != nil {
		t.Fatalf("Enter on an unreadable row sent %+v", in)
	}
	if n := m.Note(); !strings.Contains(n, "unexpected end of JSON input") {
		t.Fatalf("note %q, want the error", n)
	}
}

// TestEmptyChatClick: a click on a row resumes it.
func TestEmptyChatClick(t *testing.T) {
	m, _ := newChat(100, 24)
	m.Conversations(sampleList())
	rows := strings.Split(screen(m), "\n")
	y := -1
	for i, r := range rows {
		if strings.Contains(r, "why is the build slow") {
			y = i
		}
	}
	if y < 0 {
		t.Fatalf("no row to click:\n%s", strings.Join(rows, "\n"))
	}
	in := m.Click(10, y)
	if in == nil || in.Kind != api.ChatInputResume || in.Conversation != "c2" {
		t.Fatalf("click sent %+v, want a resume of c2", in)
	}
}

// TestEmptyChatNoConversations: with none, the chat says so. Before the
// server answers, it says nothing.
func TestEmptyChatNoConversations(t *testing.T) {
	m, _ := newChat(100, 24)
	if s := screen(m); strings.Contains(s, "conversations") {
		t.Fatalf("before the reply:\n%s", s)
	}
	m.Conversations(&api.ConversationList{Pane: 2})
	s := screen(m)
	if !strings.Contains(s, "No conversations in this folder yet.") || strings.Contains(s, "Enter resume") {
		t.Fatalf("with none:\n%s", s)
	}
}

// TestEmptyChatMoreRows: rows that do not fit give way to "... N more", and
// the selection scrolls into view.
func TestEmptyChatMoreRows(t *testing.T) {
	m, _ := newChat(100, 14)
	l := &api.ConversationList{Pane: 2}
	for i := range 20 {
		l.Items = append(l.Items, conv(fmt.Sprintf("c%02d", i), fmt.Sprintf("topic number %02d", i), 1, time.Duration(i+1)*time.Hour))
	}
	m.Conversations(l)
	s := screen(m)
	if !strings.Contains(s, "more") || !strings.Contains(s, "topic number 00") || strings.Contains(s, "topic number 19") {
		t.Fatalf("rows:\n%s", s)
	}
	for range 19 {
		m.Key(key(tea.KeyDown, 0, ""))
	}
	if s := screen(m); !strings.Contains(s, "topic number 19") {
		t.Fatalf("the selection is out of view:\n%s", s)
	}
}

// TestEmptyChatOlderNotShown: the server says how many it left out for room,
// and the list passes it on.
func TestEmptyChatOlderNotShown(t *testing.T) {
	m, _ := newChat(100, 24)
	l := sampleList()
	l.Truncated = 7
	m.Conversations(l)
	if s := screen(m); !strings.Contains(s, "7 more") {
		t.Fatalf("no count of the older ones:\n%s", s)
	}
}

var emptyChatStates = map[string]func(*Model){
	"rows": func(m *Model) { m.Conversations(sampleList()) },
	"more": func(m *Model) {
		l := &api.ConversationList{Pane: 2}
		for i := range 30 {
			l.Items = append(l.Items, conv(fmt.Sprintf("c%02d", i), fmt.Sprintf("topic number %02d", i), i%5+1, time.Duration(i+1)*40*time.Minute))
		}
		l.Items[2].Open = true
		m.Conversations(l)
		m.Key(key(tea.KeyDown, 0, ""))
	},
	"none": func(m *Model) { m.Conversations(&api.ConversationList{Pane: 2}) },
	"noprovider": func(m *Model) {
		m.Snapshot(&api.ChatSnapshot{Pane: 2, Agent: "c1", Conversation: "c1", State: api.ChatStateIdle})
		m.Conversations(sampleList())
	},
	"unreadable": func(m *Model) {
		m.Conversations(&api.ConversationList{Pane: 2, Items: []api.ConversationInfo{
			{ID: "bad", Err: "line 3: unexpected end", LastActivity: start}, conv("c1", "fine one", 1, time.Hour)}})
	},
}

// TestEmptyChatGolden draws every state at both sizes in two themes: the
// default and one with a very different palette.
func TestEmptyChatGolden(t *testing.T) {
	themes := map[string]theme.Theme{"frost": theme.Default()}
	if th, ok := theme.ByName("Gruvbox Dark"); ok {
		themes["gruvbox"] = th
	} else {
		t.Fatal("no Gruvbox Dark theme")
	}
	for name, state := range emptyChatStates {
		for tn, th := range themes {
			for _, size := range [][2]int{{80, 24}, {120, 40}} {
				t.Run(fmt.Sprintf("%s-%s-%dx%d", name, tn, size[0], size[1]), func(t *testing.T) {
					m, _ := newChat(size[0], size[1])
					m.SetTheme(th)
					state(m)
					golden.RequireEqual(t, strings.Join(m.View(), "\n"))
				})
			}
		}
	}
}

// TestEmptyChatMoreRowKeepsItsRoom: when the server left conversations out
// but every row it sent fits, the "more" row still has a place of its own.
// It does not push the first line of the transcript out of the pane.
func TestEmptyChatMoreRowKeepsItsRoom(t *testing.T) {
	for h := 8; h <= 16; h++ {
		m, _ := newChat(100, h)
		l := sampleList()
		l.Truncated = 7
		m.Conversations(l)
		s := screen(m)
		if strings.Contains(s, "7 more") && !strings.Contains(s, "No messages yet") {
			t.Fatalf("height %d: the \"more\" row pushed the transcript up:\n%s", h, s)
		}
	}
}

// TestEmptyChatTinyPaneActsOnNothingHidden: a pane too small to show the
// list draws none, and then Enter sends the typed message, not a resume of a
// row nobody can see.
func TestEmptyChatTinyPaneActsOnNothingHidden(t *testing.T) {
	m, _ := newChat(100, 5)
	m.Conversations(sampleList())
	if s := screen(m); strings.Contains(s, "Recent conversations") {
		t.Fatalf("the list drew in a pane with no room for it:\n%s", s)
	}
	if in := m.Key(key(tea.KeyEnter, 0, "")); in != nil {
		t.Fatalf("Enter sent %+v with no list on screen", in)
	}
	typeText(m, "hi")
	if in := m.Key(key(tea.KeyEnter, 0, "")); in == nil || in.Kind != api.ChatInputMessage || in.Text != "hi" {
		t.Fatalf("Enter sent %+v, want the message", in)
	}
}

// TestEmptyChatHeadingOnlyActsOnNothingHidden: with room for the heading but
// not for one row, Enter still does not resume a row that is not drawn.
func TestEmptyChatHeadingOnlyActsOnNothingHidden(t *testing.T) {
	for h := 5; h <= 12; h++ {
		m, _ := newChat(100, h)
		m.Conversations(sampleList())
		s := screen(m)
		in := m.Key(key(tea.KeyEnter, 0, ""))
		if in != nil && !strings.Contains(s, "fix the rename command") {
			t.Fatalf("height %d: Enter sent %+v, but no row is on screen:\n%s", h, in, s)
		}
	}
}
