// Package settings is the settings screen and its group interface.
package settings

import (
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
)

// Group is one section of the settings screen. Feature packages implement it
// without importing this package.
type Group interface {
	// Title is the name shown in the group list.
	Title() string
	// Update handles keys and clicks while the group has focus.
	Update(msg tea.Msg) tea.Cmd
	// Leave runs when Escape returns focus to the group list.
	Leave() tea.Cmd
	// View draws the group in the area beside the group list.
	View(th theme.Theme, width, height int) string
}

type sizedGroup interface {
	SetSize(width, height int)
}

type committedThemeGroup interface {
	SetCommittedTheme(theme.Theme)
}

// CloseMsg asks the app to close the settings screen.
type CloseMsg struct{}

// Model is the settings screen.
type Model struct {
	groups            []Group
	notice            string
	selected          int
	active            bool
	cols, rows        int
	theme             theme.Theme
	base              theme.Theme
	profile           *colorprofile.Profile
	paintedBackground color.Color
}

// New returns the settings screen with groups in the order given.
func New(groups []Group) *Model {
	th := theme.Default()
	return &Model{groups: append([]Group(nil), groups...), theme: th, base: th}
}

// AddGroups appends groups after the existing screen state has been created.
func (m *Model) AddGroups(groups []Group) {
	m.groups = append(m.groups, groups...)
	m.sizeGroups()
}

// SetNotice shows a config read failure while the screen uses defaults.
func (m *Model) SetNotice(notice string) {
	m.notice = notice
	m.sizeGroups()
}

// SetSize sets the screen size in cells.
func (m *Model) SetSize(cols, rows int) {
	m.cols, m.rows = cols, rows
	m.sizeGroups()
}

func (m *Model) groupSize() (int, int) {
	w := min(72, m.cols-4)
	if w < 4 {
		return 0, 0
	}
	extraRows := 1
	if m.notice != "" {
		extraRows++
	}
	listRows := max(m.rows-4, extraRows+1) - extraRows
	inner := w - 2
	listWidth := min(22, max(inner/3, 12))
	return max(inner-listWidth-3, 1), listRows
}

func (m *Model) sizeGroups() {
	width, height := m.groupSize()
	for _, group := range m.groups {
		if sized, ok := group.(sizedGroup); ok {
			sized.SetSize(width, height)
		}
	}
}

// SetTheme changes the theme used to draw the screen.
func (m *Model) SetTheme(th theme.Theme) {
	m.base = th
	m.adaptTheme()
}

// SetPreviewTheme changes the drawing theme without changing the restore point.
func (m *Model) SetPreviewTheme(th theme.Theme) {
	m.theme = th
	if m.profile != nil {
		m.theme = m.theme.ForBackground(*m.profile, m.paintedBackground)
	}
}

// SetCommittedTheme updates group restore points after an external config change.
func (m *Model) SetCommittedTheme(th theme.Theme) {
	for _, group := range m.groups {
		if themed, ok := group.(committedThemeGroup); ok {
			themed.SetCommittedTheme(th)
		}
	}
}

// CurrentTheme returns the raw theme shown by the active group, or the screen.
func (m *Model) CurrentTheme() theme.Theme {
	if m.active {
		if group, ok := m.groups[m.selected].(interface {
			ThemePreview() (theme.Theme, bool)
		}); ok {
			if th, previewing := group.ThemePreview(); previewing {
				return th
			}
		}
	}
	return m.base
}

// SetPaintedBackground adapts secondary text to the background Snow applies.
func (m *Model) SetPaintedBackground(bg color.Color) {
	m.paintedBackground = bg
	m.adaptTheme()
}

func (m *Model) adaptTheme() {
	m.theme = m.CurrentTheme()
	if m.profile != nil {
		m.theme = m.theme.ForBackground(*m.profile, m.paintedBackground)
	}
}

// Update handles settings navigation and sends focused input to its group.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.ColorProfileMsg:
		m.profile = &msg.Profile
		m.adaptTheme()
	case tea.KeyPressMsg:
		if m.active {
			cmd := m.groups[m.selected].Update(msg)
			if msg.Code == tea.KeyEscape {
				leave := m.groups[m.selected].Leave()
				m.active = false
				return tea.Sequence(cmd, leave)
			}
			return cmd
		}
		switch msg.Code {
		case tea.KeyEscape:
			return func() tea.Msg { return CloseMsg{} }
		case tea.KeyUp:
			m.selected = max(m.selected-1, 0)
		case tea.KeyDown:
			m.selected = min(m.selected+1, max(len(m.groups)-1, 0))
		case tea.KeyEnter:
			m.active = len(m.groups) > 0
		}
	case tea.MouseMsg:
		if m.active {
			if click, ok := msg.(tea.MouseClickMsg); ok && click.Button == tea.MouseLeft && m.onFooter(click.X, click.Y) {
				leave := m.groups[m.selected].Leave()
				m.active = false
				return leave
			}
			if click, ok := msg.(tea.MouseClickMsg); ok && !m.onDetail(click.X, click.Y) {
				return nil
			}
			return m.groups[m.selected].Update(m.groupMsg(msg))
		}
		click, ok := msg.(tea.MouseClickMsg)
		if !ok || click.Button != tea.MouseLeft {
			return nil
		}
		if m.onFooter(click.X, click.Y) {
			return func() tea.Msg { return CloseMsg{} }
		}
		if i, ok := m.groupAt(click.X, click.Y); ok {
			m.selected, m.active = i, true
		}
	}
	return nil
}

