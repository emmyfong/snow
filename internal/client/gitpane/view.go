package gitpane

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// boxRows is the height of the commit message box.
const boxRows = 3

// boxPrompt starts the commit message box.
const boxPrompt = "Message: "

// hitKind is what a row of the pane does on a click.
type hitKind int

const (
	hitNone  hitKind = iota
	hitEntry         // a change; index is its entry
	hitHints         // the key hints
	hitBox           // the commit message box
)

// hint is a key hint and the key a click on it presses.
type hint struct {
	ui.Hint
	key tea.Key
}

// layout is the drawn pane and what each row does.
type layout struct {
	rows   []string
	hits   []hitKind
	index  []int // the entry on each hitEntry row
	hints  []hint
	boxTop int // the first row of the commit box, or -1
}

func (l *layout) add(row string, kind hitKind, index int) {
	l.rows = append(l.rows, row)
	l.hits = append(l.hits, kind)
	l.index = append(l.index, index)
}

// View returns the pane's rows, each at most its width.
func (m *Model) View() []string {
	if m.width <= 0 || m.height <= 0 {
		return nil
	}
	return m.layout().rows
}

// Cursor says where the terminal's cursor goes: in the commit box while it
// is open.
func (m *Model) Cursor() (x, y int, ok bool) {
	l := m.layout()
	if m.mode != modeCommit || l.boxTop < 0 {
		return 0, 0, false
	}
	cx, cy := ui.TextAreaCursor(m.boxOptions())
	return cx + 1, l.boxTop + cy, true
}

// Click handles a left click at pane cell x, y and returns the action to
// send, or nil. A click selects a change, and a click on the selected one
// shows its diff. A click on a key hint presses its key, and a click on
// the commit box opens it.
func (m *Model) Click(x, y int) *api.SidePaneAction {
	if !m.loaded {
		return nil
	}
	l := m.layout()
	if y < 0 || y >= len(l.rows) {
		return nil
	}
	switch l.hits[y] {
	case hitEntry:
		es := m.entries()
		i := l.index[y]
		if i == m.selected(es) {
			return m.openDiff(es, i)
		}
		m.move(es, i)
	case hitHints:
		hs := make([]ui.Hint, len(l.hints))
		for i, h := range l.hints {
			hs[i] = h.Hint
		}
		// The hints row is indented by one cell.
		for _, sp := range ui.HintSpans(hs, m.width-1) {
			if x-1 >= sp.X && x-1 < sp.X+sp.W {
				return m.Key(l.hints[sp.Index].key)
			}
		}
	case hitBox:
		m.openCommit()
	}
	return nil
}

// Chrome rows: the header and the key hints, plus the status row and the
// commit box when they show. A pane too short for them shows the header
// and the list only.
func (m *Model) chrome() (status, box, hints bool) {
	// A read that failed shows its error in the body instead.
	status = (m.st.Error != "" && !m.readFailed()) || m.st.Notice != ""
	box = m.mode == modeCommit
	if m.height < 4 {
		return false, false, false
	}
	need := 3 // header, one list row, hints
	if box && m.height >= need+boxRows {
		need += boxRows
	} else {
		box = false
	}
	status = status && m.height > need
	return status, box, true
}

// listHeight is the rows the changes get.
func (m *Model) listHeight() int {
	status, box, hints := m.chrome()
	h := m.height - 1
	if status {
		h--
	}
	if box {
		h -= boxRows
	}
	if hints {
		h--
	}
	return max(h, 1)
}

// diffHeight is the rows the diff gets: all but the header and the hints.
func (m *Model) diffHeight() int {
	if m.height < 4 {
		return max(m.height-1, 1)
	}
	return m.height - 2
}

