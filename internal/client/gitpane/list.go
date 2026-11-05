package gitpane

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

func (m *Model) listLayout(l *layout) {
	th := m.th
	l.add(m.header(), hitNone, 0)
	status, box, hints := m.chrome()
	h := m.listHeight()
	body := m.listBody(h)
	for _, r := range body {
		l.add(r.text, r.hit, r.index)
	}
	for len(l.rows) < 1+h {
		l.add("", hitNone, 0)
	}
	if status {
		l.add(m.statusRow(), hitNone, 0)
	}
	if box {
		// The box grows upward as the message gets longer, so it sits on
		// the hints.
		rows := strings.Split(ui.TextArea(th, m.boxOptions()), "\n")
		for range boxRows - len(rows) {
			l.add("", hitBox, 0)
		}
		l.boxTop = len(l.rows)
		for _, r := range rows {
			l.add(" "+r, hitBox, 0)
		}
	}
	if hints {
		l.hints = m.listHints()
		hs := make([]ui.Hint, len(l.hints))
		for i, h := range l.hints {
			hs[i] = h.Hint
		}
		l.add(" "+ui.KeyHints(th, hs, m.width-1), hitHints, 0)
	}
}

// row is one row of the pane's body.
type row struct {
	text  string
	hit   hitKind
	index int
}

// listBody is the rows under the header: the changes, or what stands in
// for them.
func (m *Model) listBody(h int) []row {
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	note := func(lines ...string) []row {
		var out []row
		for _, s := range lines {
			for _, w := range strings.Split(ansi.Wrap(s, max(m.width-1, 1), ""), "\n") {
				out = append(out, row{text: " " + muted.Render(w)})
			}
		}
		return out
	}
	switch {
	case !m.loaded:
		return note("Reading git status...")
	case m.st.NotRepo:
		return note("No git repository in "+ui.FolderName(m.st.Folder)+".", "Run git init in a terminal to start one.")
	case m.readFailed():
		return note("Git did not answer:", ui.PlainLine(m.st.Error))
	}
	es := m.entries()
	if len(es) == 0 {
		return note("No changes.")
	}
	var items []ui.ListItem
	var itemEntry []int // the entry of each item, or -1 for a heading
	heading := func(text string) {
		items, itemEntry = append(items, ui.ListItem{Heading: text}), append(itemEntry, -1)
	}
	sel := -1
	count := map[int]int{}
	for _, e := range es {
		count[e.section()]++
	}
	codeW := 1
	pathW := max(m.width-2-codeW-3, 4) // pointer, code, column gap
	if len(m.st.Changes) == 0 {
		heading("No changes.")
	}
	omitted := func() {
		if m.st.Omitted > 0 {
			heading(fmt.Sprintf("%d more changes not shown", m.st.Omitted))
		}
	}
	for i, e := range es {
		if sec := e.section(); i == 0 || es[i-1].section() != sec {
			if sec >= secWorktrees && (i == 0 || es[i-1].section() < secWorktrees) {
				omitted()
			}
			heading(fmt.Sprintf("%s (%d)", sectionNames[sec], count[sec]))
		}
		if i == m.selected(es) {
			sel = len(items)
		}
		switch {
		case e.cm != nil:
			items = append(items, ui.ListItem{Cols: commitCols(*e.cm)})
		case e.wt != nil:
			items = append(items, ui.ListItem{Cols: m.worktreeCols(*e.wt)})
		default:
			items = append(items, ui.ListItem{Cols: []string{code(e), cutLeft(pathText(e.change), pathW)}})
		}
		itemEntry = append(itemEntry, i)
	}
	if count[secWorktrees]+count[secHistory] == 0 {
		omitted()
	}
	view, rows := ui.ListRows(m.th, ui.ListOptions{Items: items, Selected: sel, Width: m.width, Height: h})
	var out []row
	for i, r := range strings.Split(view, "\n") {
		if i >= len(rows) {
			break
		}
		if e := itemEntry[rows[i]]; e >= 0 {
			out = append(out, row{text: r, hit: hitEntry, index: e})
		} else {
			out = append(out, row{text: r})
		}
	}
	return out
}

var sectionNames = map[int]string{secStaged: "Staged", secChanges: "Changes", secWorktrees: "Worktrees", secHistory: "History"}

// commitCols are a history row's cells: its short hash, the agent Snow
// saw make it or "unknown", and its subject. The mark comes before the
// subject so a narrow pane cuts the subject, not the mark.
func commitCols(cm api.GitCommit) []string {
	agent := "unknown"
	if cm.Agent != "" {
		agent = "by " + ui.PlainLine(cm.Agent)
	}
	return []string{"", strings.Join([]string{ui.PlainLine(cm.Short), agent, ui.PlainLine(cm.Subject)}, "  ")}
}

// worktreeCols are a worktree row's cells: "*" for the checkout the pane
// shows, then its folder name, branch, lock, and owner in one cell, so a
// long path in the changes does not push them out of a narrow pane.
func (m *Model) worktreeCols(wt api.GitWorktree) []string {
	mark := ""
	if m.current(wt) {
		mark = "*"
	}
	parts := []string{ui.FolderName(ui.PlainLine(wt.Path))}
	if wt.Branch != "" {
		parts = append(parts, ui.PlainLine(wt.Branch))
	} else {
		parts = append(parts, "detached")
	}
	if wt.Locked {
		parts = append(parts, "locked")
	}
	if own := ownerText(wt, owners(wt, m.st.Worktrees, m.agents)); own != "" {
		parts = append(parts, own)
	}
	return []string{mark, strings.Join(parts, "  ")}
}

// current reports whether wt is the checkout the pane's folder is in.
func (m *Model) current(wt api.GitWorktree) bool {
	return len(owners(wt, m.st.Worktrees, []api.AgentInfo{{Folder: m.st.Folder}})) > 0
}

// code is the one-letter code of an entry's change.
func code(e entry) string {
	if e.staged {
		return e.change.Index
	}
	return e.change.Work
}

// pathText is a change's path, and a rename's old path before it.
func pathText(c api.GitChange) string {
	if c.Orig != "" {
		return ui.PlainLine(c.Orig) + " -> " + ui.PlainLine(c.Path)
	}
	return ui.PlainLine(c.Path)
}

// cutLeft cuts s to width cells by dropping its start, so a long path
// keeps its file name.
func cutLeft(s string, width int) string {
	w := ansi.StringWidth(s)
	if w <= width {
		return s
	}
	if width <= 3 {
		return ansi.TruncateLeft(s, w-width, "")
	}
	return "..." + ansi.TruncateLeft(s, w-(width-3), "")
}
