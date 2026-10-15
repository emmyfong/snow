package session

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/ui"
)

// View draws the shown window's panes with their borders, and the status bar
// on the bottom row. A client larger than the session shows blank space
// beyond the panes.
func (m *Model) View() tea.View {
	area := max(m.rows-1, 0)
	var panes []ui.WindowPane
	var tabs []ui.Tab
	session, right := "", ""
	if w := m.shown(); w != nil {
		for _, p := range w.Panes {
			st := m.panes[p.ID]
			var lines []string
			if st != nil {
				lines = st.lines
			}
			if rows := m.copyRows(p.ID); rows != nil {
				lines = rows
			}
			panes = append(panes, ui.WindowPane{X: p.Rect.X, Y: p.Rect.Y, W: p.Rect.W, H: p.Rect.H, Lines: lines, Focused: p.Focused})
		}
	}
	if m.layout != nil {
		session = m.layout.Session
		for _, w := range m.layout.Windows {
			tabs = append(tabs, ui.Tab{Index: w.Index, Name: w.Name, Active: w.Index == m.layout.Active, Zoomed: w.Zoomed})
		}
	}
	if p := m.focused(); p != nil {
		right = p.Profile + "  "
	}
	right += m.now().Format("15:04")
	content := m.overlayPicker(ui.Window(m.theme, ui.WindowOptions{Width: m.cols, Height: area, Panes: panes}), area)
	opts := ui.StatusOptions{Width: m.cols, Session: session, Tabs: tabs, Right: right, Message: m.message}
	if m.confirm != nil {
		opts.Prompt = m.confirm.question
	}
	bar := ui.StatusBar(m.theme, opts)
	if content != "" {
		content += "\n"
	}
	v := tea.NewView(content + bar)
	if p := m.focused(); p != nil && m.picker == nil {
		if x, y, ok := m.copyCursor(p.ID); ok {
			v.Cursor = tea.NewCursor(p.Rect.X+x, p.Rect.Y+y)
		} else if st := m.panes[p.ID]; st != nil && st.cursor.Visible && st.cursor.X < p.Rect.W && st.cursor.Y < p.Rect.H {
			v.Cursor = tea.NewCursor(p.Rect.X+st.cursor.X, p.Rect.Y+st.cursor.Y)
		}
	}
	return v
}
