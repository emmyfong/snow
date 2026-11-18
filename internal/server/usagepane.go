package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/usage"
	"github.com/emmyfong/snow/pkg/api"
)

// conversationPoll is how often a conversation-bound pane reads its
// conversation's file again, so a live turn's usage shows without a key.
const conversationPoll = 2 * time.Second

// usageKind is the usage pane: what a conversation's observable usage adds
// up to, by turn and model.
func usageKind() sideKind {
	return sideKind{Name: api.PaneUsage, Title: "Usage", Open: openUsage}
}

// usagePane is the model goroutine's half of a usage pane. Its worker reads
// the conversation's records and hands each new state over through Apply.
type usagePane struct {
	state api.UsageState
	conv  string
	jobs  chan struct{}
	done  chan struct{}
	stop  context.CancelFunc
}

func openUsage(ctx sideContext) (sidePane, error) {
	if ctx.Conversations == nil {
		return nil, errors.New("the usage pane needs conversations, and chat is off")
	}
	runCtx, cancel := context.WithCancel(context.Background())
	p := &usagePane{
		state: api.UsageState{Folder: ctx.Folder, Conversation: ctx.Conversation, Loading: true},
		conv:  ctx.Conversation,
		jobs:  make(chan struct{}, 1),
		done:  make(chan struct{}),
		stop:  cancel,
	}
	w := &usageWorker{
		store:   ctx.Conversations,
		conv:    ctx.Conversation,
		folder:  ctx.Folder,
		profile: ctx.Profile,
		apply:   func(st api.UsageState) { ctx.Apply(func() { p.take(st) }) },
	}
	go func() {
		defer close(p.done)
		w.run(runCtx, p.jobs)
	}()
	return p, nil
}

// take makes st the pane's state, on the model goroutine.
func (p *usagePane) take(st api.UsageState) {
	p.state, p.conv = st, st.Conversation
}

// State implements sidePane.
func (p *usagePane) State() api.SideState {
	st := p.state
	return &st
}

// Act implements sidePane. The only action is a fresh read.
func (p *usagePane) Act(action string, _ map[string]string) error {
	if action != api.UsageActRefresh {
		return fmt.Errorf("the usage pane has no action %q", action)
	}
	select {
	case p.jobs <- struct{}{}:
	default:
		// A read is already waiting; it sees the same file.
	}
	return nil
}

// Close implements sidePane. It stops the reads and waits for the worker.
func (p *usagePane) Close() error {
	p.stop()
	<-p.done
	return nil
}

// Conversation implements conversationSide: the conversation the pane
// follows, for a saved session.
func (p *usagePane) Conversation() string { return p.conv }

// usageWorker reads a usage pane's conversation on its own goroutine.
type usageWorker struct {
	store   *convstore.Store
	conv    string
	folder  string
	profile string
	cur     api.UsageState
	have    bool
	size    int64
	mod     time.Time
	apply   func(api.UsageState)
}

// run reads the conversation, then again on each tick when the file moved
// on, until ctx ends.
func (w *usageWorker) run(ctx context.Context, jobs <-chan struct{}) {
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

// read builds the pane's state from the conversation file. It stats the
// file first, so an unchanged conversation costs one call, not a parse.
func (w *usageWorker) read(force bool) api.UsageState {
	if w.conv == "" {
		ok, err := w.resolve()
		switch {
		case err != nil:
			return w.fail(err)
		case !ok:
			st := w.cur
			st.Folder, st.Conversation = w.folder, ""
			st.Loading = false
			st.Notes = []string{"No conversation yet. Send a message in a chat pane."}
			return st
		}
	}
	path, err := w.store.Path(w.conv)
	if err != nil {
		return w.fail(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		return w.fail(err)
	}
	if !force && w.have && fi.Size() == w.size && fi.ModTime().Equal(w.mod) {
		return w.cur
	}
	loaded, err := w.store.Load(w.conv)
	if err != nil {
		return w.fail(err)
	}
	st := usage.Account(loaded)
	st.Folder = w.folder
	w.have, w.size, w.mod = true, fi.Size(), fi.ModTime()
	return st
}

// resolve names the newest conversation in the pane's folder and profile,
// for a pane that no chat named.
func (w *usageWorker) resolve() (bool, error) {
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

// fail returns the last state with the error that stopped the read.
func (w *usageWorker) fail(err error) api.UsageState {
	st := w.cur
	st.Folder, st.Conversation = w.folder, w.conv
	st.Loading = false
	st.Error = err.Error()
	return st
}

// publish hands next over when it differs from the last state, or always
// when force is set.
func (w *usageWorker) publish(next api.UsageState, force bool) {
	next.Loading = false
	if !force && reflect.DeepEqual(next, w.cur) {
		return
	}
	w.cur = next
	w.apply(next)
}