func (m *Model) layout() layout {
	l := layout{boxTop: -1}
	if m.width <= 0 || m.height <= 0 {
		return l
	}
	if m.mode == modeDiff && m.st.Diff != nil {
		m.diffLayout(&l)
	} else {
		m.listLayout(&l)
	}
	for len(l.rows) < m.height {
		l.add("", hitNone, 0)
	}
	l.rows, l.hits, l.index = l.rows[:m.height], l.hits[:m.height], l.index[:m.height]
	for i, r := range l.rows {
		l.rows[i] = ansi.Truncate(r, m.width, "")
	}
	return l
}

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
	case len(m.st.Changes) == 0:
		return note("No changes.")
	}
	es := m.entries()
	var items []ui.ListItem
	var itemEntry []int // the entry of each item, or -1 for a heading
	sel := -1
	nStaged := 0
	for _, e := range es {
		if e.staged {
			nStaged++
		}
	}
	codeW := 1
	pathW := max(m.width-2-codeW-3, 4) // pointer, code, column gap
	for i, e := range es {
		if i == 0 && e.staged {
			items, itemEntry = append(items, ui.ListItem{Heading: fmt.Sprintf("Staged (%d)", nStaged)}), append(itemEntry, -1)
		}
		if !e.staged && (i == 0 || es[i-1].staged) {
			items, itemEntry = append(items, ui.ListItem{Heading: fmt.Sprintf("Changes (%d)", len(es)-nStaged)}), append(itemEntry, -1)
		}
		if i == m.selected(es) {
			sel = len(items)
		}
		items, itemEntry = append(items, ui.ListItem{Cols: []string{code(e), cutLeft(pathText(e.change), pathW)}}), append(itemEntry, i)
	}
	if m.st.Omitted > 0 {
		items, itemEntry = append(items, ui.ListItem{Heading: fmt.Sprintf("%d more not shown", m.st.Omitted)}), append(itemEntry, -1)
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

// header is the first row: the branch in bold, then the folder and how far
// the branch is from its upstream, muted; "reading" while git runs.
func (m *Model) header() string {
	bold := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	var left string
	switch {
	case m.st.Detached:
		left = bold.Render(" detached HEAD")
	case m.st.Branch != "":
		left = bold.Render(" " + ui.PlainLine(m.st.Branch))
	default:
		left = bold.Render(" Git")
	}
	var extra []string
	if m.loaded && m.st.Folder != "" {
		extra = append(extra, "in "+ui.FolderName(m.st.Folder))
	}
	switch {
	case m.st.Initial:
		extra = append(extra, "no commits yet")
	case m.st.Ahead > 0 && m.st.Behind > 0:
		extra = append(extra, fmt.Sprintf("%d ahead, %d behind", m.st.Ahead, m.st.Behind))
	case m.st.Ahead > 0:
		extra = append(extra, fmt.Sprintf("%d ahead", m.st.Ahead))
	case m.st.Behind > 0:
		extra = append(extra, fmt.Sprintf("%d behind", m.st.Behind))
	}
	// Each part shows whole or not at all, the folder first.
	for i, e := range extra {
		sep := ", "
		if i == 0 {
			sep = "  "
		}
		if ansi.StringWidth(left)+ansi.StringWidth(sep+e) > m.width {
			break
		}
		left += muted.Render(sep + e)
	}
	if m.st.Loading {
		right := muted.Render("reading ")
		if gap := m.width - ansi.StringWidth(left) - ansi.StringWidth(right); gap >= 1 {
			return left + strings.Repeat(" ", gap) + right
		}
	}
	return left
}

// statusRow is the last action's failure, in the error colors, or its
// outcome.
func (m *Model) statusRow() string {
	if m.st.Error != "" {
		style := lipgloss.NewStyle().Foreground(m.th.ErrorFG).Background(m.th.Error)
		text := ansi.Truncate(" "+ui.PlainLine(m.st.Error), m.width, "...")
		return style.Render(text + strings.Repeat(" ", max(m.width-ansi.StringWidth(text), 0)))
	}
	return lipgloss.NewStyle().Foreground(m.th.Success).Render(" " + ui.PlainLine(m.st.Notice))
}

func (m *Model) boxOptions() ui.TextAreaOptions {
	return ui.TextAreaOptions{
		Width: max(m.width-1, 1), Height: boxRows, Prompt: boxPrompt,
		Value: string(m.msg), Cursor: m.cursor,
		Placeholder: "Enter commits, Esc cancels",
	}
}

func (m *Model) listHints() []hint {
	refresh := hint{ui.Hint{Key: "r", Desc: "refresh"}, tea.Key{Code: 'r', Text: "r"}}
	switch {
	case m.mode == modeCommit:
		return []hint{
			{ui.Hint{Key: "enter", Desc: "commit"}, tea.Key{Code: tea.KeyEnter}},
			{ui.Hint{Key: "ctrl+j", Desc: "new line"}, tea.Key{Code: 'j', Mod: tea.ModCtrl}},
			{ui.Hint{Key: "esc", Desc: "cancel"}, tea.Key{Code: tea.KeyEscape}},
		}
	case len(m.st.Changes) == 0:
		return []hint{refresh}
	}
	return []hint{
		{ui.Hint{Key: "s", Desc: "stage"}, tea.Key{Code: 's', Text: "s"}},
		{ui.Hint{Key: "u", Desc: "unstage"}, tea.Key{Code: 'u', Text: "u"}},
		{ui.Hint{Key: "c", Desc: "commit"}, tea.Key{Code: 'c', Text: "c"}},
		{ui.Hint{Key: "enter", Desc: "diff"}, tea.Key{Code: tea.KeyEnter}},
		{ui.Hint{Key: "a", Desc: "stage all"}, tea.Key{Code: 'a', Text: "a"}},
		refresh,
	}
}

func (m *Model) diffLayout(l *layout) {
	d := m.st.Diff
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	side := "unstaged"
	if d.Staged {
		side = "staged"
	}
	l.add(lipgloss.NewStyle().Bold(true).Render(" "+ui.PlainLine(d.Path))+muted.Render("  "+side), hitNone, 0)
	h := m.diffHeight()
	rows := m.diffRows()
	m.scroll = min(m.scroll, m.maxScroll())
	for _, r := range rows[m.scroll:min(m.scroll+h, len(rows))] {
		l.add(r, hitNone, 0)
	}
	for len(l.rows) < 1+h {
		l.add("", hitNone, 0)
	}
	if m.height < 4 {
		return
	}
	l.hints = []hint{{ui.Hint{Key: "esc", Desc: "back"}, tea.Key{Code: tea.KeyEscape}}}
	if d.Staged {
		l.hints = append(l.hints, hint{ui.Hint{Key: "u", Desc: "unstage"}, tea.Key{Code: 'u', Text: "u"}})
	} else {
		l.hints = append(l.hints, hint{ui.Hint{Key: "s", Desc: "stage"}, tea.Key{Code: 's', Text: "s"}})
	}
	l.hints = append(l.hints,
		hint{ui.Hint{Key: "pgup", Desc: "up"}, tea.Key{Code: tea.KeyPgUp}},
		hint{ui.Hint{Key: "pgdn", Desc: "down"}, tea.Key{Code: tea.KeyPgDown}})
	hs := make([]ui.Hint, len(l.hints))
	for i, h := range l.hints {
		hs[i] = h.Hint
	}
	l.add(" "+ui.KeyHints(m.th, hs, m.width-1), hitHints, 0)
}

// diffRows are the open diff's rows, drawn whole: git already keeps only
// the lines near each change.
func (m *Model) diffRows() []string {
	d := m.st.Diff
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	switch {
	case d.Binary:
		return []string{" " + muted.Render("Binary file. Git shows no diff for it.")}
	case len(d.Lines) == 0:
		return []string{" " + muted.Render("No change to show.")}
	}
	lines := make([]ui.DiffLine, len(d.Lines))
	for i, l := range d.Lines {
		op := ui.DiffKeep
		switch l.Kind {
		case api.GitLineAdd:
			op = ui.DiffAdd
		case api.GitLineRemove:
			op = ui.DiffRemove
		}
		lines[i] = ui.DiffLine{Op: op, Text: l.Text}
	}
	rows := strings.Split(ui.DiffView(m.th, ui.DiffOptions{Width: m.width, Lines: lines, Context: len(lines)}), "\n")
	if d.Cut {
		rows = append(rows, " "+muted.Render("The rest of the diff is too long to show."))
	}
	return rows
}

func (m *Model) maxScroll() int {
	if m.st.Diff == nil {
		return 0
	}
	return max(len(m.diffRows())-m.diffHeight(), 0)
}
