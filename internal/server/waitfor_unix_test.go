//go:build !windows

package server

import (
	"testing"
	"time"

	"github.com/emmyfong/snow/pkg/api"
)

// TestWaitForQuietResets: output during the quiet period starts the period
// again, so a pane that keeps printing does not look quiet.
func TestWaitForQuietResets(t *testing.T) {
	s := newTestServer(t)
	c, first, second := splitTwo(t, s)
	tool := connect(t, s, 80, 24)
	// Build the marker from a variable, so only the output holds it.
	c.Send(&api.Input{Pane: second, Paste: "x=snow-quiet-; sleep 1; echo ${x}tick\r"})
	c.WaitScreen("sleep 1; echo ${x}tick")

	start := time.Now()
	got := waitReply(t, tool, &api.WaitForRequest{Caller: first, Pane: second, QuietMS: 1500, TimeoutMS: 5000})
	if got.Result != api.WaitQuiet {
		t.Fatalf("quiet wait = %+v, want quiet", got)
	}
	// The output starts the 1500 ms period again, so the wait cannot end
	// before the sleep plus the period.
	if took := time.Since(start); took < 2*time.Second {
		t.Fatalf("quiet returned after %v, before the output reset the period", took)
	}
}
