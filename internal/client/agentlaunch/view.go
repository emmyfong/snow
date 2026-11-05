package agentlaunch

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/roletext"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
)

const (
	minWidth    = 36
	maxWidth    = 72
	maxListRows = 5
	labelWidth  = 9 // "Profile" and a gap
)

// branchHint is the branch field's placeholder: the server picks the name.
const branchHint = "agent/N, the next free number"

// about says what the form does.
const about = "The agent gets its own git worktree on the new branch, in a new window. A folder outside git is used as it is."

var hints = []ui.Hint{{Key: "Enter", Desc: "start"}, {Key: "Tab", Desc: "next"}, {Key: "Esc", Desc: "cancel"}}

// layout is where the form goes. The rows are screen rows, for mapping a
// click to a field or a profile.
type layout struct {
	box                  string
	x, y, w, h           int
	folderRow, branchRow int
	listTop, listRows    int
	listItems            []int // profile index per list row
	roleTop, roleRows    int
	roleItems            []int // role choice per role row: 0 is no role
}

// View draws the form centered on a window of cols by rows cells and
// returns the box with the cell of its top-left corner. It returns "" when
// the window is too small; the keys still answer.
func (m *Model) View(th theme.Theme, cols, rows int) (box string, x, y int) {
	l, ok := m.layout(th, cols, rows)
	if !ok {
		return "", 0, 0
	}
	return l.box, l.x, l.y
}

// Click handles a left click: a field takes the focus and a profile is
// picked. A click outside closes the form.
func (m *Model) Click(th theme.Theme, cols, rows, x, y int) picker.Result {
	l, ok := m.layout(th, cols, rows)
	if !ok || x < l.x || x >= l.x+l.w || y < l.y || y >= l.y+l.h {
		return picker.Cancelled
	}
	switch {
	case y == l.folderRow:
		m.focus = fieldFolder
	case y == l.branchRow:
		m.focus = fieldBranch
	case y >= l.listTop && y < l.listTop+l.listRows:
		m.focus = fieldProfile
		if i := y - l.listTop; i < len(l.listItems) {
			m.selected = l.listItems[i]
		}
	case l.roleRows > 0 && y >= l.roleTop && y < l.roleTop+l.roleRows:
		m.focus = fieldRole
		if i := y - l.roleTop; i < len(l.roleItems) {
			m.role = l.roleItems[i]
		}
	}
	return picker.Open
}

func (m *Model) layout(th theme.Theme, cols, rows int) (layout, bool) {
	w := min(maxWidth, cols-4)
	if w < minWidth {
		return layout{}, false
	}
	inner := w - 3 // the frame and one cell of margin
	fieldW := inner - labelWidth
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	var body []string
	add := func(s string) { body = append(body, " "+s) }
	label := func(text string, f field) string {
		style := muted
		if m.focus == f {
			style = lipgloss.NewStyle().Foreground(th.Accent).Bold(true)
		}
		return style.Render(ansi.Truncate(text, labelWidth, "")) + strings.Repeat(" ", max(labelWidth-ansi.StringWidth(text), 0))
	}

	l := layout{w: w}
	l.folderRow = len(body)
	add(label("Folder", fieldFolder) + ui.Input(th, ui.InputOptions{Value: string(m.folder), Width: fieldW, Focused: m.focus == fieldFolder}))
	l.branchRow = len(body)
	add(label("Branch", fieldBranch) + ui.Input(th, ui.InputOptions{Value: string(m.branch), Placeholder: branchHint, Width: fieldW, Focused: m.focus == fieldBranch}))
	body = append(body, "")
	add(label("Profile", fieldProfile))
	l.listTop = len(body)
	if len(m.profiles) == 0 {
		for _, line := range wrapText(noProfile, inner) {
			add(muted.Render(line))
		}
	} else {
		items := make([]ui.ListItem, len(m.profiles))
		for i, p := range m.profiles {
			items[i] = ui.ListItem{Cols: []string{p}}
		}
		selected := m.selected
		if m.focus != fieldProfile {
			selected = -1 // the list shows its choice only while it has the focus
		}
		list, rowItems := ui.ListRows(th, ui.ListOptions{Items: items, Selected: selected, Width: inner, Height: min(len(items), maxListRows)})
		for _, line := range strings.Split(list, "\n") {
			add(line)
		}
		l.listItems = rowItems
		if m.focus != fieldProfile {
			// Mark the chosen profile even without the highlight.
			body[l.listTop+indexOf(rowItems, m.selected)] = " " + chosen(th, m.profiles[m.selected], inner)
		}
	}
	l.listRows = len(body) - l.listTop
	if len(m.roles) > 0 {
		body = append(body, "")
		add(label("Role", fieldRole) + muted.Render(ansi.Truncate(m.roleLimits(), fieldW, "…")))
		l.roleTop = len(body)
		items := make([]ui.ListItem, 0, len(m.roles)+1)
		items = append(items, ui.ListItem{Cols: []string{noRole, ""}})
		for _, r := range m.roles {
			items = append(items, ui.ListItem{Cols: []string{r.Title, roletext.Short(r.PermissionMode, r.ReadOnly)}})
		}
		selected := m.role
		if m.focus != fieldRole {
			selected = -1
		}
		list, rowItems := ui.ListRows(th, ui.ListOptions{Items: items, Selected: selected, Width: inner, Height: min(len(items), maxListRows)})
		for _, line := range strings.Split(list, "\n") {
			add(line)
		}
		l.roleItems = rowItems
		if m.focus != fieldRole {
			body[l.roleTop+indexOf(rowItems, m.role)] = " " + chosen(th, items[m.role].Cols[0], inner)
		}
		l.roleRows = len(body) - l.roleTop
	}
	body = append(body, "")
	for _, line := range wrapText(about, inner) {
		add(muted.Render(line))
	}
	if m.note != "" {
		body = append(body, "")
		for _, line := range wrapText(m.note, inner) {
			add(lipgloss.NewStyle().Foreground(th.Error).Render(line))
		}
	}
	body = append(body, "")
	add(ui.KeyHints(th, hints, inner))

	l.h = len(body) + 2
	if l.h > rows {
		return layout{}, false
	}
	l.x, l.y = max((cols-w)/2, 0), max((rows-l.h)/2, 0)
	// Body rows start one row below the top edge.
	l.folderRow += l.y + 1
	l.branchRow += l.y + 1
	l.listTop += l.y + 1
	l.roleTop += l.y + 1
	l.box = ui.Panel(th, ui.PanelOptions{Title: "New agent", Body: strings.Join(body, "\n"), Width: w})
	return l, true
}

// noRole is the role list's first choice.
const noRole = "No role"

// roleLimits says what the chosen role limits, and whether Claude Code
// enforces it.
func (m *Model) roleLimits() string {
	if m.role == 0 {
		return "Claude Code's own settings"
	}
	r := m.roles[m.role-1]
	return roletext.Limits(r.PermissionMode, r.ReadOnly)
}

// chosen draws the picked profile, marked with the pointer, when the list
// does not have the focus.
func chosen(th theme.Theme, name string, width int) string {
	row := th.Glyphs.Pointer + name
	return ansi.Truncate(row, width, "…")
}

func indexOf(items []int, v int) int {
	for i, it := range items {
		if it == v {
			return i
		}
	}
	return 0
}

// wrapText wraps text to width, at most three rows.
func wrapText(text string, width int) []string {
	lines := strings.Split(ansi.Wrap(text, width, ""), "\n")
	return lines[:min(len(lines), 3)]
}
