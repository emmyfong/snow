package session

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/mouse"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// grab is the pane that a held mouse button belongs to. A program that asked
// for the mouse keeps getting the motion and the release even when the
// pointer leaves its pane, as in any terminal.
type grab struct {
	pane   int
	button string
}

// mouseMsg handles one mouse event. Clicks pick what they hit: a tab selects
// its window, a border starts a resize drag, and a pane takes the focus. A
// pane whose program asked for the mouse also gets the event, so htop sees
// the click that focuses it. The wheel goes to the pane under the pointer.
func (m *Model) mouseMsg(msg tea.MouseMsg) tea.Cmd {
	// When copy mode (#55) merges, route to it before anything below: no
	// border drag or focus change may start while it is open.
	if m.menu != nil {
		return m.menuMouse(msg)
	}
	if m.confirm != nil {
		m.confirmMouse(msg)
		return nil
	}
	if m.modal() {
		return nil
	}
	ev := msg.Mouse()
	switch msg.(type) {
	case tea.MouseClickMsg:
		m.endPrefix()
		// A release can be lost, so every press starts clean.
		m.drag, m.grab = nil, nil
		m.mousePress(ev)
	case tea.MouseMotionMsg:
		m.mouseMotion(ev)
	case tea.MouseReleaseMsg:
		m.mouseRelease(ev)
	case tea.MouseWheelMsg:
		m.mouseWheel(ev)
	}
	return nil
}

// hit says what lies under a screen cell.
func (m *Model) hit(x, y int) mouse.Hit {
	var panes []api.PaneInfo
	if w := m.shown(); w != nil {
		panes = w.Panes
	}
	var tabs []mouse.TabSpan
	for _, t := range ui.TabSpans(m.theme, m.statusOptions()) {
		tabs = append(tabs, mouse.TabSpan{Index: t.Index, X: t.X, W: t.W})
	}
	return mouse.HitTest(panes, tabs, x, y, m.cols, m.rows)
}

func (m *Model) mousePress(ev tea.Mouse) {
	h := m.hit(ev.X, ev.Y)
	switch h.Kind {
	case mouse.Tab:
		switch ev.Button {
		case tea.MouseLeft:
			m.send(&api.Command{Op: api.OpSelectWindow, Target: h.Window})
		case tea.MouseMiddle:
			m.confirmCloseWindowAt(h.Window)
		}
	case mouse.Border:
		if ev.Button == tea.MouseLeft {
			m.drag = mouse.StartDrag(h, ev.X, ev.Y)
		}
	case mouse.Pane:
		p := m.paneByID(h.Pane)
		if p == nil {
			return
		}
		if (ev.Button == tea.MouseLeft || ev.Button == tea.MouseRight) && !p.Focused {
			m.send(&api.Command{Op: api.OpFocus, Target: p.ID})
		}
		st := m.panes[p.ID]
		wantsMouse := st != nil && st.mouse
		// A program that uses the mouse gets the right click too; Ctrl keeps
		// the menu reachable inside it.
		if ev.Button == tea.MouseRight && (!wantsMouse || ev.Mod.Contains(tea.ModCtrl)) {
			m.openMenu(ev.X, ev.Y)
			return
		}
		if ev.Button == tea.MouseLeft && !wantsMouse {
			m.startPaneDrag(p.ID, h.X, h.Y)
		}
		if wantsMouse {
			if b := apiButton(ev.Button); b != "" {
				m.send(&api.Mouse{Pane: p.ID, X: h.X, Y: h.Y, Button: b, Action: api.ActionPress})
				m.grab = &grab{pane: p.ID, button: b}
			}
		}
	}
}

func (m *Model) mouseMotion(ev tea.Mouse) {
	if m.drag != nil {
		if cmd := m.drag.Move(ev.X, ev.Y); cmd != nil {
			m.send(cmd)
		}
		return
	}
	if m.grab != nil {
		m.sendGrabbed(ev, api.ActionMotion)
	}
}

func (m *Model) mouseRelease(ev tea.Mouse) {
	m.drag = nil
	if m.grab != nil {
		m.sendGrabbed(ev, api.ActionRelease)
		m.grab = nil
	}
}

// sendGrabbed sends a motion or release to the grabbed pane, with the
// pointer held inside it.
func (m *Model) sendGrabbed(ev tea.Mouse, action string) {
	p := m.paneByID(m.grab.pane)
	if p == nil {
		m.grab = nil
		return
	}
	x := min(max(ev.X-p.Rect.X, 0), max(p.Rect.W-1, 0))
	y := min(max(ev.Y-p.Rect.Y, 0), max(p.Rect.H-1, 0))
	m.send(&api.Mouse{Pane: p.ID, X: x, Y: y, Button: m.grab.button, Action: action})
}

// mouseWheel sends a wheel step to the pane under the pointer when its
// program asked for the mouse.
func (m *Model) mouseWheel(ev tea.Mouse) {
	h := m.hit(ev.X, ev.Y)
	if h.Kind != mouse.Pane {
		return
	}
	st := m.panes[h.Pane]
	b := apiButton(ev.Button)
	if st == nil || (b != api.ButtonWheelUp && b != api.ButtonWheelDown) {
		return
	}
	if st.mouse {
		m.send(&api.Mouse{Pane: h.Pane, X: h.X, Y: h.Y, Button: b, Action: api.ActionPress})
	}
	// TODO(#55): a pane that did not ask for the mouse scrolls its
	// scrollback; copy mode owns the scrolled view.
}

// syncDrag keeps a border drag in step with a new layout and ends it when
// its panes are gone.
func (m *Model) syncDrag() {
	if m.drag == nil {
		return
	}
	var panes []api.PaneInfo
	if w := m.shown(); w != nil {
		panes = w.Panes
	}
	if !m.drag.Sync(panes) {
		m.drag = nil
	}
}

// paneByID returns the pane with id in the shown window, or nil.
func (m *Model) paneByID(id int) *api.PaneInfo {
	w := m.shown()
	if w == nil {
		return nil
	}
	for i := range w.Panes {
		if w.Panes[i].ID == id {
			return &w.Panes[i]
		}
	}
	return nil
}

// apiButton names a button for the protocol, or "" for one it has no name for.
func apiButton(b tea.MouseButton) string {
	switch b {
	case tea.MouseLeft:
		return api.ButtonLeft
	case tea.MouseMiddle:
		return api.ButtonMiddle
	case tea.MouseRight:
		return api.ButtonRight
	case tea.MouseWheelUp:
		return api.ButtonWheelUp
	case tea.MouseWheelDown:
		return api.ButtonWheelDown
	case tea.MouseNone:
		return api.ButtonNone
	}
	return ""
}
