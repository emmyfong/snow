package server

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/pkg/api"
)

// waitReply sends one wait_for request and returns its answer.
func waitReply(t *testing.T, tool *apitest.Client, r *api.WaitForRequest) *api.WaitFor {
	t.Helper()
	tool.Send(r)
	got, e := apitest.Reply[*api.WaitFor](tool)
	if e != nil {
		t.Fatalf("WaitForRequest: %v", e)
	}
	return got
}

// TestWaitForText: a wait returns the line with the text once the pane
// prints it, and not the command line that typed it.
func TestWaitForText(t *testing.T) {
	s := newTestServer(t)
	c, first, second := splitTwo(t, s)
	// Build the marker from a variable, so the typed command does not hold
	// the text itself: only the output can match.
	setup, delay := "", "x=snow-wait-; sleep 0.4; echo ${x}new"
	if runtime.GOOS == "windows" {
		setup, delay = "set x=snow-wait-", "ping -n 2 127.0.0.1 >nul & echo %x%new"
	}
	if setup != "" {
		c.Send(&api.Input{Pane: second, Paste: setup + "\r"})
	}
	c.Send(&api.Input{Pane: second, Paste: delay + "\r"})

	tool := connect(t, s, 80, 24)
	got := waitReply(t, tool, &api.WaitForRequest{Caller: first, Pane: second, Text: "snow-wait-new", TimeoutMS: 5000})
	if got.Pane != second || got.Result != api.WaitFound || !strings.Contains(got.Line, "snow-wait-new") {
		t.Fatalf("wait = %+v, want found with the new marker; pane:\n%s", got, paneText(s, "work", second))
	}
	if strings.Contains(got.Line, "echo") {
		t.Fatalf("found line %q; want the output, not the typed command", got.Line)
	}
}

// TestWaitForIncludeExisting: text already on the pane counts only when the
// request asks for it.
func TestWaitForIncludeExisting(t *testing.T) {
	s := newTestServer(t)
	c, first, second := splitTwo(t, s)
	c.Send(&api.Input{Pane: second, Paste: "echo snow-wait-present\r"})
	// Two copies: the typed command and its output.
	eventually(t, "the marker twice", func() bool {
		return strings.Count(paneText(s, "work", second), "snow-wait-present") >= 2
	})

	tool := connect(t, s, 80, 24)
	got := waitReply(t, tool, &api.WaitForRequest{Caller: first, Pane: second, Text: "snow-wait-present", IncludeExisting: true, TimeoutMS: 2000})
	if got.Result != api.WaitFound || !strings.Contains(got.Line, "snow-wait-present") {
		t.Fatalf("wait with IncludeExisting = %+v, want found", got)
	}

	got = waitReply(t, tool, &api.WaitForRequest{Caller: first, Pane: second, Text: "snow-wait-present", TimeoutMS: 300})
	if got.Result != api.WaitTimeout {
		t.Fatalf("wait without IncludeExisting = %+v, want a timeout", got)
	}
}

// TestWaitForQuiet: a pane that stops changing returns quiet.
func TestWaitForQuiet(t *testing.T) {
	s := newTestServer(t)
	_, first, second := splitTwo(t, s)

	tool := connect(t, s, 80, 24)
	got := waitReply(t, tool, &api.WaitForRequest{Caller: first, Pane: second, QuietMS: 150, TimeoutMS: 5000})
	if got.Pane != second || got.Result != api.WaitQuiet {
		t.Fatalf("quiet wait = %+v, want quiet", got)
	}
}

// TestWaitForTimeout: a timeout is a normal result with the pane named.
func TestWaitForTimeout(t *testing.T) {
	s := newTestServer(t)
	_, first, second := splitTwo(t, s)

	tool := connect(t, s, 80, 24)
	start := time.Now()
	got := waitReply(t, tool, &api.WaitForRequest{Caller: first, Pane: second, Text: "snow-never-printed", TimeoutMS: 300})
	if got.Pane != second || got.Result != api.WaitTimeout {
		t.Fatalf("wait = %+v, want a timeout for pane %d", got, second)
	}
	if took := time.Since(start); took < 250*time.Millisecond {
		t.Fatalf("the wait took %v, before its 300 ms limit", took)
	}
}

// TestWaitForExit: a program that exits returns its status, even when the
// pane is removed at once.
func TestWaitForExit(t *testing.T) {
	s := newTestServer(t)
	c, first, second := splitTwo(t, s)

	tool := connect(t, s, 80, 24)
	tool.Send(&api.WaitForRequest{Caller: first, Pane: second, Text: "snow-never-printed", TimeoutMS: 10000})
	// Let the wait find the pane before the program ends.
	time.Sleep(200 * time.Millisecond)
	c.Send(&api.Input{Pane: second, Paste: "exit 4\r"})

	got, e := apitest.Reply[*api.WaitFor](tool)
	if e != nil {
		t.Fatalf("the wait's answer: %v", e)
	}
	if got.Result != api.WaitExited {
		t.Fatalf("wait = %+v, want exited", got)
	}
	if got.Exit == nil || *got.Exit != 4 {
		t.Fatalf("exit status = %v, want 4", got.Exit)
	}
}

