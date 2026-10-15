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
