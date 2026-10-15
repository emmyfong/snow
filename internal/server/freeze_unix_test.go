//go:build !windows

package server

import (
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// TestPasteIntoBlockedProgramDoesNotFreeze: a program in raw mode that never
// reads its input must not stall the server for everyone else.
func TestPasteIntoBlockedProgramDoesNotFreeze(t *testing.T) {
	opts := testOptions()
	opts.Spec = func(string) (term.Spec, string) {
		return term.Spec{Path: "/bin/sh", Args: []string{"-c", "stty raw -echo; echo ready; sleep 600"}}, "stuck"
	}
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 80, 24)
	c.Send(&api.Attach{Session: "stuck", Create: true})
	c.WaitScreen("ready")
	c.Send(&api.Input{Pane: 1, Paste: strings.Repeat("x", 512*1024)})
	time.Sleep(200 * time.Millisecond)

	answered := make(chan struct{})
	go func() {
		_ = s.Sessions()
		other := connect(t, s, 80, 24)
		other.Send(&api.Attach{Session: "stuck"})
		other.WaitScreen("ready")
		close(answered)
	}()
	select {
	case <-answered:
	case <-time.After(5 * time.Second):
		t.Fatal("the server froze after a paste into a program that does not read")
	}
}
