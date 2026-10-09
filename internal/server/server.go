// Package server owns Snow's sessions, windows, and panes. One model
// goroutine owns all state; other goroutines send it closures over a channel,
// so no lock guards the session maps.
package server

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/emmyfong/snow/internal/term"
)

// Server errors. Callers match them with errors.Is.
var (
	ErrNoSession     = errors.New("server: no such session")
	ErrSessionExists = errors.New("server: session already exists")
	ErrClosed        = errors.New("server: closed")
)

// Server holds sessions. Create one with New and stop it with Close.
type Server struct {
	opts   Options
	log    *slog.Logger
	spec   func(dir string) (term.Spec, string)
	outbox int // per-client queue length; tests lower it

	requests chan func(*state)
	quit     chan struct{}
	stopped  chan struct{}
	done     chan struct{} // closed when the last session ends
	once     sync.Once
}

// New starts a server's model goroutine. It does not listen on a socket; see
// Serve.
func New(opts Options) *Server {
	s := &Server{
		opts:     opts,
		outbox:   outboxSize,
		log:      opts.logger(),
		requests: make(chan func(*state)),
		quit:     make(chan struct{}),
		stopped:  make(chan struct{}),
		done:     make(chan struct{}),
	}
	s.spec = opts.Spec
	if s.spec == nil {
		s.spec = defaultSpec(s.log)
	}
	go s.loop()
	return s
}

// frameInterval caps pane updates at about 30 per second per client, so a
// flood of output cannot flood the connection.
const frameInterval = 33 * time.Millisecond

func (s *Server) loop() {
	st := newState(s)
	defer close(s.stopped)
	tick := time.NewTicker(frameInterval)
	defer tick.Stop()
	for {
		select {
		case f := <-s.requests:
			f(st)
		case <-tick.C:
			st.flush()
		case <-s.quit:
			st.closeAll()
			return
		}
	}
}

// call runs f in the model goroutine and waits for it. It returns false if
// the server has stopped.
func (s *Server) call(f func(*state)) bool {
	ran := make(chan struct{})
	select {
	case s.requests <- func(st *state) { f(st); close(ran) }:
		<-ran
		return true
	case <-s.quit:
		return false
	}
}

// post runs f in the model goroutine without waiting. It drops f if the
// server has stopped.
func (s *Server) post(f func(*state)) {
	go func() {
		select {
		case s.requests <- f:
		case <-s.quit:
		}
	}()
}

// Done is closed when the last session ends, so the process can exit.
func (s *Server) Done() <-chan struct{} { return s.done }

// Close stops the model goroutine and ends every pane.
func (s *Server) Close() {
	s.once.Do(func() { close(s.quit) })
	<-s.stopped
}
