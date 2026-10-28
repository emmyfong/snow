package server

import (
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/pkg/api"
)

// paneDriven reports whether the layout marks pane id as driven.
func paneDriven(l *api.Layout, id int) bool {
	for _, w := range l.Windows {
		for _, p := range w.Panes {
			if p.ID == id {
				return p.Driven
			}
		}
	}
	return false
}

// windowWith returns the window that holds pane id, or nil.
func windowWith(l *api.Layout, id int) *api.WindowInfo {
	for i := range l.Windows {
		for _, p := range l.Windows[i].Panes {
			if p.ID == id {
				return &l.Windows[i]
			}
		}
	}
	return nil
}

// TestSendKeysTypes: text is pasted first, then the keys run, so a command
// typed without its newline waits for the key that sends it.
func TestSendKeysTypes(t *testing.T) {
	s := newTestServer(t)
	_, first, second := splitTwo(t, s)

	tool := connect(t, s, 80, 24)
	tool.Send(&api.SendKeysRequest{Caller: first, Pane: second, Text: "echo snow-send-marker"})
	if got, e := apitest.Reply[*api.SendKeys](tool); e != nil {
		t.Fatalf("SendKeysRequest with text: %v", e)
	} else if got.Pane != second || got.Dropped != 0 {
		t.Fatalf("send = %+v; want pane %d and nothing dropped", got, second)
	}
	tool.Send(&api.SendKeysRequest{Caller: first, Pane: second, Keys: []api.Key{{Code: api.KeyEnter}}})
	if _, e := apitest.Reply[*api.SendKeys](tool); e != nil {
		t.Fatalf("SendKeysRequest with keys: %v", e)
	}
	// Two copies: the echoed command and its output.
	eventually(t, "the typed command's output", func() bool {
		return strings.Count(paneText(s, "work", second), "snow-send-marker") >= 2
	})
}

// TestSendKeysMarksDriven: while an agent types, the pane and its window
// carry the driven mark, and the mark clears when its time ends.
func TestSendKeysMarksDriven(t *testing.T) {
	opts := testOptions()
	due := make(chan func(), 4)
	opts.afterDriven = func(f func()) { due <- f }
	s := New(opts)
	t.Cleanup(s.Close)
	c, first, second := splitTwo(t, s)

	tool := connect(t, s, 80, 24)
	tool.Send(&api.SendKeysRequest{Caller: first, Pane: second, Text: "echo driven-ok", Keys: []api.Key{{Code: api.KeyEnter}}})
	if _, e := apitest.Reply[*api.SendKeys](tool); e != nil {
		t.Fatalf("SendKeysRequest: %v", e)
	}
	marked := nextLayout(t, c, func(l *api.Layout) bool { return paneDriven(l, second) })
	if paneDriven(marked, first) {
		t.Fatal("the caller's pane shows the driven mark too")
	}
	if w := windowWith(marked, second); w == nil || !w.Driven {
		t.Fatalf("window %+v: want the driven mark on the window too", w)
	}

	var endMark func()
	select {
	case endMark = <-due:
	case <-time.After(5 * time.Second):
		t.Fatal("the driven mark scheduled no end")
	}
	endMark()
	nextLayout(t, c, func(l *api.Layout) bool { return !paneDriven(l, second) })
}

// TestSendKeysErrors: a send_keys call refuses the caller's own pane,
// another agent's pane, a chat pane, a pane of another session, a pane that
// is not there, and an ended pane, each with its code.
func TestSendKeysErrors(t *testing.T) {
	s, _, _ := newChatServer(t)
	a, first := sessionWith(t, s, "a")
	a.Send(split(first, api.DirRight))
	second := shownPanes(nextLayout(t, a, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 }))[1].ID
	a.Send(&api.AgentState{Pane: second, State: api.AgentWorking})
	nextLayout(t, a, func(l *api.Layout) bool {
		for _, p := range shownPanes(l) {
			if p.ID == second {
				return p.AgentState == api.AgentWorking
			}
		}
		return false
	})
	a.Send(&api.Command{Op: api.OpSplit, Target: second, Args: map[string]string{api.ArgDir: api.DirRight, api.ArgKind: api.PaneChat}})
	chat := chatIn(nextLayout(t, a, func(l *api.Layout) bool { return chatIn(l) != 0 }))
	_, other := sessionWith(t, s, "b")

	tool := connect(t, s, 80, 24)
	tests := []struct {
		name string
		req  *api.SendKeysRequest
		code string
	}{
		{"own pane", &api.SendKeysRequest{Caller: first, Pane: first, Text: "ls"}, api.CodeOwnPane},
		{"agent pane", &api.SendKeysRequest{Caller: first, Pane: second, Text: "ls"}, api.CodeAgentPane},
		{"chat pane", &api.SendKeysRequest{Caller: first, Pane: chat, Text: "ls"}, api.CodeNotTerminal},
		{"other session", &api.SendKeysRequest{Caller: first, Pane: other, Text: "ls"}, api.CodeOtherSession},
		{"missing pane", &api.SendKeysRequest{Caller: first, Pane: 999, Text: "ls"}, api.CodeNoPane},
		{"no input", &api.SendKeysRequest{Caller: first, Pane: first}, api.CodeBadCommand},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool.Send(tt.req)
			if _, e := apitest.Reply[*api.SendKeys](tool); e == nil || e.Code != tt.code {
				t.Fatalf("send = %v, want %s", e, tt.code)
			}
		})
	}
}

// TestSendKeysEndedPane: an ended pane waits for the user to restore it, so
// send_keys refuses it instead of restarting it.
func TestSendKeysEndedPane(t *testing.T) {
	s := wslServer(t, false)
	c, first, second := splitTwo(t, s)
	c.Send(&api.Input{Pane: second, Paste: "exit\r"})
	eventually(t, "the ended note", func() bool {
		return strings.Contains(paneText(s, "work", second), "press r to restore")
	})

	tool := connect(t, s, 80, 24)
	tool.Send(&api.SendKeysRequest{Caller: first, Pane: second, Text: "ls"})
	if _, e := apitest.Reply[*api.SendKeys](tool); e == nil || e.Code != api.CodePaneEnded {
		t.Fatalf("send to an ended pane = %v, want %s", e, api.CodePaneEnded)
	}
}
