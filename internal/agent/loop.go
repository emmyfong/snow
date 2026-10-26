package agent

import (
	"context"
	"fmt"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/provider"
)

// Commands posted to the loop.
type (
	sendCmd   struct{ text string }
	switchCmd struct{ sw *convstore.ModelSwitch }
	modeCmd   struct{ mode Mode }
	renameCmd struct{ title string }
	waitCmd   struct {
		req   *ApprovalRequest
		reply chan<- Answer
	}
	answerCmd struct {
		a   Answer
		res chan<- error
	}
	pendingCmd   struct{ res chan<- *ApprovalRequest }
	retryCmd     struct{ res chan<- error }
	interruptCmd struct{}
	closeCmd     struct{}
	savedCmd     struct{ err error } // the save in flight is done
)

// save is a record the loop writes off its goroutine, then applies.
type save struct {
	rec convstore.Record
	// holdsTurns keeps the next turn from starting until the record is
	// saved and applied: a model switch must be in force before the turn
	// that uses it.
	holdsTurns bool
	done       func(err error) // runs on the loop's goroutine
}

// loopState is what the loop owns. Nothing else touches it.
type loopState struct {
	queue   []string               // messages for the next turns
	pending *convstore.ModelSwitch // applies when the running turn ends
	waiting *waitCmd               // the approval the turn waits for
	cancel  context.CancelFunc     // of the running turn; nil when idle
	ended   chan struct{}          // the running turn is over
	turn    int
	saves   []save // to write, in order; the first is in flight when saving
	saving  bool
	closing bool
	shown   bool // a QueueChanged event announced the queue
}

// loop owns the queue, the approval that waits, and the saves, and starts
// turns one at a time. It never blocks on the disk: saves run on their own
// goroutine, one at a time, in order, and post back when done.
func (c *Conversation) loop() {
	defer close(c.done)
	l := &loopState{ended: make(chan struct{})}
	for {
		select {
		case cmd := <-c.cmds:
			c.handle(l, cmd)
		case <-l.ended:
			l.cancel()
			l.cancel, l.waiting = nil, nil
			if l.pending != nil {
				c.queueSwitch(l, l.pending)
				l.pending = nil
			}
			if len(l.queue) == 0 && !l.closing {
				c.emit(&StateChanged{State: StateIdle})
			}
			c.next(l)
		}
		if l.closing && l.cancel == nil && len(l.saves) == 0 {
			return
		}
	}
}

func (c *Conversation) handle(l *loopState, cmd any) {
	switch cmd := cmd.(type) {
	case sendCmd:
		l.queue = append(l.queue, cmd.text)
		if l.cancel != nil || l.holding() {
			l.shown = true
			c.emit(&QueueChanged{Messages: append([]string(nil), l.queue...)})
		}
		c.next(l)
	case switchCmd:
		if l.cancel == nil {
			c.queueSwitch(l, cmd.sw)
		} else {
			l.pending = cmd.sw
		}
	case modeCmd:
		m := cmd.mode
		c.queueSave(l, save{rec: &convstore.Mode{At: c.now(), Mode: string(m)}, done: func(err error) {
			if err != nil {
				c.emit(&ModeChanged{Mode: c.Mode(), Failure: saveFailure(err)})
				return
			}
			c.mode.Store(m)
			c.emit(&ModeChanged{Mode: m})
		}})
	case renameCmd:
		t := cmd.title
		c.queueSave(l, save{rec: &convstore.Rename{At: c.now(), Title: t}, done: func(err error) {
			if err != nil {
				c.emit(&TitleChanged{Failure: saveFailure(err)})
				return
			}
			c.emit(&TitleChanged{Title: t})
		}})
	case waitCmd:
		l.waiting = &cmd
	case answerCmd:
		if l.waiting == nil || l.waiting.req.ID != cmd.a.ID {
			cmd.res <- fmt.Errorf("answer %q: %w", cmd.a.ID, ErrNoApproval)
			return
		}
		l.waiting.reply <- cmd.a
		l.waiting = nil
		cmd.res <- nil
	case pendingCmd:
		if l.waiting != nil {
			cmd.res <- l.waiting.req
		} else {
			cmd.res <- nil
		}
	case retryCmd:
		if l.cancel != nil || l.closing || l.holding() || len(l.queue) > 0 {
			cmd.res <- ErrBusy
			return
		}
		c.begin(l, "", true)
		cmd.res <- nil
	case interruptCmd:
		if l.cancel != nil {
			l.cancel()
		}
		// The turn stops waiting; an answer that comes later is refused.
		l.waiting = nil
	case closeCmd:
		l.closing = true
		l.queue = nil
		if l.cancel != nil {
			l.cancel()
		}
		l.waiting = nil
	case savedCmd:
		s := l.saves[0]
		l.saves, l.saving = l.saves[1:], false
		s.done(cmd.err)
		c.startSave(l)
		c.next(l)
	}
}

// holding reports whether a save must finish before the next turn starts.
func (l *loopState) holding() bool {
	for _, s := range l.saves {
		if s.holdsTurns {
			return true
		}
	}
	return false
}

// next starts the next queued turn when nothing holds it back.
func (c *Conversation) next(l *loopState) {
	if l.cancel != nil || l.closing || l.holding() || len(l.queue) == 0 {
		return
	}
	text := l.queue[0]
	l.queue = l.queue[1:]
	if l.shown {
		l.shown = len(l.queue) > 0
		c.emit(&QueueChanged{Messages: append([]string(nil), l.queue...)})
	}
	c.begin(l, text, false)
}

// begin starts a turn on its own goroutine.
func (c *Conversation) begin(l *loopState, text string, retry bool) {
	l.turn++
	ctx, cancel := context.WithCancel(context.Background())
	l.cancel = cancel
	go func(n int) {
		c.runTurn(ctx, n, text, retry)
		l.ended <- struct{}{}
	}(l.turn)
}

// queueSwitch saves sw and then makes it the model of the next turn. The
// model changes on the loop's goroutine while no turn runs: switches are
// queued only between turns, and they hold the next turn until applied.
func (c *Conversation) queueSwitch(l *loopState, sw *convstore.ModelSwitch) {
	sw.At = c.now()
	c.queueSave(l, save{rec: sw, holdsTurns: true, done: func(err error) {
		if err != nil {
			c.emit(&ModelChanged{Model: c.model, Effort: string(c.effort), Failure: saveFailure(err)})
			return
		}
		c.model, c.effort = sw.Model, provider.Effort(sw.Effort)
		c.emit(&ModelChanged{Model: c.model, Effort: sw.Effort})
	}})
}

func (c *Conversation) queueSave(l *loopState, s save) {
	l.saves = append(l.saves, s)
	c.startSave(l)
}

// startSave writes the first queued record on its own goroutine, unless a
// save is in flight. The Writer orders it with the turn's own writes.
func (c *Conversation) startSave(l *loopState) {
	if l.saving || len(l.saves) == 0 {
		return
	}
	l.saving = true
	rec := l.saves[0].rec
	go func() {
		// The loop does not return while a save is in flight, so the post
		// is always received.
		_ = c.post(savedCmd{err: c.w.Append(rec)})
	}()
}