// View draws the group list and the focused group's content.
func (m *Model) View() tea.View {
	box := m.box()
	w, h := ui.Size(box)
	x, y := max((m.cols-w)/2, 0), max((m.rows-h)/2, 0)
	canvas := make([]string, max(m.rows, 0))
	for i := range canvas {
		canvas[i] = strings.Repeat(" ", max(m.cols, 0))
	}
	content := ui.Overlay(strings.Join(canvas, "\n"), box, x, y)
	v := tea.NewView(content)
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m *Model) box() string {
	w := min(72, m.cols-4)
	if w < 4 {
		return ""
	}
	inner := w - 2
	muted := lipgloss.NewStyle().Foreground(m.theme.Muted)
	var body []string
	if len(m.groups) == 0 {
		body = []string{muted.Render("No settings groups yet."), " " + ui.KeyHints(m.theme, []ui.Hint{{Key: "Esc", Desc: "close"}}, inner-1)}
	} else {
		rightWidth, listRows := m.groupSize()
		inner := w - 2
		listWidth := min(22, max(inner/3, 12))
		items := make([]ui.ListItem, len(m.groups))
		for i, group := range m.groups {
			items[i] = ui.ListItem{Cols: []string{group.Title()}}
		}
		list := strings.Split(ui.List(m.theme, ui.ListOptions{Items: items, Selected: m.selected, Width: listWidth, Height: listRows}), "\n")
		var detail []string
		if m.active {
			detail = strings.Split(m.groups[m.selected].View(m.theme, rightWidth, listRows), "\n")
		} else {
			detail = []string{muted.Render("Select a group and press Enter.")}
		}
		rows := make([]string, listRows)
		divider := lipgloss.NewStyle().Foreground(m.theme.Border).Render(m.theme.Glyphs.Vertical)
		for i := range rows {
			left, right := "", ""
			if i < len(list) {
				left = list[i]
			}
			if i < len(detail) {
				right = ansi.Truncate(detail[i], rightWidth, "")
			}
			left = ansi.Truncate(left, listWidth, "")
			left += strings.Repeat(" ", max(listWidth-ansi.StringWidth(left), 0))
			right += strings.Repeat(" ", max(rightWidth-ansi.StringWidth(right), 0))
			rows[i] = left + " " + divider + " " + right
		}
		footer := []ui.Hint{{Key: "Esc", Desc: "close"}}
		if m.active {
			footer[0].Desc = "back to groups"
		}
		body = append(body, rows...)
		if m.notice != "" {
			body = append(body, ansi.Truncate(muted.Render(m.notice), inner, ""))
		}
		body = append(body, " "+ui.KeyHints(m.theme, footer, inner-1))
	}
	return ui.Panel(m.theme, ui.PanelOptions{Title: "Settings", Body: strings.Join(body, "\n"), Width: w})
}

func (m *Model) groupAt(x, y int) (int, bool) {
	w := min(72, m.cols-4)
	box := m.box()
	_, h := ui.Size(box)
	left, top := max((m.cols-w)/2, 0), max((m.rows-h)/2, 0)
	inner := w - 2
	_, listRows := m.groupSize()
	listWidth := min(22, max(inner/3, 12))
	if x < left+1 || x >= left+1+listWidth || y < top+1 || y >= top+1+listRows {
		return 0, false
	}
	first, last := ui.ListWindow(len(m.groups), m.selected, listRows)
	i := first + y - (top + 1)
	return i, i < last
}

func (m *Model) onDetail(x, y int) bool {
	w := min(72, m.cols-4)
	if w < 4 {
		return false
	}
	box := m.box()
	_, h := ui.Size(box)
	left, top := max((m.cols-w)/2, 0), max((m.rows-h)/2, 0)
	inner := w - 2
	listWidth := min(22, max(inner/3, 12))
	detailWidth, listRows := m.groupSize()
	left += 1 + listWidth + 3
	top++
	return x >= left && x < left+detailWidth && y >= top && y < top+listRows
}

func (m *Model) groupMsg(msg tea.MouseMsg) tea.Msg {
	click, ok := msg.(tea.MouseClickMsg)
	if !ok {
		return msg
	}
	w := min(72, m.cols-4)
	box := m.box()
	_, h := ui.Size(box)
	left, top := max((m.cols-w)/2, 0), max((m.rows-h)/2, 0)
	inner := w - 2
	listWidth := min(22, max(inner/3, 12))
	click.X -= left + 1 + listWidth + 3
	click.Y -= top + 1
	return click
}

func (m *Model) onFooter(x, y int) bool {
	box := m.box()
	w, h := ui.Size(box)
	left, top := max((m.cols-w)/2, 0), max((m.rows-h)/2, 0)
	return x >= left+1 && x < left+w-1 && y == top+h-2
}
