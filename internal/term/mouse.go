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

// SendMouse encodes m for the program's mouse mode and sends it. A program
// that did not ask for the mouse gets nothing, because the bytes would look
// like typed text.
func (p *Pane) SendMouse(m Mouse) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.mouseModes) == 0 {
		return
	}
	pos := vt.Mouse(vt.MouseClick{X: m.X, Y: m.Y, Button: vtButton(m.Button)})
	switch {
	case m.Button == ButtonWheelUp || m.Button == ButtonWheelDown:
		pos = vt.MouseWheel{X: m.X, Y: m.Y, Button: vtButton(m.Button)}
	case m.Action == ActionRelease:
		pos = vt.MouseRelease{X: m.X, Y: m.Y, Button: vtButton(m.Button)}
	case m.Action == ActionMotion:
		pos = vt.MouseMotion{X: m.X, Y: m.Y, Button: vtButton(m.Button)}
	}
	p.emu.SendMouse(pos)
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
