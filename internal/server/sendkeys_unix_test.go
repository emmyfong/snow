//go:build !windows

package server

import (
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// TestSendKeysReportsDroppedWhenProgramStopsReading: a program in raw mode
// never reads, so the pane's input queue fills; send_keys counts the bytes
// that were dropped, and the owner goroutine still answers.
func TestSendKeysReportsDroppedWhenProgramStopsReading(t *testing.T) {
	if raceDetector {
		t.Skip("filling the input queue is slow under the race detector")
	}
	opts := testOptions()
	plain := opts.Spec
	opts.Spec = func(profile, dir string) (term.Spec, string, error) {
		if profile == "stuck" {
			return term.Spec{Path: "/bin/sh", Args: []string{"-c", "stty raw -echo; echo ready; sleep 600"}}, "stuck", nil
		}
		return plain(profile, dir)
	}
	s := New(opts)
	t.Cleanup(s.Close)
	c, first := attached(t, s, 100, 30)
	c.Send(&api.Command{Op: api.OpSplit, Target: first, Args: map[string]string{
		api.ArgDir: api.DirRight, api.ArgProfile: "stuck",
	}})
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	second := shownPanes(l)[1].ID
	c.WaitScreen("ready")

	// A chunk big enough to fill the PTY buffer and block the pane's writer,
	// then small pastes that fill the queue behind it.
	c.Send(&api.Input{Pane: second, Paste: strings.Repeat("x", 128<<10)})
	for range 300 {
		c.Send(&api.Input{Pane: second, Paste: "y"})
	}

	tool := connect(t, s, 80, 24)
	tool.Send(&api.SendKeysRequest{Caller: first, Pane: second, Text: "typed-by-agent"})
	got, e := apitest.Reply[*api.SendKeys](tool)
	if e != nil {
		t.Fatalf("SendKeysRequest: %v", e)
	}
	if got.Dropped == 0 {
		t.Fatal("Dropped = 0; a program that does not read must have dropped input")
	}

	// The owner goroutine never wrote to the pane, so it still answers.
	other := connect(t, s, 80, 24)
	other.Send(&api.PanesRequest{Caller: first})
	if _, e := apitest.Reply[*api.Panes](other); e != nil {
		t.Fatalf("list while a program does not read: %v", e)
	}
}
