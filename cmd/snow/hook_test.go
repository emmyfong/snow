package main

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/claudecode"
	"github.com/emmyfong/snow/internal/transport"
	"github.com/emmyfong/snow/pkg/api"
)

// nextPaneAgent reads layouts from c until pane id shows want.
func nextPaneAgent(t *testing.T, c *apitest.Client, id int, want string) {
	t.Helper()
	deadline := time.Now().Add(apitest.Wait)
	for time.Now().Before(deadline) {
		m, err := c.Next(time.Second)
		if err != nil && !errors.Is(err, apitest.ErrTimeout) {
			t.Fatal(err)
		}
		if l, ok := m.(*api.Layout); ok {
			for _, w := range l.Windows {
				for _, p := range w.Panes {
					if p.ID == id && p.AgentState == want {
						return
					}
				}
			}
		}
	}
	t.Fatalf("pane %d never showed agent state %q", id, want)
}

// hookRun runs snow hook with payload on stdin, and checks that it exits 0
// and prints nothing, as a Claude Code hook must.
func hookRun(t *testing.T, payload string) {
	t.Helper()
	old := stdin
	stdin = strings.NewReader(payload)
	t.Cleanup(func() { stdin = old })
	var out, errOut bytes.Buffer
	if code := run([]string{"hook"}, &out, &errOut); code != 0 || out.Len() > 0 || errOut.Len() > 0 {
		t.Fatalf("snow hook: code %d, stdout %q, stderr %q; want 0 and nothing", code, out.String(), errOut.String())
	}
}

// TestHookEvents: each Claude Code event reaches the server as the pane's
// agent state, and snow hook always exits 0 and prints nothing, even on a
// payload it cannot read.
func TestHookEvents(t *testing.T) {
	path := startServer(t)
	c := apitest.Dial(t, path, 80, 24)
	c.Send(&api.Attach{Session: "hooks", Create: true})
	var pane int
	for pane == 0 {
		m, err := c.Next(apitest.Wait)
		if err != nil {
			t.Fatal(err)
		}
		if l, ok := m.(*api.Layout); ok {
			pane = l.Windows[0].Panes[0].ID
		}
	}
	t.Setenv(transport.EnvPane, strconv.Itoa(pane))
	t.Setenv(transport.EnvSocket, path)
	tests := []struct{ payload, want string }{
		{`{"hook_event_name":"SessionStart","source":"startup"}`, api.AgentIdle},
		{`{"hook_event_name":"UserPromptSubmit","prompt":"fix the bug"}`, api.AgentWorking},
		{`{"hook_event_name":"Notification","notification_type":"permission_prompt"}`, api.AgentNeedsYou},
		{`{"hook_event_name":"PostToolUse","tool_name":"Bash"}`, api.AgentWorking},
		{`{"hook_event_name":"Notification"}`, api.AgentNeedsYou},
		{`{"hook_event_name":"PreToolUse","tool_name":"Edit"}`, api.AgentWorking},
		{`{"hook_event_name":"Stop"}`, api.AgentIdle},
		{`{"hook_event_name":"SessionEnd","reason":"exit"}`, ""},
	}
	for _, tt := range tests {
		hookRun(t, tt.payload)
		nextPaneAgent(t, c, pane, tt.want)
	}
	// A broken payload and the reminder after a turn change nothing: a
	// client that attaches later still sees the last state.
	hookRun(t, `{"hook_event_name":"Stop"}`)
	nextPaneAgent(t, c, pane, api.AgentIdle)
	hookRun(t, `{"hook_event_name":`)
	hookRun(t, `{"hook_event_name":"Notification","notification_type":"idle_prompt"}`)
	later := apitest.Dial(t, path, 80, 24)
	later.Send(&api.Attach{Session: "hooks"})
	nextPaneAgent(t, later, pane, api.AgentIdle)
}

// TestHookPanicExitsZero: a panic in snow hook still exits 0 and prints
// nothing; Claude Code treats exit code 2 as a block.
func TestHookPanicExitsZero(t *testing.T) {
	old := reportHook
	reportHook = func(claudecode.Hook, io.Reader) error { panic("boom") }
	t.Cleanup(func() { reportHook = old })
	hookRun(t, `{"hook_event_name":"Stop"}`)
}

// TestHookOutsideSnowExitsZero: outside a Snow pane, and with no server,
// snow hook still exits 0 and prints nothing.
func TestHookOutsideSnowExitsZero(t *testing.T) {
	t.Setenv(transport.EnvPane, "")
	t.Setenv(transport.EnvSocket, "")
	hookRun(t, `{"hook_event_name":"Stop"}`)
	t.Setenv(transport.EnvPane, "3")
	t.Setenv(transport.EnvSocket, apitest.SocketPath(t))
	hookRun(t, `{"hook_event_name":"Stop"}`)
}

func TestParseHook(t *testing.T) {
	if got, err := parseArgs([]string{"hook"}); err != nil || got.kind != kindHook {
		t.Fatalf("parseArgs(hook) = %+v, %v; want kindHook", got, err)
	}
}