// TestWaitForEndedPane: a WSL pane that already ended reports exited at
// once instead of waiting for its text.
func TestWaitForEndedPane(t *testing.T) {
	s := wslServer(t, false)
	c, first, second := splitTwo(t, s)
	c.Send(&api.Input{Pane: second, Paste: "exit\r"})
	eventually(t, "the ended note", func() bool {
		return strings.Contains(paneText(s, "work", second), "press r to restore")
	})

	tool := connect(t, s, 80, 24)
	start := time.Now()
	got := waitReply(t, tool, &api.WaitForRequest{Caller: first, Pane: second, Text: "never", TimeoutMS: 10000})
	if got.Result != api.WaitExited {
		t.Fatalf("wait on an ended pane = %+v, want exited", got)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("a wait on an ended pane took %v", took)
	}
}

// TestWaitForErrors: a wait refuses the caller's own pane, a chat pane, a
// pane of another session, a pane that is not there, and a bad request.
func TestWaitForErrors(t *testing.T) {
	s, _, _ := newChatServer(t)
	a, first := sessionWith(t, s, "a")
	a.Send(split(first, api.DirRight))
	second := shownPanes(nextLayout(t, a, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 }))[1].ID
	a.Send(&api.Command{Op: api.OpSplit, Target: second, Args: map[string]string{api.ArgDir: api.DirRight, api.ArgKind: api.PaneChat}})
	chat := chatIn(nextLayout(t, a, func(l *api.Layout) bool { return chatIn(l) != 0 }))
	_, other := sessionWith(t, s, "b")

	tool := connect(t, s, 80, 24)
	tests := []struct {
		name string
		req  *api.WaitForRequest
		code string
	}{
		{"own pane", &api.WaitForRequest{Caller: first, Pane: first, Text: "ok"}, api.CodeOwnPane},
		{"chat pane", &api.WaitForRequest{Caller: first, Pane: chat, Text: "ok"}, api.CodeNotTerminal},
		{"other session", &api.WaitForRequest{Caller: first, Pane: other, Text: "ok"}, api.CodeOtherSession},
		{"missing pane", &api.WaitForRequest{Caller: first, Pane: 999, Text: "ok"}, api.CodeNoPane},
		{"unknown caller", &api.WaitForRequest{Caller: 999, Pane: second, Text: "ok"}, api.CodeNoPane},
		{"no condition", &api.WaitForRequest{Caller: first, Pane: second}, api.CodeBadCommand},
		{"two conditions", &api.WaitForRequest{Caller: first, Pane: second, Text: "ok", QuietMS: 10}, api.CodeBadCommand},
		{"negative timeout", &api.WaitForRequest{Caller: first, Pane: second, Text: "ok", TimeoutMS: -1}, api.CodeBadCommand},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool.Send(tt.req)
			if _, e := apitest.Reply[*api.WaitFor](tool); e == nil || e.Code != tt.code {
				t.Fatalf("wait = %v, want %s", e, tt.code)
			}
		})
	}
}

// TestWaitForDoesNotBlockOwner: a wait runs outside the model goroutine, so
// other clients still get answers while it waits.
func TestWaitForDoesNotBlockOwner(t *testing.T) {
	s := newTestServer(t)
	_, first, second := splitTwo(t, s)

	tool := connect(t, s, 80, 24)
	tool.Send(&api.WaitForRequest{Caller: first, Pane: second, Text: "snow-never-printed", TimeoutMS: 1500})
	time.Sleep(100 * time.Millisecond)

	other := connect(t, s, 80, 24)
	start := time.Now()
	other.Send(&api.PanesRequest{Caller: first})
	if _, e := apitest.Reply[*api.Panes](other); e != nil {
		t.Fatalf("list during a wait: %v", e)
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("a request during a wait took %v", took)
	}
	got, e := apitest.Reply[*api.WaitFor](tool)
	if e != nil {
		t.Fatalf("the wait's answer: %v", e)
	}
	if got.Result != api.WaitTimeout {
		t.Fatalf("wait = %+v, want a timeout", got)
	}
}

// TestWaitForInterrupt: closing the tool's connection stops its wait at
// once and leaves no goroutine behind.
func TestWaitForInterrupt(t *testing.T) {
	opts := testOptions()
	ended := make(chan struct{}, 1)
	opts.afterWait = func() {
		select {
		case ended <- struct{}{}:
		default:
		}
	}
	s := New(opts)
	t.Cleanup(s.Close)
	_, first, second := splitTwo(t, s)

	tool := connect(t, s, 80, 24)
	tool.Send(&api.WaitForRequest{Caller: first, Pane: second, Text: "snow-never-printed", TimeoutMS: 60000})
	time.Sleep(200 * time.Millisecond)
	tool.Close()

	select {
	case <-ended:
	case <-time.After(3 * time.Second):
		t.Fatal("the wait kept running after its connection closed")
	}
}

// TestWaitLimit: a request gets the default timeout, and anything over the
// cap is clamped to it.
func TestWaitLimit(t *testing.T) {
	tests := []struct {
		ms   int
		want time.Duration
	}{
		{0, 30 * time.Second},
		{250, 250 * time.Millisecond},
		{int((10 * time.Minute) / time.Millisecond), 10 * time.Minute},
		{int((24 * time.Hour) / time.Millisecond), 10 * time.Minute},
		{1 << 40, 10 * time.Minute},
	}
	for _, tt := range tests {
		if got := waitLimit(tt.ms); got != tt.want {
			t.Errorf("waitLimit(%d) = %v, want %v", tt.ms, got, tt.want)
		}
		if tt.ms <= 0 {
			continue
		}
		if got := quietPeriod(tt.ms); got != tt.want {
			t.Errorf("quietPeriod(%d) = %v, want %v", tt.ms, got, tt.want)
		}
	}
}
