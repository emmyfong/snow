package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/usage"
	"github.com/emmyfong/snow/pkg/api"
)

// contextKind is the context pane: the reported context occupancy and the
// conversation's observable sources.
func contextKind() sideKind {
	return sideKind{Name: api.PaneContext, Title: "Context", Open: openContext}
}

// contextPane is the model goroutine's half of a context pane. Its worker
// reads the conversation's records and hands each new state over through
// Apply.
type contextPane struct {
	state api.ContextState
	conv  string
	off   bool // chat is off, so the pane has no store to read
	jobs  chan struct{}
	done  chan struct{}
	stop  context.CancelFunc
}

func openContext(ctx sideContext) (sidePane, error) {
	if ctx.Conversations == nil {
		return &contextPane{
			state: api.ContextState{Folder: ctx.Folder, Error: chatOff},
			conv:  ctx.Conversation,
			off:   true,
			done:  closedChan(),
			stop:  func() {},
		}, nil
	}
	runCtx, cancel := context.WithCancel(context.Background())
	p := &contextPane{
		state: api.ContextState{Folder: ctx.Folder, Conversation: ctx.Conversation, Loading: true},
		conv:  ctx.Conversation,
		jobs:  make(chan struct{}, 1),
		done:  make(chan struct{}),
		stop:  cancel,
	}
	w := &convWorker[api.ContextState]{
		store:   ctx.Conversations,
		conv:    ctx.Conversation,
		folder:  ctx.Folder,
		profile: ctx.Profile,
		apply:   func(st api.ContextState) { ctx.Apply(func() { p.take(st) }) },
		build: func(loaded *convstore.Conversation) api.ContextState {
			st := usage.Context(loaded)
			st.Folder = ctx.Folder
			return st
		},
		noConv: func(st api.ContextState) api.ContextState {
			st.Folder, st.Conversation, st.Loading, st.Error = ctx.Folder, "", false, ""
			st.Notes = []string{"No conversation yet. Send a message in a chat pane."}
			return st
		},
		fail: func(st api.ContextState, err error) api.ContextState {
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
func (p *contextPane) take(st api.ContextState) {
	p.state, p.conv = st, st.Conversation
}

// State implements sidePane.
func (p *contextPane) State() api.SideState {
	st := p.state
	return &st
}

// Act implements sidePane. The only action is a fresh read.
func (p *contextPane) Act(action string, _ map[string]string) error {
	if action != api.ContextActRefresh {
		return fmt.Errorf("the context pane has no action %q", action)
	}
	if p.off {
		return errors.New("the context pane needs chat, and chat is off")
	}
	select {
	case p.jobs <- struct{}{}:
	default:
		// A read is already waiting; it sees the same file.
	}
	return nil
}

// Close implements sidePane. It stops the reads and waits for the worker.
func (p *contextPane) Close() error {
	p.stop()
	<-p.done
	return nil
}

// Conversation implements conversationSide: the conversation the pane
// follows, for a saved session.
func (p *contextPane) Conversation() string { return p.conv }
