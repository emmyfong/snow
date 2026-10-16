package session

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/pkg/api"
)

func click(x, y int) tea.MouseClickMsg {
	return tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft}
}

func motion(x, y int) tea.MouseMotionMsg {
	return tea.MouseMotionMsg{X: x, Y: y, Button: tea.MouseLeft}
}

func release(x, y int) tea.MouseReleaseMsg {
	return tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft}
}

// threePaneScreen is h(1, v(2, 3)) at 80x24, with pane 2 focused: pane 1 at
// columns 0-39, a border at column 40, pane 2 above pane 3 on the right, and
// a border at row 11 on the right.
func threePaneScreen(t *testing.T) (*Model, *recorder) {
	t.Helper()
	r := &recorder{}
	m := New(r.send)
	m.now = fixedClock
	m.SetSize(80, 24)
	panes := threePanes(80, 24)
	m.Update(&api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{
		{Index: 1, Name: "dev", Panes: panes},
		{Index: 2, Name: "logs"},
	}})
	return m, r
}

func lastMouse(t *testing.T, r *recorder) *api.Mouse {
	t.Helper()
	if len(r.sent) == 0 {
		t.Fatal("nothing sent")
	}
	ev, ok := r.sent[len(r.sent)-1].(*api.Mouse)
	if !ok {
		t.Fatalf("sent %T, want *api.Mouse", r.sent[len(r.sent)-1])
	}
	return ev
}

func TestClickPaneFocusesIt(t *testing.T) {
	m, r := threePaneScreen(t)
	m.Update(click(5, 5))
	c := lastCommand(t, r)
	if c.Op != api.OpFocus || c.Target != 1 {
		t.Fatalf("sent %+v, want focus pane 1", c)
	}
	if len(r.sent) != 1 {
		t.Fatalf("sent %d messages, want 1: a plain pane gets no mouse event", len(r.sent))
	}
}

func TestClickFocusedPaneSendsNothing(t *testing.T) {
	m, r := threePaneScreen(t)
	m.Update(click(50, 3)) // pane 2 has the focus
	m.Update(release(50, 3))
	if len(r.sent) != 0 {
		t.Fatalf("sent %v, want nothing", r.sent)
	}
}

func TestClickTabSelectsWindow(t *testing.T) {
	m, r := threePaneScreen(t)
	// The bar reads " [api] 1:dev* 2:logs": the second tab starts at column 14.
	m.Update(click(15, 23))
	c := lastCommand(t, r)
	if c.Op != api.OpSelectWindow || c.Target != 2 {
		t.Fatalf("sent %+v, want select-window 2", c)
	}
	r.sent = nil
	m.Update(click(70, 23)) // the bar, not a tab
	m.Update(click(2, 23))  // the session name
	if len(r.sent) != 0 {
		t.Fatalf("sent %v for clicks outside the tabs", r.sent)
	}
}

func TestOnlyTheLeftButtonSelectsAndFocuses(t *testing.T) {
	m, r := threePaneScreen(t)
	m.Update(tea.MouseClickMsg{X: 5, Y: 5, Button: tea.MouseMiddle})
	m.Update(tea.MouseClickMsg{X: 15, Y: 23, Button: tea.MouseMiddle})
	m.Update(tea.MouseClickMsg{X: 40, Y: 5, Button: tea.MouseRight})
	m.Update(motion(30, 5))
	if len(r.sent) != 0 {
		t.Fatalf("sent %v for non-left clicks", r.sent)
	}
}

func TestBorderDragSendsResizes(t *testing.T) {
	m, r := threePaneScreen(t)
	m.Update(click(40, 5))
	m.Update(motion(43, 5))
	c := lastCommand(t, r)
	if c.Op != api.OpResize || c.Target != 1 || c.Args[api.ArgDir] != api.DirRight || c.Args[api.ArgCells] != "3" {
		t.Fatalf("sent %+v, want resize pane 1 right by 3", c)
	}
	m.Update(release(43, 5))
	n := len(r.sent)
	m.Update(motion(50, 5)) // after the release, motion is not a drag
	if len(r.sent) != n {
		t.Fatalf("motion after release sent %v", r.sent[n:])
	}
}

