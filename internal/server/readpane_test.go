package server

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/pkg/api"
)

// readText joins a pane copy as the agent sees it, to look for text and
// styles in one place.
func readText(r *api.ReadPane) string {
	return strings.Join(r.Screen, "\n") + "\n" + strings.Join(r.Scrollback, "\n")
}

// TestReadPaneScreen: a read_pane call returns the shell's screen as plain
// text, with the pane's size and a visible cursor.
func TestReadPaneScreen(t *testing.T) {
	s := newTestServer(t)
	c, pane := attached(t, s, 100, 30)
	c.Send(&api.Input{Pane: pane, Paste: "echo snow-read-marker\r"})
	c.WaitScreen("snow-read-marker")

	tool := connect(t, s, 80, 24)
	tool.Send(&api.ReadPaneRequest{Caller: pane, Pane: pane})
	got, e := apitest.Reply[*api.ReadPane](tool)
	if e != nil {
		t.Fatalf("ReadPaneRequest: %v", e)
	}
	if got.Pane != pane || got.Session != "work" || got.Profile != "sh" {
		t.Fatalf("read pane %d of %q profile %q, want pane %d of work with sh", got.Pane, got.Session, got.Profile, pane)
	}
	if got.Cols != 100 || got.Rows != 29 {
		t.Fatalf("size %dx%d, want 100x29", got.Cols, got.Rows)
	}
	if !got.CursorVisible || got.CursorY < 1 {
		t.Fatalf("cursor (%d,%d) visible %v, want a visible cursor below the top", got.CursorX, got.CursorY, got.CursorVisible)
	}
	text := readText(got)
	if !strings.Contains(text, "snow-read-marker") {
		t.Fatalf("screen does not hold the marker:\n%s", text)
	}
	if strings.Contains(text, "\x1b") {
		t.Fatalf("screen keeps styles:\n%q", text)
	}
}

// TestReadPaneScrollback: lines that scrolled off the screen come back in
// order, plain, with the older ones counted.
func TestReadPaneScrollback(t *testing.T) {
	s := newTestServer(t)
	c, pane := attached(t, s, 100, 30)
	printNumbers(c, pane, 100)

	tool := connect(t, s, 80, 24)
	tool.Send(&api.ReadPaneRequest{Caller: pane, Pane: pane, Scrollback: 10})
	got, e := apitest.Reply[*api.ReadPane](tool)
	if e != nil {
		t.Fatalf("ReadPaneRequest: %v", e)
	}
	if len(got.Scrollback) != 10 {
		t.Fatalf("%d scrollback lines, want 10", len(got.Scrollback))
	}
	if got.Omitted == 0 {
		t.Fatalf("omitted 0 lines; 100 printed lines fill the scrollback")
	}
	if strings.Contains(readText(got), "\x1b") {
		t.Fatalf("the copy keeps styles:\n%q", readText(got))
	}
	first := numberAt(t, got.Scrollback[0])
	for i := 1; i < len(got.Scrollback); i++ {
		if n := numberAt(t, got.Scrollback[i]); n != first+i {
			t.Fatalf("scrollback line %d is %d, want %d: oldest first", i, n, first+i)
		}
	}
	// The scrollback ends just above the screen, so the screen's last
	// number is above the returned ones.
	if last := lastNumber(t, got.Screen); last <= first {
		t.Fatalf("screen's last number %d is not above the scrollback's first %d", last, first)
	}
}

// numberAt returns the number a printed line holds.
func numberAt(t *testing.T, line string) int {
	t.Helper()
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("not a numbered line: %q", line)
	}
	return n
}

// lastNumber returns the greatest number in lines, or 0.
func lastNumber(t *testing.T, lines []string) int {
	t.Helper()
	last := 0
	for _, line := range lines {
		if n, err := strconv.Atoi(strings.TrimSpace(line)); err == nil && n > last {
			last = n
		}
	}
	return last
}

