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

// ownClick is a left press in a pane the client draws, kept until the
// button is released. A press that moves over text is a selection, so it
// sends no click.
type ownClick struct {
	pane, x, y int // the pane and the cell pressed, in pane coordinates
}

// mouseMsg handles one mouse event. Clicks pick what they hit: a tab selects
// its window, a border starts a resize drag, and a pane takes the focus. A
// pane whose program asked for the mouse also gets the event, so htop sees
// the click that focuses it. The wheel goes to the pane under the pointer.
func (m *Model) mouseMsg(msg tea.MouseMsg) tea.Cmd {
	// Copy mode takes the mouse first: no border drag or focus change may
	// start while it is open.
	if m.copy != nil && m.layout != nil {
		return m.copyMouse(msg)
	}
	if m.menu != nil {
		return m.menuMouse(msg)
	}
	if m.switcher != nil {
		return m.switcherMouse(msg)
	}
	if m.authPick != nil {
		return m.authMouse(msg)
	}
	if m.modelPick != nil {
		return m.modelMouse(msg)
	}
	if m.over != nil {
		return m.overMouse(msg)
	}
	if m.prompt != nil {
		return m.promptMouse(msg)
	}
	if m.confirm != nil {
		m.confirmMouse(msg)
		return nil
	}
	if m.modal() {
		return nil
	}
	if m.shellStartupPending() {
		return nil
	}
	ev := msg.Mouse()
	switch msg.(type) {
	case tea.MouseClickMsg:
		m.endPrefix()
		// A release can be lost, so every press starts clean.
		m.drag, m.grab, m.dragFrom, m.ownPress = nil, nil, nil, nil
		return m.mousePress(ev)
	case tea.MouseMotionMsg:
		if m.dragFrom != nil && ev.Button == tea.MouseLeft {
			return m.dragToCopy(ev)
		}
		m.mouseMotion(ev)
	case tea.MouseReleaseMsg:
		m.dragFrom = nil
		return m.mouseRelease(ev)
	case tea.MouseWheelMsg:
		return m.mouseWheel(ev)
	}
	return nil
}

// hit says what lies under a screen cell.
func (m *Model) hit(x, y int) mouse.Hit {
	var panes []api.PaneInfo
	if w := m.shown(); w != nil {
		panes = w.Panes
	}
	if y == 0 {
		_, spans := ui.WindowBarRow(m.theme, m.windowBarOptions())
		for _, span := range spans {
			if x < span.X || x >= span.X+span.W {
				continue
			}
			switch span.Kind {
			case ui.TopBarSession:
				return mouse.Hit{Kind: mouse.Session}
			case ui.TopBarTab:
				return mouse.Hit{Kind: mouse.Tab, Window: span.Index}
			case ui.TopBarNewWindow:
				return mouse.Hit{Kind: mouse.NewWindowButton}
			case ui.TopBarNeedsYou:
				return mouse.Hit{Kind: mouse.NeedsYouButton}
			case ui.TopBarMenu:
				return mouse.Hit{Kind: mouse.MenuButton}
			}
		}
		return mouse.Hit{Kind: mouse.Bar}
	}
	if y == m.rows-1 {
		return mouse.Hit{Kind: mouse.Bar}
	}
	return mouse.HitTest(panes, mouse.Span{}, nil, x, y-m.areaTop(), m.cols, m.rows-m.areaTop())
}