func TestBorderDragFollowsTheNewLayout(t *testing.T) {
	m, r := threePaneScreen(t)
	m.Update(click(40, 5))
	m.Update(motion(45, 5))
	// The server moved the border only to column 43.
	moved := threePanes(80, 24)
	moved[0].Rect.W = 43
	moved[1].Rect.X, moved[1].Rect.W = 44, 36
	moved[2].Rect.X, moved[2].Rect.W = 44, 36
	m.Update(&api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "dev", Panes: moved}}})
	m.Update(motion(46, 5))
	c := lastCommand(t, r)
	if c.Target != 1 || c.Args[api.ArgCells] != "3" {
		t.Fatalf("sent %+v, want 3 more cells: the pointer is 3 right of the border", c)
	}
}

func TestHorizontalBorderDrag(t *testing.T) {
	m, r := threePaneScreen(t)
	m.Update(click(60, 11))
	m.Update(motion(60, 8))
	c := lastCommand(t, r)
	if c.Op != api.OpResize || c.Target != 3 || c.Args[api.ArgDir] != api.DirUp || c.Args[api.ArgCells] != "3" {
		t.Fatalf("sent %+v, want resize pane 3 up by 3", c)
	}
}

func mousePane(t *testing.T) (*Model, *recorder) {
	t.Helper()
	m, r := threePaneScreen(t)
	m.Update(&api.PaneUpdate{Pane: 1, Mouse: true}) // pane 1 runs a program that wants the mouse
	return m, r
}

// TestPaneThatWantsTheMouseGetsIt: htop must get the clicks inside its pane.
func TestPaneThatWantsTheMouseGetsIt(t *testing.T) {
	m, r := mousePane(t)
	m.Update(click(5, 6))
	if len(r.sent) != 2 {
		t.Fatalf("sent %v, want a focus command and the click", r.sent)
	}
	if c := r.sent[0].(*api.Command); c.Op != api.OpFocus || c.Target != 1 {
		t.Fatalf("first message %+v, want focus pane 1", c)
	}
	ev := lastMouse(t, r)
	if ev.Pane != 1 || ev.X != 5 || ev.Y != 6 || ev.Button != api.ButtonLeft || ev.Action != api.ActionPress {
		t.Fatalf("mouse event %+v, want a left press at 5,6 of pane 1", ev)
	}
	m.Update(release(7, 6))
	if ev := lastMouse(t, r); ev.Action != api.ActionRelease || ev.X != 7 {
		t.Fatalf("release %+v", ev)
	}
}

func TestMousePaneCellsAreRelativeToThePane(t *testing.T) {
	m, r := threePaneScreen(t)
	m.Update(&api.PaneUpdate{Pane: 3, Mouse: true})
	m.Update(click(45, 20)) // pane 3 starts at column 41, row 12
	if ev := lastMouse(t, r); ev.Pane != 3 || ev.X != 4 || ev.Y != 8 {
		t.Fatalf("mouse event %+v, want cell 4,8 of pane 3", ev)
	}
}

func TestGrabbedPaneKeepsTheDragOutsideIt(t *testing.T) {
	m, r := mousePane(t)
	m.Update(click(5, 6))
	m.Update(motion(70, 40)) // far outside pane 1
	ev := lastMouse(t, r)
	if ev.Pane != 1 || ev.Action != api.ActionMotion || ev.X != 39 || ev.Y != 22 {
		t.Fatalf("motion %+v, want the pointer held at pane 1's last cell 39,22", ev)
	}
	m.Update(motion(-3, -3))
	if ev := lastMouse(t, r); ev.X != 0 || ev.Y != 0 {
		t.Fatalf("motion %+v, want the pointer held at 0,0", ev)
	}
	m.Update(release(70, 40))
	if ev := lastMouse(t, r); ev.Action != api.ActionRelease || ev.Pane != 1 {
		t.Fatalf("release %+v", ev)
	}
	n := len(r.sent)
	m.Update(motion(5, 5))
	m.Update(release(5, 5))
	if len(r.sent) != n {
		t.Fatalf("events after the release reached the program: %v", r.sent[n:])
	}
}

