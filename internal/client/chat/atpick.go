package chat

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// atQuery returns the text after the @ the user is typing a path for, and
// whether there is one. The @ must start the input or follow a space, and
// no space may follow it up to the cursor, so a sentence that mentions an
// address does not open the picker.
func (m *Model) atQuery() (string, bool) {
	if m.cursor <= 0 || len(m.input) == 0 {
		return "", false
	}
	at := -1
	for i := m.cursor - 1; i >= 0; i-- {
		switch m.input[i] {
		case '@':
			at = i
		case ' ', '\t', '\n':
			return "", false
		default:
			continue
		}
		break
	}
	if at < 0 {
		return "", false
	}
	if at > 0 && !isSpace(m.input[at-1]) {
		return "", false
	}
	return string(m.input[at+1 : m.cursor]), true
}

// isSpace reports whether r separates words.
func isSpace(r rune) bool { return r == ' ' || r == '\t' || r == '\n' }

// syncAt opens the picker while the input holds a @ query, and closes it
// when the query ends or the user dismissed it. The selection resets when
// the filter changes.
func (m *Model) syncAt() {
	q, ok := m.atQuery()
	if !ok || m.commanding() || !m.loaded {
		m.atOpen, m.files.dismissed = false, false
		return
	}
	if q != m.files.query {
		m.files.sel, m.files.query = 0, q
		m.files.dismissed = false
	}
	m.atOpen = !m.files.dismissed
}

// atFilter splits the typed query into the path it filters by and the line
// suffix the user typed, such as ":12". The suffix goes in the inserted
// text as it is.
func atFilter(q string) (path, line string) {
	if i := strings.LastIndexByte(q, ':'); i >= 0 && allDigits(q[i+1:]) {
		return q[:i], q[i:]
	}
	return q, ""
}

// atMatches returns the files that match what is typed, in walk order.
func (m *Model) atMatches() []string {
	q, ok := m.atQuery()
	if !ok || !m.files.loaded {
		return nil
	}
	want, _ := atFilter(q)
	want = strings.ToLower(want)
	var out []string
	for _, p := range m.files.paths {
		if strings.Contains(strings.ToLower(p), want) {
			out = append(out, p)
		}
	}
	return out
}

// atKey handles the keys the picker takes while it is open, and reports
// whether it handled k. Up and Down choose a row, Enter and Tab insert the
// file, Ctrl+o opens it in the viewer, and Esc closes the picker without
// touching what is typed.
func (m *Model) atKey(k tea.Key) (*api.ChatInput, bool) {
	if !m.atOpen {
		return nil, false
	}
	ms := m.atMatches()
	switch {
	case k.Code == tea.KeyEscape:
		m.files.dismissed, m.atOpen = true, false
		return nil, true
	case k.Code == tea.KeyUp:
		m.files.sel = max(m.files.sel-1, 0)
	case k.Code == tea.KeyDown:
		m.files.sel = min(m.files.sel+1, max(len(ms)-1, 0))
	case k.Code == tea.KeyTab, k.Code == tea.KeyEnter:
		if len(ms) == 0 {
			if k.Code == tea.KeyEnter {
				m.note = "No file matches. Type more of the path, or Esc to close."
			}
			return nil, true
		}
		m.insertFile(ms[min(m.files.sel, len(ms)-1)])
		return nil, true
	case k.Code == 'o' && k.Mod == tea.ModCtrl:
		if len(ms) == 0 {
			return nil, true
		}
		m.askViewer(ms[min(m.files.sel, len(ms)-1)])
		return nil, true
	default:
		return nil, false
	}
	return nil, true
}

// insertFile replaces the @ query with path, keeping a line suffix the user
// typed. The picker stays shut for the path it just inserted, so Enter does
// not reopen it.
func (m *Model) insertFile(p string) {
	q, ok := m.atQuery()
	if !ok {
		return
	}
	_, line := atFilter(q)
	at := m.cursor - len([]rune(q)) - 1
	text := []rune("@" + p + line)
	next := make([]rune, 0, len(m.input)+len(text))
	next = append(next, m.input[:at]...)
	next = append(next, text...)
	next = append(next, m.input[m.cursor:]...)
	m.input, m.cursor = next, at+len(text)
	m.files.query, m.files.dismissed, m.atOpen = p+line, true, false
	m.Follow()
}

