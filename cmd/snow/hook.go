package main

import (
	"io"
	"os"

	"github.com/emmyfong/snow/internal/claudecode"
	"github.com/emmyfong/snow/internal/transport"
)

// reportHook runs one hook; tests replace it.
var reportHook = func(h claudecode.Hook, in io.Reader) error { return h.Run(in) }

// runHook is snow hook: Claude Code runs it at each hook event in a Snow
// pane, and it tells the server what the agent in that pane does. It
// always exits 0 and prints nothing: Claude Code shows a hook's output,
// and treats exit code 2, which a Go panic gives, as a block.
func runHook(in io.Reader) (code int) {
	defer func() {
		// A panic only means the mark does not change this time.
		if recover() != nil {
			code = 0
		}
	}()
	h := claudecode.Hook{Pane: os.Getenv(transport.EnvPane), Socket: os.Getenv(transport.EnvSocket), Version: version}
	// A failure here only means the mark does not change; there is no one
	// to tell without disturbing Claude Code.
	_ = reportHook(h, in)
	return 0
}
