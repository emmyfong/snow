package session

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// View draws the window bar, the shown window's panes with their headers,
// and the status line. A client larger than the session shows blank space
// beyond the panes.
func (m *Model) View() tea.View {
	area := m.windowRows()
	panes := m.windowPanes()
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
	windowBar := ui.WindowBar(m.theme, m.windowBarOptions())
	statusBar := ui.StatusBar(m.theme, m.statusOptions())
	var rows []string
	if m.rows > 0 {
		rows = append(rows, windowBar)
	}
	if area > 0 {
		rows = append(rows, content)
	}
	if m.rows > 1 {
		rows = append(rows, statusBar)
	}
	v := tea.NewView(strings.Join(rows, "\n"))
	v.MouseMode = tea.MouseModeCellMotion
	if p := m.focused(); p != nil && m.picker == nil && m.switcher == nil && m.modelPick == nil && m.authPick == nil && m.over == nil && m.prompt == nil {
		if x, y, ok := m.copyCursor(p.ID); ok {
			v.Cursor = tea.NewCursor(p.Rect.X+x, p.Rect.Y+y+m.areaTop())
		} else if c := m.own[p.ID]; c != nil {
			if x, y, ok := c.Cursor(); ok && m.confirm == nil && m.menu == nil && m.rename == nil {
				v.Cursor = tea.NewCursor(p.Rect.X+x, p.Rect.Y+y+m.areaTop())
			}
		} else if st := m.panes[p.ID]; st != nil && st.cursor.Visible && st.cursor.X < p.Rect.W && st.cursor.Y < p.Rect.H {
			v.Cursor = tea.NewCursor(p.Rect.X+st.cursor.X, p.Rect.Y+st.cursor.Y+m.areaTop())
		}
	}
	return v
}

func (m *Model) windowBarOptions() ui.WindowBarOptions {
	o := ui.WindowBarOptions{Width: m.cols}
	if m.layout != nil {
		o.Session = m.layout.Session
		for _, w := range m.layout.Windows {
			o.Tabs = append(o.Tabs, ui.Tab{Index: w.Index, Name: w.Name, Active: w.Index == m.layout.Active, Zoomed: w.Zoomed, Agent: windowMark(w), Driven: w.Driven})
		}
	}
	o.NeedsYou = m.needsYou()
	return o
}

// promptHint shows how to leave an open prompt.
const promptHint = "Esc: cancel"

// scrollModeLabel names scroll mode and its exit key on the status line.
const scrollModeLabel = "Scroll mode  Esc: exit"

// windowPanes returns the shown window's panes with the rows each one draws,
// for View and the pane controls' hit targets.
func (m *Model) windowPanes() []ui.WindowPane {
	w := m.shown()
	if w == nil {
		return nil
	}
	panes := make([]ui.WindowPane, 0, len(w.Panes))
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
		name := p.Profile
		if name == "" {
			name = p.Kind
		}
		panes = append(panes, ui.WindowPane{Name: name, Kind: p.Kind, X: p.Rect.X, Y: p.Rect.Y, W: p.Rect.W, H: p.Rect.H, Lines: lines, Focused: p.Focused, Driven: p.Driven})
	}
	return panes
}

// statusOptions describes the quiet bottom status line.
func (m *Model) statusOptions() ui.StatusOptions {
	opts := ui.StatusOptions{Width: m.cols, Session: m.current(), Message: m.message, Notice: m.notice}
	if p := m.focused(); p != nil {
		opts.Folder = ui.FolderTail(p.Folder)
	}
	if count := m.needsYou(); count > 0 {
		opts.Right = strconv.Itoa(count) + " need you"
	}
	if m.copy != nil && m.copy.Status() != "" {
		opts.Right = m.copy.Status()
		opts.Notice = ""
	}
	if m.copy != nil {
		opts.Mode = scrollModeLabel
	}
	switch {
	case m.rename != nil:
		opts.Prompt = m.renamePrompt()
	case m.copy != nil && m.copy.Prompt() != "":
		opts.Prompt = m.copy.Prompt()
	}
	// Scroll mode's own text names the exit key, so its prompt needs no hint.
	if (m.modal() || opts.Prompt != "") && m.copy == nil {
		opts.Hint = promptHint
	}
	return opts
}

func (m *Model) needsYou() int {
	count := 0
	for _, a := range m.agents {
		if a.State == api.AgentNeedsYou {
			count++
		}
	}
	return count
}

// marks maps the protocol's agent states to the status bar's marks.
var marks = map[string]ui.Mark{api.AgentIdle: ui.MarkIdle, api.AgentWorking: ui.MarkWorking, api.AgentNeedsYou: ui.MarkNeedsYou}

// windowMark is the most urgent agent state of w, as the server counts it,
// panes a zoom hides included.
func windowMark(w api.WindowInfo) ui.Mark { return marks[w.AgentState] }
