// Package server owns Snow's sessions, windows, and panes. One model
// goroutine owns all state; other goroutines send it closures over a channel,
// so no lock guards the session maps.
package server

import (
	"errors"
	"github.com/emmyfong/snow/internal/store"
	"log/slog"
	"sync"
	"time"

	"github.com/emmyfong/snow/pkg/api"
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
	launch launcher
	outbox int // per-client queue length; tests lower it

	requests chan func(*state)
	quit     chan struct{}
	stopped  chan struct{}
	done     chan struct{} // closed when the last session ends
	once     sync.Once
	closing  sync.WaitGroup // panes still being closed in the background
	saver    *store.Saver   // nil when sessions are not saved
	store    *store.Store   // where saver writes; nil with it
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
	s.launch = newLauncher(opts, s.log)
	s.saver = s.openSaver()
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
	busy := time.NewTicker(cmpDuration(s.opts.busyEvery, defaultBusyEvery))
	defer busy.Stop()
	for {
		select {
		case f := <-s.requests:
			f(st)
		case <-tick.C:
			st.flush()
		case <-busy.C:
			st.saveBusy()
		case <-s.quit:
			st.saveAll()
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

// Done is closed when no session runs and no client is connected, once
// either has existed, so the process can exit.
func (s *Server) Done() <-chan struct{} { return s.done }

// Close stops the model goroutine and ends every pane.
func (s *Server) Close() {
	s.once.Do(func() { close(s.quit) })
	<-s.stopped
	// Wait for panes closed in the background, so no program outlives the
	// server.
	s.closing.Wait()
}

// CreateSession starts a session in the user's home folder and returns its
// name. An empty name takes the lowest free number.
func (s *Server) CreateSession(name string) (string, error) {
	var (
		got string
		err error
	)
	if !s.call(func(st *state) {
		var sess *session
		if sess, err = st.createSession(name, 0, 0, ""); err == nil {
			got = sess.name
			st.sessionsChanged()
		}
	}) {
		return "", ErrClosed
	}
	return got, err
}

// KillSession ends a session and every program in it.
func (s *Server) KillSession(name string) error {
	var err error
	if !s.call(func(st *state) { err = st.killSession(name) }) {
		return ErrClosed
	}
	return err
}

// Sessions lists the running sessions by name.
func (s *Server) Sessions() []api.SessionInfo {
	var out []api.SessionInfo
	s.call(func(st *state) { out = st.sessionList() })
	return out
}

// idle reports whether nothing runs: no session and no connected client.
func (s *Server) idle() bool {
	idle := true
	s.call(func(st *state) { idle = len(st.sessions) == 0 && len(st.clients) == 0 })
	return idle
}