func TestWheelGoesToPaneThatWantsTheMouse(t *testing.T) {
	m, r := mousePane(t)
	m.Update(tea.MouseWheelMsg{X: 10, Y: 4, Button: tea.MouseWheelDown})
	ev := lastMouse(t, r)
	if ev.Pane != 1 || ev.Button != api.ButtonWheelDown || ev.Action != api.ActionPress || ev.X != 10 || ev.Y != 4 {
		t.Fatalf("wheel %+v, want wheeldown at 10,4 of pane 1 (the pane under the pointer, not the focused one)", ev)
	}
	r.sent = nil
	m.Update(tea.MouseWheelMsg{X: 50, Y: 3, Button: tea.MouseWheelUp}) // pane 2 did not ask for the mouse
	m.Update(tea.MouseWheelMsg{X: 40, Y: 3, Button: tea.MouseWheelUp}) // a border
	m.Update(tea.MouseWheelMsg{X: 10, Y: 4, Button: tea.MouseWheelLeft})
	if len(r.sent) != 0 {
		t.Fatalf("sent %v, want nothing", r.sent)
	}
}

func TestMouseOffUpdateStopsForwarding(t *testing.T) {
	m, r := mousePane(t)
	m.Update(&api.PaneUpdate{Pane: 1}) // the program turned the mouse off
	m.Update(tea.MouseWheelMsg{X: 10, Y: 4, Button: tea.MouseWheelDown})
	if len(r.sent) != 0 {
		t.Fatalf("sent %v after the program stopped asking", r.sent)
	}
}

func TestMouseIgnoredWhileAPopupIsOpen(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.Update(ctrlB)
	m.Update(press('%', 0, "%"))
	m.Update(click(5, 5))
	m.Update(click(15, 23))
	if len(r.sent) != 0 {
		t.Fatalf("sent %v while the picker was open", r.sent)
	}
}

func TestMouseCancelsAWaitingPrefix(t *testing.T) {
	m, r := threePaneScreen(t)
	m.Update(ctrlB)
	m.Update(click(50, 3))
	m.Update(press('z', 0, "z"))
	if got := r.keys(); len(got) != 1 || got[0] != "z" {
		t.Fatalf("sent %v, want z to reach the pane after the click ended the prefix", r.sent)
	}
}

func TestViewAsksForTheMouse(t *testing.T) {
	m, _ := threePaneScreen(t)
	if got := m.View().MouseMode; got != tea.MouseModeCellMotion {
		t.Fatalf("MouseMode = %v, want cell motion: drags need motion events", got)
	}
}

func TestMouseBeforeFirstLayout(t *testing.T) {
	m := screen(80, 24)
	for _, msg := range []tea.Msg{click(5, 5), motion(6, 5), release(6, 5), tea.MouseWheelMsg{X: 1, Y: 1, Button: tea.MouseWheelUp}, click(5, 23)} {
		m.Update(msg)
	}
	if m.drag != nil || m.grab != nil {
		t.Fatal("mouse events before the first layout started a drag or grabbed a pane")
	}
}

// A release can be lost: the pointer leaves the window, or another program
// takes the mouse. The next press must start clean.
func TestPressClearsADragWhoseReleaseWasLost(t *testing.T) {
	m, r := threePaneScreen(t)
	m.Update(click(40, 5)) // a border drag starts
	m.Update(click(50, 3)) // the release never came; a new press
	m.Update(motion(55, 3))
	for _, s := range r.sent {
		if c, ok := s.(*api.Command); ok && c.Op == api.OpResize {
			t.Fatalf("a stale drag sent %+v", c)
		}
	}
	if m.drag != nil {
		t.Fatal("the drag survived a new press")
	}
}

func TestPressClearsAGrabWhoseReleaseWasLost(t *testing.T) {
	m, r := mousePane(t)
	m.Update(click(5, 6)) // pane 1 grabs the pointer
	m.Update(click(50, 3))
	r.sent = nil
	m.Update(motion(8, 8))
	m.Update(release(8, 8))
	if len(r.sent) != 0 {
		t.Fatalf("a stale grab sent %v", r.sent)
	}
}

func TestReleaseEndsEveryDragAndGrab(t *testing.T) {
	m, _ := mousePane(t)
	m.Update(click(5, 6))
	m.drag = nil
	m.Update(release(5, 6))
	if m.grab != nil {
		t.Fatal("release left the grab")
	}
	m, _ = threePaneScreen(t)
	m.Update(click(40, 5))
	m.Update(tea.MouseReleaseMsg{X: 40, Y: 5, Button: tea.MouseRight}) // another button's release
	if m.drag != nil {
		t.Fatal("a release left the drag")
	}
}
