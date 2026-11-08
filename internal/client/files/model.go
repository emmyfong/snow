// Package files draws the files pane: a lazy tree of the folder with a
// mark for each file git changed. The server reads the folders and git; the
// pane sends the user's actions as api.SidePaneAction and draws each
// api.FilesState the server sends back.
package files

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// Model is the client half of a files pane.
type Model struct {
	id     int
	th     theme.Theme
	width  int
	height int
	st     api.FilesState
	loaded bool
	// sel is the selected path. The server's Selected reveals a file, but
	// moving in the tree stays local until the user opens a file.
	sel        string
	lastSelect string // the server's last Selected, so a reveal happens once
	top        int    // the first visible row
	note       string
}

// New returns a files pane for pane id.
func New(id int, th theme.Theme) *Model {
	return &Model{id: id, th: th}
}

// SetSize sets the pane's size in cells.
func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
	m.reveal()
}

// SetTheme sets the theme.
func (m *Model) SetTheme(th theme.Theme) { m.th = th }

// SetState shows st.
func (m *Model) SetState(st api.FilesState) {
	m.st = st
	m.loaded = true
	if st.Selected != "" && st.Selected != m.lastSelect {
		m.lastSelect = st.Selected
		if m.index(st.Selected) >= 0 {
			m.sel = st.Selected
		}
	}
	rows := m.rows()
	if len(rows) == 0 {
		m.sel = ""
		m.top = 0
		return
	}
	if m.index(m.sel) < 0 {
		m.sel = rows[0].entry.Path
	}
	m.reveal()
}

// Note returns a short message for the status bar, or "".
func (m *Model) Note() string { return m.note }

// row is one drawn line of the tree.
type row struct {
	entry api.FilesEntry
	depth int
}

// rows flattens the expanded folders into the visible tree rows.
func (m *Model) rows() []row {
	folders := make(map[string][]api.FilesEntry, len(m.st.Folders))
	expanded := make(map[string]bool, len(m.st.Expanded))
	for _, f := range m.st.Folders {
		folders[f.Path] = f.Entries
	}
	for _, p := range m.st.Expanded {
		expanded[p] = true
	}
	var out []row
	var walk func(parent string, depth int)
	walk = func(parent string, depth int) {
		for _, e := range folders[parent] {
			out = append(out, row{entry: e, depth: depth})
			if e.Dir && expanded[e.Path] {
				walk(e.Path, depth+1)
			}
		}
	}
	walk("", 0)
	return out
}

// index returns the row of path, or -1.
func (m *Model) index(path string) int {
	for i, r := range m.rows() {
		if r.entry.Path == path {
			return i
		}
	}
	return -1
}

// move selects row i, clamped to the tree.
func (m *Model) move(i int) {
	rows := m.rows()
	if len(rows) == 0 {
		m.sel = ""
		return
	}
	i = min(max(i, 0), len(rows)-1)
	m.sel = rows[i].entry.Path
	m.reveal()
}

// reveal scrolls the selected row into view.
func (m *Model) reveal() {
	i := m.index(m.sel)
	if i < 0 {
		m.top = 0
		return
	}
	h := m.bodyHeight()
	if h <= 0 {
		m.top = 0
		return
	}
	if i < m.top {
		m.top = i
	}
	if i >= m.top+h {
		m.top = i - h + 1
	}
	maxTop := max(len(m.rows())-h, 0)
	m.top = min(max(m.top, 0), maxTop)
}

// bodyHeight is the number of tree rows the pane shows.
func (m *Model) bodyHeight() int {
	h := m.height - 2 // header and hints
	if m.st.Error != "" && m.height >= 4 {
		h--
	}
	return max(h, 0)
}

// Key handles a key. It returns the message to send to the server, or nil.
func (m *Model) Key(k tea.Key) api.Message {
	if !m.loaded {
		return nil
	}
	rows := m.rows()
	i := m.index(m.sel)
	switch {
	case k.Code == tea.KeyUp || k.Text == "k":
		m.move(i - 1)
	case k.Code == tea.KeyDown || k.Text == "j":
		m.move(i + 1)
	case k.Code == tea.KeyPgUp:
		m.move(i - max(m.bodyHeight()-1, 1))
	case k.Code == tea.KeyPgDown:
		m.move(i + max(m.bodyHeight()-1, 1))
	case k.Code == tea.KeyHome:
		m.move(0)
	case k.Code == tea.KeyEnd:
		m.move(len(rows) - 1)
	case k.Code == tea.KeyEnter:
		if i < 0 {
			return nil
		}
		return m.activate(rows[i].entry)
	case k.Code == tea.KeyRight:
		if i < 0 {
			return nil
		}
		return m.right(rows, i)
	case k.Code == tea.KeyLeft:
		if i < 0 {
			return nil
		}
		return m.left(rows, i)
	case k.Text == "r":
		return m.action(api.FilesActRefresh, "")
	}
	return nil
}

// activate opens a file, or opens and closes a folder.
func (m *Model) activate(e api.FilesEntry) api.Message {
	if e.Dir {
		if m.expanded(e.Path) {
			return m.action(api.FilesActCollapse, e.Path)
		}
		return m.action(api.FilesActExpand, e.Path)
	}
	return &api.Command{Op: api.OpOpenFile, Target: m.id, Args: map[string]string{api.ArgPath: e.Path}}
}

// right opens a folder, or moves into an open one.
func (m *Model) right(rows []row, i int) api.Message {
	e := rows[i].entry
	if !e.Dir {
		return nil
	}
	if !m.expanded(e.Path) {
		return m.action(api.FilesActExpand, e.Path)
	}
	if i+1 < len(rows) && rows[i+1].depth > rows[i].depth {
		m.sel = rows[i+1].entry.Path
		m.reveal()
	}
	return nil
}

// left closes an open folder, or moves to the folder that holds the row.
func (m *Model) left(rows []row, i int) api.Message {
	e := rows[i].entry
	if e.Dir && m.expanded(e.Path) {
		return m.action(api.FilesActCollapse, e.Path)
	}
	parent := parentPath(e.Path)
	if parent == "" {
		return nil
	}
	if j := m.index(parent); j >= 0 {
		m.move(j)
	}
	return nil
}

// parentPath returns the folder of p, or "" at the root.
func parentPath(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return ""
}

// expanded reports whether path's children are shown.
func (m *Model) expanded(path string) bool {
	for _, p := range m.st.Expanded {
		if p == path {
			return true
		}
	}
	return false
}

// action returns the side pane action for the server, or nil.
func (m *Model) action(action, path string) api.Message {
	args := map[string]string{}
	if path != "" {
		args[api.ArgPath] = path
	}
	return &api.SidePaneAction{Pane: m.id, Action: action, Args: args}
}

// Click handles a left click at pane cell x, y. A click selects a row. A
// click on the selected folder opens or closes it, and a click on the
// selected file opens it.
func (m *Model) Click(x, y int) api.Message {
	if !m.loaded {
		return nil
	}
	rows := m.rows()
	i := m.top + y - 1
	if y < 1 || i < 0 || i >= len(rows) {
		return nil
	}
	e := rows[i].entry
	if m.sel != e.Path {
		m.sel = e.Path
		return nil
	}
	return m.activate(e)
}

// Scroll moves the selection by rows, toward older rows when positive.
func (m *Model) Scroll(rows int) { m.move(m.index(m.sel) - rows) }

// Paste inserts text into the pane; the files pane takes none.
func (m *Model) Paste(string) {}

// Animating reports whether the pane needs ticks.
func (m *Model) Animating() bool { return false }

// Tick advances the pane.
func (m *Model) Tick() {}
