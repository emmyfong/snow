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
		rows := m.statusRows(m.statusHeight(box, hints))
		for _, row := range rows {
			l.add(row, hitNone, 0)
		}
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
	if hints && m.mode == modeList && m.removePath == "" {
		opts, _ := m.listButtons()
		l.add(ui.ButtonRow(th, opts), hitButtons, 0)
	} else if hints {
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
				if len(out) >= h {
					return out
				}
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
	var groups []int
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
			groups = append(groups, len(items))
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
			items = append(items, ui.ListItem{Cols: []string{m.changeCell(e, max(m.width-2, 0))}})
		}
		itemEntry = append(itemEntry, i)
	}
	if count[secWorktrees]+count[secHistory] == 0 {
		omitted()
	}
	// Each section owns its columns. A long change path must not push a
	// worktree's branch and owner off the pane.
	groups = append(groups, len(items))
	first, last := ui.ListWindow(len(items), sel, h)
	var out []row
	start := 0
	for _, end := range groups {
		visible := min(end, last) - max(start, first)
		if visible > 0 {
			view, rows := ui.ListRows(m.th, ui.ListOptions{Items: items[start:end], Selected: sel - start, Width: m.width, Height: visible})
			for i, text := range strings.Split(view, "\n") {
				if e := itemEntry[start+rows[i]]; e >= 0 {
					out = append(out, row{text: text, hit: hitEntry, index: e})
				} else {
					out = append(out, row{text: text})
				}
			}
		}
		start = end
	}
	return out
}

var sectionNames = map[int]string{secStaged: "Staged", secChanges: "Changes", secWorktrees: "Worktrees", secHistory: "History"}

// commitCols are a history row's cells: its short hash, the agent Snow
// saw make it when known, and its subject. The mark comes before the subject
// so a narrow pane cuts the subject, not the mark.
func commitCols(cm api.GitCommit) []string {
	parts := []string{ui.PlainLine(cm.Short)}
	if cm.Agent != "" {
		parts = append(parts, "by "+ui.PlainLine(cm.Agent))
	}
	parts = append(parts, ui.PlainLine(cm.Subject))
	return []string{"", strings.Join(parts, "  ")}
}

// worktreeCols are a worktree row's cells: its current-checkout mark,
// folder, branch, lock, and owner. Empty cells preserve column alignment.
func (m *Model) worktreeCols(wt api.GitWorktree) []string {
	mark, branch, lock := "", ui.PlainLine(wt.Branch), ""
	if m.current(wt) {
		mark = "*"
	}
	if branch == "" {
		branch = "detached"
	}
	if wt.Locked {
		lock = "locked"
	}
	return []string{mark, ui.FolderName(ui.PlainLine(wt.Path)), branch, lock, ownerText(wt, owners(wt, m.st.Worktrees, m.agents))}
}

// current reports whether wt is the checkout the pane's folder is in.
func (m *Model) current(wt api.GitWorktree) bool {
	return len(owners(wt, m.st.Worktrees, []api.AgentInfo{{Folder: m.st.Folder}})) > 0
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
