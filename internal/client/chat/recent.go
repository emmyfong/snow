package chat

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/emmyfong/snow/internal/client/convrow"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// recentList is the past conversations an empty chat offers: those of its
// folder, or of every folder.
type recentList struct {
	agent  string // the conversation the list belongs to; "" while there is none
	all    bool
	items  []api.ConversationInfo
	left   int  // older conversations the server left out to fit a frame
	loaded bool // the server answered for the current scope
	sel    int
	want   *api.ConversationListRequest // what the session sends next, once
}

// empty reports whether the conversation has no message yet, so the chat
// shows its recent conversations. A running turn or a queued message is
// not empty.
func (m *Model) empty() bool {
	return m.loaded && len(m.snap.Entries) == 0 && len(m.snap.Queue) == 0 && !m.snap.Running
}

// syncRecent keeps the list in step with the conversation in the pane. A
// new empty conversation asks for the list, once; a conversation with
// messages drops it.
func (m *Model) syncRecent() {
	switch {
	case !m.empty():
		m.recent = recentList{}
	case m.recent.agent != m.snap.Agent:
		m.recent = recentList{agent: m.snap.Agent, want: &api.ConversationListRequest{Pane: m.snap.Pane}}
	}
}

// ListRequest returns the request for the list of past conversations that
// the session should send, once, or nil.
func (m *Model) ListRequest() *api.ConversationListRequest {
	r := m.recent.want
	m.recent.want = nil
	return r
}

// Conversations takes the server's list. A list for the scope the user has
// left, or for another pane, is dropped.
func (m *Model) Conversations(l *api.ConversationList) {
	r := &m.recent
	if !m.empty() || l.Pane != m.snap.Pane || l.All != r.all {
		return
	}
	r.items, r.left, r.loaded = l.Items, l.Truncated, true
	r.sel = min(r.sel, max(len(r.items)-1, 0))
	m.invalidate()
}

// recentKey handles the keys of the list. Ctrl+a switches between this
// folder and every folder. Up, Down, and Enter work on the rows while
// nothing is typed, so a message that starts with a key the list uses is
// still typed.
func (m *Model) recentKey(k tea.Key) (*api.ChatInput, bool) {
	r := &m.recent
	if !m.empty() || m.cmdOpen {
		return nil, false
	}
	switch {
	case k.Code == 'a' && k.Mod == tea.ModCtrl:
		*r = recentList{agent: r.agent, all: !r.all, want: &api.ConversationListRequest{Pane: m.snap.Pane, All: !r.all}}
		m.invalidate()
		return nil, true
	case len(m.input) > 0 || len(r.items) == 0 || k.Mod != 0:
		return nil, false
	case k.Code == tea.KeyUp:
		r.sel = max(r.sel-1, 0)
	case k.Code == tea.KeyDown:
		r.sel = min(r.sel+1, len(r.items)-1)
	case k.Code == tea.KeyEnter:
		return m.resumeRow(r.sel), true
	default:
		return nil, false
	}
	m.invalidate()
	return nil, true
}

// resumeRow returns the input that resumes row i. The server moves focus
// to the pane that shows it when another pane does. A row for a file that
// cannot be read resumes nothing and says why.
func (m *Model) resumeRow(i int) *api.ChatInput {
	it := m.recent.items[i]
	if it.Err != "" {
		m.note = fmt.Sprintf("Cannot read %s: %s", ui.PlainLine(it.ID), ui.PlainLine(it.Err))
		return nil
	}
	in := m.chatInput(api.ChatInputResume, "")
	in.Conversation = it.ID
	return in
}

// drawRecent adds the list to the transcript being drawn: a heading, then
// the rows (or what there is instead of rows), then the keys. Rows past the
// room give way to "... N more". It adds nothing when there is no room for
// a heading and one row.
func (m *Model) drawRecent(indent int) {
	r := &m.recent
	width := m.width - indent
	room := m.transcriptHeight() - len(m.drawn.rows) - 1 // 1: the blank row above
	if !r.loaded || width < 10 || room < 3 {
		return // nothing until the server answers, so no text flashes by
	}
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	lead := strings.Repeat(" ", indent)
	scope := "this folder"
	if r.all {
		scope = "every folder"
	}
	emit := func(row string, h hit) {
		if row != "" {
			row = lead + row
		}
		m.drawn.rows = append(m.drawn.rows, row)
		m.drawn.hits = append(m.drawn.hits, h)
	}
	emit("", hit{})
	emit(muted.Render("Recent conversations in "+scope), hit{})
	switch {
	case len(r.items) == 0:
		if r.all {
			emit(muted.Render("No conversations yet."), hit{})
		} else {
			emit(muted.Render("No conversations in this folder yet."), hit{})
		}
		return
	}
	items := make([]ui.ListItem, len(r.items))
	for i, it := range r.items {
		it.Model = shortModel(it.Model)
		items[i] = ui.ListItem{Cols: convrow.Columns(it, m.now(), r.all)}
	}
	fit := room - 2 // the heading and the keys
	if len(items) > fit {
		fit-- // the "more" row
	}
	if fit < 1 {
		return
	}
	view, idx := ui.ListRows(m.th, ui.ListOptions{Items: items, Selected: r.sel, Width: width, Height: fit})
	for i, row := range strings.Split(view, "\n") {
		emit(row, hit{recent: idx[i] + 1})
	}
	if more := len(items) - len(idx) + r.left; more > 0 {
		emit(muted.Render(fmt.Sprintf("... %d more", more)), hit{})
	}
	emit(ui.KeyHints(m.th, []ui.Hint{{Key: "Enter", Desc: "resume"}, {Key: "Ctrl+a", Desc: scopeWord(r.all)}}, width), hit{})
}

// shortModel drops the provider from a model name: the row has little room,
// and the heading of the list already says this is Snow's own chat.
func shortModel(name string) string {
	if _, model, ok := api.ParseModelName(name); ok {
		return model
	}
	return name
}

// scopeWord names the list that Ctrl+a switches to.
func scopeWord(all bool) string {
	if all {
		return "this folder"
	}
	return "every folder"
}
