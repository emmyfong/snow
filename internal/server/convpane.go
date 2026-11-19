package server

import (
	"context"
	"os"
	"reflect"
	"time"

	"github.com/emmyfong/snow/internal/convstore"
)

// conversationPoll is how often a conversation-bound pane reads its
// conversation's file again, so a live turn shows without a key.
const conversationPoll = 2 * time.Second

// conversationResolve is how often a pane with no conversation looks for
// one. Listing the store reads every conversation file, so a miss waits
// much longer than a file that is already known.
const conversationResolve = 30 * time.Second

// chatOff is what a conversation pane shows when chat is off, so it has no
// conversation store to read.
const chatOff = "Chat is off, so this pane cannot read conversations."

// convWorker reads one conversation's file on its own goroutine and hands
// each new pane state to apply. It stats the file first, so an unchanged
// conversation costs one call, not a parse. It runs off the model
// goroutine; T is the pane's state type.
type convWorker[T any] struct {
	store   *convstore.Store
	conv    string
	folder  string
	profile string

	cur  T
	have bool
	size int64
	mod  time.Time

	// resolved is when the worker last looked for a conversation, and
	// resolveEvery overrides how long it waits before looking again.
	resolved     time.Time
	resolveEvery time.Duration

	build  func(*convstore.Conversation) T // the state a loaded conversation makes
	noConv func(T) T                       // the state for a pane with no conversation
	fail   func(T, error) T                // the state with an error that stopped the read
	apply  func(T)                         // hands a new state over on the model goroutine
}

// run reads the conversation, then again on each tick when the file moved
// on or on a job, until ctx ends.
func (w *convWorker[T]) run(ctx context.Context, jobs <-chan struct{}) {
	w.publish(w.read(true), true)
	tick := time.NewTicker(conversationPoll)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-jobs:
			w.publish(w.read(true), true)
		case <-tick.C:
			w.publish(w.read(false), false)
		}
	}
}

// read builds the pane's state from the conversation file.
func (w *convWorker[T]) read(force bool) T {
	if w.conv == "" {
		if !force && time.Since(w.resolved) < w.resolveInterval() {
			return w.cur
		}
		ok, err := w.resolve()
		w.resolved = time.Now()
		switch {
		case err != nil:
			return w.fail(w.cur, err)
		case !ok:
			return w.noConv(w.cur)
		}
	}
	path, err := w.store.Path(w.conv)
	if err != nil {
		return w.fail(w.cur, err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		return w.fail(w.cur, err)
	}
	if !force && w.have && fi.Size() == w.size && fi.ModTime().Equal(w.mod) {
		return w.cur
	}
	loaded, err := w.store.Load(w.conv)
	if err != nil {
		return w.fail(w.cur, err)
	}
	w.have, w.size, w.mod = true, fi.Size(), fi.ModTime()
	return w.build(loaded)
}

// resolveInterval is how long a pane with no conversation waits before it
// looks for one again.
func (w *convWorker[T]) resolveInterval() time.Duration {
	if w.resolveEvery > 0 {
		return w.resolveEvery
	}
	return conversationResolve
}

// closedChan returns a channel that is already closed, for a pane that has
// no worker to wait for.
func closedChan() chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

// resolve names the newest conversation in the pane's folder and profile,
// for a pane that no chat named.
func (w *convWorker[T]) resolve() (bool, error) {
	entries, err := w.store.List(w.folder, w.profile)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if e.Err == nil && e.Turns > 0 {
			w.conv = e.ID
			return true, nil
		}
	}
	return false, nil
}

// publish hands next over when it differs from the last state, or always
// when force is set.
func (w *convWorker[T]) publish(next T, force bool) {
	if !force && reflect.DeepEqual(next, w.cur) {
		return
	}
	w.cur = next
	w.apply(next)
}
