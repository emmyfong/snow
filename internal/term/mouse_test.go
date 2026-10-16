package term

import "testing"

const (
	enableMouse  = "\x1b[?1000h\x1b[?1006h" // report clicks, in the SGR encoding
	disableMouse = "\x1b[?1000l"
)

func TestMouseModeFollowsTheProgram(t *testing.T) {
	p, f := newTestPane(t)
	if p.MouseMode() {
		t.Fatal("a new pane reports mouse mode")
	}
	f.shellWrites(enableMouse)
	eventually(t, "mouse mode on", p.MouseMode)
	f.shellWrites(disableMouse)
	eventually(t, "mouse mode off", func() bool { return !p.MouseMode() })
}

// Programs ask for the mode they need: clicks only (1000), drags (1002), or
// all motion (1003). Each counts as mouse mode, and it ends only when the
// last one is off.
func TestMouseModeCountsEveryTrackingMode(t *testing.T) {
	for _, mode := range []string{"9", "1000", "1001", "1002", "1003"} {
		p, f := newTestPane(t)
		f.shellWrites("\x1b[?" + mode + "h")
		eventually(t, "mode "+mode, p.MouseMode)
		f.shellWrites("\x1b[?" + mode + "l")
		eventually(t, "mode "+mode+" off", func() bool { return !p.MouseMode() })
	}
	p, f := newTestPane(t)
	f.shellWrites("\x1b[?1000h\x1b[?1002h\x1b[?1000l")
	eventually(t, "1002 stays on", p.MouseMode)
	f.shellWrites("\x1b[?1002l")
	eventually(t, "all off", func() bool { return !p.MouseMode() })
}

func TestSendMouseEncodesForTheProgram(t *testing.T) {
	tests := []struct {
		name string
		m    Mouse
		want string
	}{
		{"left press", Mouse{X: 4, Y: 2, Button: ButtonLeft, Action: ActionPress}, "\x1b[<0;5;3M"},
		{"left release", Mouse{X: 4, Y: 2, Button: ButtonLeft, Action: ActionRelease}, "\x1b[<0;5;3m"},
		{"right press", Mouse{X: 0, Y: 0, Button: ButtonRight, Action: ActionPress}, "\x1b[<2;1;1M"},
		{"wheel up", Mouse{X: 4, Y: 2, Button: ButtonWheelUp, Action: ActionPress}, "\x1b[<64;5;3M"},
		{"wheel down", Mouse{X: 4, Y: 2, Button: ButtonWheelDown, Action: ActionPress}, "\x1b[<65;5;3M"},
	}
	for _, tt := range tests {
		p, f := newTestPane(t)
		f.shellWrites(enableMouse)
		eventually(t, "mouse mode", p.MouseMode)
		p.SendMouse(tt.m)
		eventually(t, tt.name, func() bool { return f.sent() == tt.want })
	}
}

func TestSendMouseDragReportsMotion(t *testing.T) {
	p, f := newTestPane(t)
	f.shellWrites("\x1b[?1002h\x1b[?1006h")
	eventually(t, "mouse mode", p.MouseMode)
	p.SendMouse(Mouse{X: 5, Y: 1, Button: ButtonLeft, Action: ActionMotion})
	eventually(t, "drag", func() bool { return f.sent() == "\x1b[<32;6;2M" })
}

// A program that never asked for the mouse must not get mouse bytes: they
// would appear as typed text.
func TestSendMouseIsSilentWithoutMouseMode(t *testing.T) {
	p, f := newTestPane(t)
	p.SendMouse(Mouse{X: 1, Y: 1, Button: ButtonLeft, Action: ActionPress})
	p.Paste("x") // arrives after any stray mouse bytes
	eventually(t, "the paste", func() bool { return f.sent() != "" })
	if got := f.sent(); got != "x" {
		t.Fatalf("program received %q, want only the paste", got)
	}
}