// TestReadPaneErrors: a read_pane call refuses a pane that is not there, a
// chat pane, and a pane of another session, each with its code.
func TestReadPaneErrors(t *testing.T) {
	s, _, _ := newChatServer(t)
	a, first := sessionWith(t, s, "a")
	a.Send(&api.Command{Op: api.OpSplit, Target: first, Args: map[string]string{api.ArgDir: api.DirRight, api.ArgKind: api.PaneChat}})
	chat := chatIn(nextLayout(t, a, func(l *api.Layout) bool { return chatIn(l) != 0 }))
	_, other := sessionWith(t, s, "b")

	tool := connect(t, s, 80, 24)
	tests := []struct {
		name string
		req  *api.ReadPaneRequest
		code string
	}{
		{"missing pane", &api.ReadPaneRequest{Caller: first, Pane: 999}, api.CodeNoPane},
		{"chat pane", &api.ReadPaneRequest{Caller: first, Pane: chat}, api.CodeNotTerminal},
		{"other session", &api.ReadPaneRequest{Caller: first, Pane: other}, api.CodeOtherSession},
		{"unknown caller", &api.ReadPaneRequest{Caller: 999, Pane: first}, api.CodeNoPane},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool.Send(tt.req)
			if _, e := apitest.Reply[*api.ReadPane](tool); e == nil || e.Code != tt.code {
				t.Fatalf("read = %v, want %s", e, tt.code)
			}
		})
	}

	tool.Send(&api.ReadPaneRequest{Caller: first, Pane: first, Scrollback: -1})
	if _, e := apitest.Reply[*api.ReadPane](tool); e == nil || e.Code != api.CodeBadCommand {
		t.Fatalf("negative scrollback = %v, want %s", e, api.CodeBadCommand)
	}
}

// TestFitReadTrimsOldest: a copy over the byte cap loses its oldest lines
// first, the scrollback before the screen, and keeps its newest text.
func TestFitReadTrimsOldest(t *testing.T) {
	line := strings.Repeat("x", 999)
	screen := make([]string, 24)
	for i := range screen {
		screen[i] = line
	}
	back := make([]string, 60)
	for i := range back {
		back[i] = line
	}
	gotScreen, gotBack, omitted := fitRead(screen, back, 0)
	if omitted == 0 {
		t.Fatal("no lines were left out")
	}
	size := 0
	for _, l := range gotScreen {
		size += len(l) + 1
	}
	for _, l := range gotBack {
		size += len(l) + 1
	}
	if size > readPaneMaxText {
		t.Fatalf("kept %d bytes, cap is %d", size, readPaneMaxText)
	}
	if len(gotScreen) == 0 {
		t.Fatal("no screen row was kept")
	}
	if len(gotScreen)+len(gotBack) < 1 || omitted+len(gotScreen)+len(gotBack) != len(screen)+len(back) {
		t.Fatalf("kept %d + %d lines and left out %d, want %d lines in all", len(gotScreen), len(gotBack), omitted, len(screen)+len(back))
	}
}

// TestReadPaneDoesNotBlockOwner: while a pane floods its screen, a read_pane
// call and another client's request both finish quickly, and the output
// still reaches the screen, because the owner goroutine only looks the pane
// up and the copy is taken outside it.
func TestReadPaneDoesNotBlockOwner(t *testing.T) {
	s := newTestServer(t)
	c, pane := attached(t, s, 100, 30)
	// The race detector slows the shell pipeline many times over, so the
	// flood is shorter there.
	n := 5000
	if raceDetector {
		n = 1000
	}
	flood := fmt.Sprintf("seq 1 %d\r", n)
	if runtime.GOOS == "windows" {
		flood = fmt.Sprintf("for /l %%i in (1,1,%d) do @echo %%i\r", n)
	}
	c.Send(&api.Input{Pane: pane, Paste: flood})

	tool := connect(t, s, 80, 24)
	other := connect(t, s, 80, 24)
	start := time.Now()
	tool.Send(&api.ReadPaneRequest{Caller: pane, Pane: pane, Scrollback: 500})
	other.Send(&api.PanesRequest{Caller: pane})

	if _, e := apitest.Reply[*api.ReadPane](tool); e != nil {
		t.Fatalf("read during a flood: %v", e)
	}
	if _, e := apitest.Reply[*api.Panes](other); e != nil {
		t.Fatalf("list during a flood: %v", e)
	}
	if took := time.Since(start); took > 5*time.Second && !raceDetector {
		t.Fatalf("requests during a flood took %v", took)
	}
	c.WaitScreen("\n" + strconv.Itoa(n) + "\n")
}
