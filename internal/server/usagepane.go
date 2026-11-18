package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/usage"
	"github.com/emmyfong/snow/pkg/api"
)

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
	w := &convWorker[api.UsageState]{
		store:   ctx.Conversations,
		conv:    ctx.Conversation,
		folder:  ctx.Folder,
		profile: ctx.Profile,
		apply:   func(st api.UsageState) { ctx.Apply(func() { p.take(st) }) },
		build: func(loaded *convstore.Conversation) api.UsageState {
			st := usage.Account(loaded)
			st.Folder = ctx.Folder
			return st
		},
		noConv: func(st api.UsageState) api.UsageState {
			st.Folder, st.Conversation, st.Loading, st.Error = ctx.Folder, "", false, ""
			st.Notes = []string{"No conversation yet. Send a message in a chat pane."}
			return st
		},
		fail: func(st api.UsageState, err error) api.UsageState {
			st.Loading, st.Error = false, err.Error()
			return st
		},
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