// sendsNothing sends m and then a paste; the program must receive only the
// paste, so any mouse bytes would show before it.
func sendsNothing(t *testing.T, name string, setup string, m Mouse) {
	t.Helper()
	p, f := newTestPane(t)
	f.shellWrites(setup)
	eventually(t, "mouse mode", p.MouseMode)
	p.SendMouse(m)
	p.Paste("x")
	eventually(t, name+": the paste", func() bool { return f.sent() != "" })
	if got := f.sent(); got != "x" {
		t.Errorf("%s: program received %q, want nothing from the mouse", name, got)
	}
}

func sends(t *testing.T, name, setup string, m Mouse, want string) {
	t.Helper()
	p, f := newTestPane(t)
	f.shellWrites(setup)
	eventually(t, "mouse mode", p.MouseMode)
	p.SendMouse(m)
	eventually(t, name, func() bool { return f.sent() == want })
}

// A program gets only the events its mode asks for: clicks (1000) do not
// include motion, drags (1002) do not include hover, and X10 (9) has no
// release or wheel.
func TestSendMouseHonorsTheMode(t *testing.T) {
	const sgr = "\x1b[?1006h"
	drag := Mouse{X: 3, Y: 1, Button: ButtonLeft, Action: ActionMotion}
	hover := Mouse{X: 3, Y: 1, Button: ButtonNone, Action: ActionMotion}
	press := Mouse{X: 3, Y: 1, Button: ButtonLeft, Action: ActionPress}
	rel := Mouse{X: 3, Y: 1, Button: ButtonLeft, Action: ActionRelease}
	wheel := Mouse{X: 3, Y: 1, Button: ButtonWheelUp, Action: ActionPress}

	sendsNothing(t, "drag in 1000", "\x1b[?1000h"+sgr, drag)
	sendsNothing(t, "hover in 1000", "\x1b[?1000h"+sgr, hover)
	sendsNothing(t, "hover in 1002", "\x1b[?1002h"+sgr, hover)
	sends(t, "drag in 1002", "\x1b[?1002h"+sgr, drag, "\x1b[<32;4;2M")
	sends(t, "hover in 1003", "\x1b[?1003h"+sgr, hover, "\x1b[<35;4;2M")
	sends(t, "drag in 1003", "\x1b[?1003h"+sgr, drag, "\x1b[<32;4;2M")
	sends(t, "press in X10", "\x1b[?9h"+sgr, press, "\x1b[<0;4;2M")
	sendsNothing(t, "release in X10", "\x1b[?9h"+sgr, rel)
	sendsNothing(t, "wheel in X10", "\x1b[?9h"+sgr, wheel)
	sendsNothing(t, "drag in X10", "\x1b[?9h"+sgr, drag)
	sends(t, "wheel in 1000", "\x1b[?1000h"+sgr, wheel, "\x1b[<64;4;2M")
}

// Without SGR (mode 1006) the program reads the old encoding: a release is
// button 3, and a cell past column 222 cannot be written.
func TestSendMouseLegacyEncoding(t *testing.T) {
	cell := func(n int) string { return string(rune(32 + n + 1)) }
	press := Mouse{X: 3, Y: 1, Button: ButtonLeft, Action: ActionPress}
	rel := press
	rel.Action = ActionRelease
	sends(t, "legacy press", "\x1b[?1000h", press, "\x1b[M"+" "+cell(3)+cell(1))
	sends(t, "legacy release is button 3", "\x1b[?1000h", rel, "\x1b[M"+"#"+cell(3)+cell(1))
	sends(t, "legacy last column", "\x1b[?1000h", Mouse{X: 222, Y: 1, Button: ButtonLeft, Action: ActionPress}, "\x1b[M"+" "+cell(222)+cell(1))
	sendsNothing(t, "legacy column 223", "\x1b[?1000h", Mouse{X: 223, Y: 1, Button: ButtonLeft, Action: ActionPress})
	sendsNothing(t, "legacy row 223", "\x1b[?1000h", Mouse{X: 1, Y: 223, Button: ButtonLeft, Action: ActionPress})
	sends(t, "SGR keeps large cells", "\x1b[?1000h\x1b[?1006h", Mouse{X: 250, Y: 1, Button: ButtonLeft, Action: ActionPress}, "\x1b[<0;251;2M")
}