// askViewer asks the session to open the file in the viewer, at the line
// the user typed after it.
func (m *Model) askViewer(p string) {
	q, _ := m.atQuery()
	_, line := atFilter(q)
	n, _ := strconv.Atoi(strings.TrimPrefix(line, ":"))
	m.asked, m.askFile, m.askLine = AskOpenFile, p, n
}

// atLimit is the most rows the picker may take, leaving the input and the
// transcript room. It is 0 when the pane is too short for the picker.
func (m *Model) atLimit() int {
	if !m.atOpen || m.height <= 0 {
		return 0
	}
	avail := m.height - ui.TextAreaRows(m.inputOptions()) - 1 - m.barRows() - len(m.taskRows())
	if avail <= 0 {
		return 0
	}
	return min(max(m.height/3, 3), avail)
}

// atRows is the picker as drawn: its rows, and the file on each row (-1
// for a row that is not a file).
type atRows struct {
	lines []string
	files []int
}

// atView builds the picker for the current query. It draws the files the
// browser lists, a note while the list is on its way or cannot be read, and
// the keys.
func (m *Model) atView() (atRows, bool) {
	if !m.atOpen {
		return atRows{}, false
	}
	limit := m.atLimit()
	if limit <= 0 {
		return atRows{}, false
	}
	w := m.atRowsWidth()
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	noteRow := func(text string) string {
		line := muted.Render(ansi.Truncate(text, w, "…"))
		return line + strings.Repeat(" ", max(w-ansi.StringWidth(line), 0))
	}
	note := func(text string, keys bool) (atRows, bool) {
		v := atRows{lines: []string{noteRow(text)}, files: []int{-1}}
		if keys && limit > 1 {
			v.lines = append(v.lines, m.atHints(w))
			v.files = append(v.files, -1)
		}
		return v, true
	}
	switch {
	case !m.files.loaded && m.files.err != "":
		return note("Cannot read the files of this folder: "+ui.PlainLine(m.files.err), true)
	case !m.files.loaded:
		return note("Reading the files of this folder…", false)
	}
	ms := m.atMatches()
	if len(ms) == 0 {
		return note("No file matches.", true)
	}
	hints := 0
	if limit > 1 {
		hints = 1
	}
	cut := 0
	if m.files.cut && limit-hints-1 >= 1 {
		cut = 1
	}
	items := make([]ui.ListItem, len(ms))
	for i, p := range ms {
		items[i] = ui.ListItem{Cols: []string{p}}
	}
	view, rows := ui.ListRows(m.th, ui.ListOptions{Items: items, Selected: m.files.sel, Width: w, Height: limit - hints - cut})
	v := atRows{lines: strings.Split(view, "\n"), files: rows}
	if cut == 1 {
		v.lines = append(v.lines, noteRow("Only the first "+strconv.Itoa(len(m.files.paths))+" files are listed."))
		v.files = append(v.files, -1)
	}
	if hints == 1 {
		v.lines = append(v.lines, m.atHints(w))
		v.files = append(v.files, -1)
	}
	return v, true
}

// atHints draws the keys of the picker.
func (m *Model) atHints(width int) string {
	return ui.KeyHints(m.th, []ui.Hint{{Key: "Enter", Desc: "insert"}, {Key: "Ctrl+o", Desc: "view"}}, width)
}

// atRowsWidth is the width the picker's rows take: the pane.
func (m *Model) atRowsWidth() int {
	return max(m.width, 1)
}

// atHeight is how many rows the picker takes.
func (m *Model) atHeight() int {
	v, ok := m.atView()
	if !ok {
		return 0
	}
	return len(v.lines)
}

// atTop is the first screen row of the picker.
func (m *Model) atTop() int {
	return m.barRows() + m.transcriptHeight() + len(m.taskRows()) + 1
}

// atClick handles a click at y when it lands on a picker row: a click on a
// file inserts it. It reports whether the click was the picker's.
func (m *Model) atClick(y int) bool {
	v, ok := m.atView()
	if !ok {
		return false
	}
	row := y - m.atTop()
	if row < 0 || row >= len(v.lines) || row >= len(v.files) {
		return false
	}
	i := v.files[row]
	ms := m.atMatches()
	if i < 0 || i >= len(ms) {
		return true
	}
	m.insertFile(ms[i])
	return true
}
