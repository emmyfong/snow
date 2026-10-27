package session

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// View draws the shown window's panes with their borders, and the status bar
// on the bottom row. A client larger than the session shows blank space
// beyond the panes.
func (m *Model) View() tea.View {
	area := m.windowRows()
	var panes []ui.WindowPane
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
			if o := m.own[p.ID]; o != nil {
				lines = o.View()
			}
			panes = append(panes, ui.WindowPane{X: p.Rect.X, Y: p.Rect.Y, W: p.Rect.W, H: p.Rect.H, Lines: lines, Focused: p.Focused})
		}
	}
	content := ui.Window(m.theme, ui.WindowOptions{Width: m.cols, Height: area, Panes: panes})
	content = m.overlayPicker(content, area)
	content = m.overlaySwitcher(content, area)
	content = m.overlayModelPick(content, area)
	content = m.overlayAuthPick(content, area)
	content = m.overlayOver(content, area)
	content = m.overlayPrompt(content)
	content = m.overlayHint(content, area)
	content = m.overlayMenu(content, area)
	content = m.overlayConfirm(content, area)
	bar := ui.StatusBar(m.theme, m.statusOptions())
	if content != "" {
		content += "\n"
	}
	v := tea.NewView(content + bar)
	v.MouseMode = tea.MouseModeCellMotion
	if p := m.focused(); p != nil && m.picker == nil && m.switcher == nil && m.modelPick == nil && m.authPick == nil && m.over == nil && m.prompt == nil {
		if x, y, ok := m.copyCursor(p.ID); ok {
			v.Cursor = tea.NewCursor(p.Rect.X+x, p.Rect.Y+y)
		} else if c := m.own[p.ID]; c != nil {
			if x, y, ok := c.Cursor(); ok && m.confirm == nil && m.menu == nil && m.rename == nil {
				v.Cursor = tea.NewCursor(p.Rect.X+x, p.Rect.Y+y)
			}
		} else if st := m.panes[p.ID]; st != nil && st.cursor.Visible && st.cursor.X < p.Rect.W && st.cursor.Y < p.Rect.H {
			v.Cursor = tea.NewCursor(p.Rect.X+st.cursor.X, p.Rect.Y+st.cursor.Y)
		}
	}
	return v
}

// statusOptions describes the status bar. The view draws it, and a click on
// a tab is matched against it.
func (m *Model) statusOptions() ui.StatusOptions {
	opts := ui.StatusOptions{Width: m.cols, Message: m.message}
	if m.layout != nil {
		opts.Session = m.layout.Session
		for _, w := range m.layout.Windows {
			opts.Tabs = append(opts.Tabs, ui.Tab{Index: w.Index, Name: w.Name, Active: w.Index == m.layout.Active, Zoomed: w.Zoomed, Agent: windowMark(w)})
		}
	}
	if p := m.focused(); p != nil {
		opts.Right = p.Profile + "  "
	}
	note := m.notice
	if m.copy != nil && m.copy.Status() != "" {
		note = m.copy.Status()
	}
	if note != "" {
		opts.Right = note + "  "
	}
	opts.Right += m.now().Format("15:04")
	switch {
	case m.rename != nil:
		opts.Prompt = m.renamePrompt()
	case m.copy != nil && m.copy.Prompt() != "":
		opts.Prompt = m.copy.Prompt()
	}
	return opts
}

// marks maps the protocol's agent states to the status bar's marks.
var marks = map[string]ui.Mark{api.AgentIdle: ui.MarkIdle, api.AgentWorking: ui.MarkWorking, api.AgentNeedsYou: ui.MarkNeedsYou}

// windowMark is the most urgent agent state of w, as the server counts it,
// panes a zoom hides included.
func windowMark(w api.WindowInfo) ui.Mark { return marks[w.AgentState] }
