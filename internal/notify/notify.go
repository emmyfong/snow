// Package notify shows a desktop notification when an agent needs the user
// or finishes a turn. It starts the platform's helper program in its own
// goroutine with a timeout, so the server never waits on it.
package notify

import (
	"context"
	"log/slog"
	"os/exec"
	"sync"
	"time"
)

// Kind is why a notification shows.
type Kind int

// Notification kinds.
const (
	NeedsYou Kind = iota // a new item in the needs-you inbox
	TurnEnd              // an agent finished a turn
)

// String names k, for logs and tests.
func (k Kind) String() string {
	switch k {
	case NeedsYou:
		return "needs-you"
	case TurnEnd:
		return "turn-end"
	}
	return "unknown"
}

// Notice is what a notification says: the kind, the agent, and its session.
// It never holds a question, a command, or a path, because a notification
// can show on a locked screen.
type Notice struct {
	Kind    Kind
	Agent   string // "Claude Code" or "Snow chat"
	Session string
}

// Options configures a Notifier.
type Options struct {
	NeedsYou bool // notify when an agent starts to need the user
	TurnEnd  bool // notify when an agent finishes a turn
	Log      *slog.Logger
	// Helper builds the helper process for a notice, bounded by ctx. Nil
	// chooses the platform's helper. Tests replace it with the test binary.
	Helper func(ctx context.Context, n Notice) *exec.Cmd
}

// Notifier starts the desktop helper for the events the config turns on. It
// is safe to use from the server's owner goroutine: Notify only builds a
// command and starts it in another goroutine.
type Notifier struct {
	opts Options
	once sync.Once // logs the first failure only
}

// New returns a Notifier for opts.
func New(opts Options) *Notifier { return &Notifier{opts: opts} }

// runTimeout bounds a helper, so a stuck program never keeps a process
// around.
const runTimeout = 5 * time.Second

// Notify shows n on the desktop unless its event is off. It returns at
// once; the helper runs in its own goroutine.
func (n *Notifier) Notify(notice Notice) {
	if notice.Kind == NeedsYou && !n.opts.NeedsYou || notice.Kind == TurnEnd && !n.opts.TurnEnd {
		return
	}
	helper := n.opts.Helper
	if helper == nil {
		helper = command
	}
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	cmd := helper(ctx, notice)
	go func() {
		defer cancel()
		if err := cmd.Run(); err != nil {
			n.once.Do(func() { n.log().Info("desktop notification failed", "err", err) })
		}
	}()
}

// log returns the notifier's logger, or one that discards.
func (n *Notifier) log() *slog.Logger {
	if n.opts.Log != nil {
		return n.opts.Log
	}
	return slog.New(slog.DiscardHandler)
}

// message returns the title and body of a notice. The body names the agent
// and the session and nothing else.
func message(n Notice) (title, body string) {
	switch n.Kind {
	case TurnEnd:
		return "Snow", n.Agent + " in session \"" + n.Session + "\" finished"
	}
	return "Snow", n.Agent + " in session \"" + n.Session + "\" needs you"
}
