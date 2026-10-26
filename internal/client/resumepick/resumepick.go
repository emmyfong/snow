// Package resumepick is the picker /resume opens: the past conversations of
// the chat's folder, newest first, or of every folder. It asks the server
// for the list and shows the reply; the session sends what it asks.
package resumepick

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/listpick"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/sessionlist"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// Model is the open resume picker.
type Model struct {
	pane    int
	now     func() time.Time
	list    *listpick.Model
	items   []api.ConversationInfo
	all     bool
	pending *api.ConversationListRequest
}

// New opens the picker for chat pane. It starts with the pane's folder, and
// Request holds the first request for the list.
func New(pane int, now func() time.Time) *Model {
	m := &Model{pane: pane, now: now}
	m.list = listpick.New("Resume", nil)
	m.ask(false)
	return m
}

// ask clears the rows and queues a request for this folder or all of them.
func (m *Model) ask(all bool) {
	m.all, m.items = all, nil
	m.list.SetRows(nil)
	m.list.SetNote("Loading conversations...")
	m.list.SetTitle(title(all, 0))
	m.pending = &api.ConversationListRequest{Pane: m.pane, All: all}
}

// title names the list and, when the server left conversations out, says
// how many.
func title(all bool, left int) string {
	t := "Resume · this folder"
	if all {
		t = "Resume · every folder"
	}
	if left > 0 {
		t += fmt.Sprintf(" · %d older not shown", left)
	}
	return t
}

// Request returns the list request the picker wants sent, once, or nil.
func (m *Model) Request() *api.ConversationListRequest {
	r := m.pending
	m.pending = nil
	return r
}

// SetList shows a server reply. A reply for the other scope, such as one
// that crossed a Ctrl+a, is ignored.
func (m *Model) SetList(l *api.ConversationList) {
	if l.Pane != m.pane || l.All != m.all {
		return
	}
	m.items = l.Items
	m.list.SetTitle(title(m.all, l.Truncated))
	sel := m.list.Selected()
	rows := make([][]string, len(l.Items))
	for i, it := range l.Items {
		rows[i] = m.row(it)
	}
	m.list.SetRows(rows)
	m.list.Select(max(sel, 0))
	if m.all {
		m.list.SetNote("No saved conversations.")
	} else {
		m.list.SetNote("No saved conversations in this folder. Ctrl+a shows every folder.")
	}
}

// row is the columns of one conversation: title, model, folder (in the
// every-folder list), age, turns, and whether a pane shows it.
func (m *Model) row(it api.ConversationInfo) []string {
	name := it.Title
	switch {
	case it.Err != "":
		name = "(cannot read this file)"
	case name == "":
		name = "(no messages yet)"
	}
	cols := []string{name, it.Model}
	if m.all {
		cols = append(cols, folderName(it.Folder))
	}
	turns := strconv.Itoa(it.Turns) + " turns"
	if it.Turns == 1 {
		turns = "1 turn"
	}
	open := ""
	if it.Open {
		open = "open"
	}
	cols = append(cols, sessionlist.Ago(max(m.now().Sub(it.LastActivity), 0)), turns, open)
	// The text comes from files the user may not have written.
	for i, c := range cols {
		cols[i] = ui.PlainLine(c)
	}
	return cols
}

// folderName is the last part of a folder path, as Linux, macOS, WSL, or
// Windows writes it, or "-" for no folder.
func folderName(folder string) string {
	s := strings.TrimRight(strings.ReplaceAll(folder, `\`, "/"), "/")
	switch {
	case s != "":
		return path.Base(s)
	case folder != "":
		return "/"
	default:
		return "-"
	}
}

// Key handles one key press. Ctrl+a switches between this folder and every
// folder.
func (m *Model) Key(k tea.Key) picker.Result {
	if k.Code == 'a' && k.Mod == tea.ModCtrl {
		m.ask(!m.all)
		return picker.Open
	}
	return m.list.Key(k)
}

// Click handles a left click at window cell x, y.
func (m *Model) Click(th theme.Theme, cols, rows, x, y int) picker.Result {
	return m.list.Click(th, cols, rows, x, y)
}

// View draws the picker centered on a window of cols by rows cells.
func (m *Model) View(th theme.Theme, cols, rows int) (box string, x, y int) {
	m.list.SetHints([]ui.Hint{
		{Key: "Enter", Desc: "resume"},
		{Key: "Ctrl+a", Desc: scopeHint(m.all)},
		{Key: "Esc", Desc: "close"},
	})
	return m.list.View(th, cols, rows)
}

func scopeHint(all bool) string {
	if all {
		return "this folder"
	}
	return "every folder"
}

// Choice returns the selected conversation, and false with none.
func (m *Model) Choice() (api.ConversationInfo, bool) {
	if i := m.list.Selected(); i >= 0 && i < len(m.items) {
		return m.items[i], true
	}
	return api.ConversationInfo{}, false
}
