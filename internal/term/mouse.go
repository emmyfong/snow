package term

import (
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

// Button is a mouse button or wheel direction.
type Button int

// Mouse buttons.
const (
	ButtonNone Button = iota
	ButtonLeft
	ButtonMiddle
	ButtonRight
	ButtonWheelUp
	ButtonWheelDown
)

// Action is what happened to a button.
type Action int

// Mouse actions. A wheel step is a Press.
const (
	ActionPress Action = iota
	ActionRelease
	ActionMotion
)

// Mouse is one mouse event at a cell of the pane, counted from 0.
type Mouse struct {
	X, Y   int
	Button Button
	Action Action
}

// mouseModes are the terminal modes in which a program asks to receive
// mouse events: X10 clicks, clicks and releases, highlight, drags, and all
// motion.
var mouseModes = []ansi.Mode{
	ansi.ModeMouseX10, ansi.ModeMouseNormal, ansi.ModeMouseHighlight,
	ansi.ModeMouseButtonEvent, ansi.ModeMouseAnyEvent,
}

// trackMouse records a mode change. It runs inside the emulator with mu
// held, so it must not lock mu.
func (p *Pane) trackMouse(mode ansi.Mode, on bool) {
	if mode == ansi.ModeMouseExtSgr {
		p.mouseSGR = on
		return
	}
	for _, m := range mouseModes {
		if m != mode {
			continue
		}
		if p.mouseModes == nil {
			p.mouseModes = map[ansi.Mode]bool{}
		}
		if on {
			p.mouseModes[mode] = true
		} else {
			delete(p.mouseModes, mode)
		}
	}
}

// MouseMode reports whether the program asked to receive mouse events. Snow
// then sends it clicks, drags, and the wheel instead of acting on them.
func (p *Pane) MouseMode() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.mouseModes) > 0
}

// maxLegacyCell is the last cell the old encoding can name: one byte holds
// 32 plus the 1-based cell.
const maxLegacyCell = 222

// SendMouse encodes m for the program's mouse mode and sends it. A program
// gets only the events its mode asks for: X10 (9) wants presses; 1000 and
// 1001 add releases and the wheel; 1002 adds motion with a button held; 1003
// adds hover. A program that did not ask for the mouse gets nothing, because
// the bytes would look like typed text.
func (p *Pane) SendMouse(m Mouse) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.mouseModes) == 0 {
		return
	}
	has := func(modes ...ansi.Mode) bool {
		for _, mode := range modes {
			if p.mouseModes[mode] {
				return true
			}
		}
		return false
	}
	wheel := m.Button == ButtonWheelUp || m.Button == ButtonWheelDown
	onlyX10 := !has(ansi.ModeMouseNormal, ansi.ModeMouseHighlight, ansi.ModeMouseButtonEvent, ansi.ModeMouseAnyEvent)
	switch {
	case wheel && onlyX10, m.Action == ActionRelease && onlyX10:
		return
	case m.Action == ActionMotion && m.Button == ButtonNone && !has(ansi.ModeMouseAnyEvent):
		return
	case m.Action == ActionMotion && !has(ansi.ModeMouseButtonEvent, ansi.ModeMouseAnyEvent):
		return
	case !p.mouseSGR && (m.X > maxLegacyCell || m.Y > maxLegacyCell):
		return
	}
	var ev vt.Mouse
	switch {
	case wheel:
		ev = vt.MouseWheel{X: m.X, Y: m.Y, Button: vtButton(m.Button)}
	case m.Action == ActionRelease && p.mouseSGR:
		ev = vt.MouseRelease{X: m.X, Y: m.Y, Button: vtButton(m.Button)}
	case m.Action == ActionRelease:
		// The old encoding has no release button: it says "button 3".
		ev = vt.MouseClick{X: m.X, Y: m.Y, Button: vt.MouseNone}
	case m.Action == ActionMotion:
		ev = vt.MouseMotion{X: m.X, Y: m.Y, Button: vtButton(m.Button)}
	default:
		ev = vt.MouseClick{X: m.X, Y: m.Y, Button: vtButton(m.Button)}
	}
	p.emu.SendMouse(ev)
}

func vtButton(b Button) vt.MouseButton {
	switch b {
	case ButtonLeft:
		return vt.MouseLeft
	case ButtonMiddle:
		return vt.MouseMiddle
	case ButtonRight:
		return vt.MouseRight
	case ButtonWheelUp:
		return vt.MouseWheelUp
	case ButtonWheelDown:
		return vt.MouseWheelDown
	}
	return vt.MouseNone
}
