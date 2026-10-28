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
	c.Send(&api.Input{Pane: second, Paste: "sleep 0.2; echo snow-quiet-tick\r"})
	c.WaitScreen("sleep 0.2; echo snow-quiet-tick")

	tool := connect(t, s, 80, 24)
	start := time.Now()
	got := waitReply(t, tool, &api.WaitForRequest{Caller: first, Pane: second, QuietMS: 300, TimeoutMS: 5000})
	if got.Result != api.WaitQuiet {
		t.Fatalf("quiet wait = %+v, want quiet", got)
	}
	// The output starts the 300 ms period again, so the wait cannot end
	// before it plus 300 ms.
	if took := time.Since(start); took < 450*time.Millisecond {
		t.Fatalf("quiet returned after %v, before the output reset the period", took)
	}
}