func (m *Model) mousePress(ev tea.Mouse) tea.Cmd {
	if ev.Button == tea.MouseLeft {
		for _, b := range ui.PaneButtons(m.windowPanes(), m.cols, m.windowRows()) {
			if ev.Y-m.areaTop() == b.Y && ev.X >= b.X && ev.X < b.X+b.W {
				return m.run(b.Action, tea.Key{})
			}
		}
	}
	h := m.hit(ev.X, ev.Y)
	switch h.Kind {
	case mouse.Session:
		if ev.Button == tea.MouseLeft {
			return m.run("sessions", tea.Key{})
		}
	case mouse.NewWindowButton:
		if ev.Button == tea.MouseLeft {
			return m.openNewWindow()
		}
	case mouse.MenuButton:
		if ev.Button == tea.MouseLeft {
			m.openWindowMenu()
		}
	case mouse.NeedsYouButton:
		if ev.Button == tea.MouseLeft {
			return m.run("inbox", tea.Key{})
		}
	case mouse.Tab:
		switch ev.Button {
		case tea.MouseLeft:
			m.send(&api.Command{Op: api.OpSelectWindow, Target: h.Window})
		case tea.MouseMiddle:
			m.confirmCloseWindowAt(h.Window)
		}
	case mouse.Border:
		if ev.Button == tea.MouseLeft {
			m.drag = mouse.StartDrag(h, ev.X, ev.Y-m.areaTop())
		}
	case mouse.Pane:
		p := m.paneByID(h.Pane)
		if p == nil {
			return nil
		}
		if (ev.Button == tea.MouseLeft || ev.Button == tea.MouseRight) && !p.Focused {
			m.send(&api.Command{Op: api.OpFocus, Target: p.ID})
		}
		if o := m.own[p.ID]; o != nil {
			switch ev.Button {
			case tea.MouseLeft:
				// A click waits for the release: a press that moves is a
				// selection, not a click.
				m.ownPress = &ownClick{pane: p.ID, x: h.X, y: h.Y}
			case tea.MouseRight:
				m.openMenu(ev.X, ev.Y-m.areaTop())
			}
			return nil
		}
		st := m.panes[p.ID]
		wantsMouse := st != nil && st.mouse
		// A program that uses the mouse gets the right click too; Ctrl keeps
		// the menu reachable inside it.
		if ev.Button == tea.MouseRight && (!wantsMouse || ev.Mod.Contains(tea.ModCtrl)) {
			m.openMenu(ev.X, ev.Y-m.areaTop())
			return nil
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
	return nil
}

func (m *Model) mouseMotion(ev tea.Mouse) {
	if m.ownPress != nil {
		// The pointer moved with the button held: the user is selecting
		// text, so the press is not a click.
		m.ownPress = nil
		return
	}
	if m.drag != nil {
		if cmd := m.drag.Move(ev.X, ev.Y-m.areaTop()); cmd != nil {
			m.send(cmd)
		}
		return
	}
	if m.grab != nil {
		m.sendGrabbed(ev, api.ActionMotion)
	}
}

func (m *Model) mouseRelease(ev tea.Mouse) tea.Cmd {
	m.drag = nil
	var cmd tea.Cmd
	if c := m.ownPress; c != nil {
		m.ownPress = nil
		if o := m.own[c.pane]; o != nil {
			msg, note := o.Click(c.x, c.y)
			cmd = m.ownResult(c.pane, o, msg, note)
		}
	}
	if m.grab != nil {
		m.sendGrabbed(ev, api.ActionRelease)
		m.grab = nil
	}
	return cmd
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
	y := min(max(ev.Y-p.Rect.Y-m.areaTop(), 0), max(p.Rect.H-1, 0))
	m.send(&api.Mouse{Pane: p.ID, X: x, Y: y, Button: m.grab.button, Action: action})
}

// mouseWheel sends a wheel step to the pane under the pointer when its
// program asked for the mouse.
func (m *Model) mouseWheel(ev tea.Mouse) tea.Cmd {
	h := m.hit(ev.X, ev.Y)
	if h.Kind != mouse.Pane {
		return nil
	}
	st := m.panes[h.Pane]
	b := apiButton(ev.Button)
	if st == nil || (b != api.ButtonWheelUp && b != api.ButtonWheelDown) {
		return nil
	}
	if c := m.own[h.Pane]; c != nil {
		if b == api.ButtonWheelUp {
			c.Scroll(wheelRows)
		} else {
			c.Scroll(-wheelRows)
		}
		return nil
	}
	if st.mouse {
		m.send(&api.Mouse{Pane: h.Pane, X: h.X, Y: h.Y, Button: b, Action: api.ActionPress})
		return nil
	}
	// A pane that did not ask for the mouse scrolls its history, as in tmux:
	// the wheel focuses it and opens copy mode over it, so keys go where
	// the user is looking. Copy mode keeps the notch until its history
	// arrives.
	if p := m.paneByID(h.Pane); p != nil && !p.Focused {
		m.send(&api.Command{Op: api.OpFocus, Target: p.ID})
	}
	if !m.enterCopyOn(h.Pane) {
		return nil
	}
	return m.copyMouse(tea.MouseWheelMsg(ev))
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
