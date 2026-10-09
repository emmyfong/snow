package server

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/emmyfong/snow/pkg/api"
)

const defaultCols, defaultRows = 80, 24

// state is everything the model goroutine owns.
type state struct {
	srv      *Server
	sessions map[string]*session
	clients  map[*client]struct{}
	nextPane int
	hadOne   bool // a session has existed; the last one ending closes Done
}

type session struct {
	name       string
	windows    []*window
	lastUsed   time.Time
	cols, rows int
}

type window struct {
	index int
	name  string
	pane  *pane
}

func newState(s *Server) *state {
	return &state{srv: s, sessions: map[string]*session{}, clients: map[*client]struct{}{}, nextPane: 1}
}

// createSession starts a session with one window and one pane in dir. An
// empty name takes the lowest free number: "0", "1", ...
func (st *state) createSession(name string, cols, rows int, dir string) (*session, error) {
	if name == "" {
		name = st.freeNumber()
	}
	if _, ok := st.sessions[name]; ok {
		return nil, fmt.Errorf("%w: %s", ErrSessionExists, name)
	}
	if cols <= 0 || rows <= 0 {
		cols, rows = defaultCols, defaultRows
	}
	p, err := st.srv.startPane(st.nextPane, cols, rows, dir)
	if err != nil {
		return nil, fmt.Errorf("start pane for %s: %w", name, err)
	}
	st.nextPane++
	sess := &session{
		name:     name,
		windows:  []*window{{index: 1, name: p.profile, pane: p}},
		lastUsed: time.Now(),
		cols:     cols,
		rows:     rows,
	}
	st.sessions[name] = sess
	st.hadOne = true
	st.srv.log.Info("session created", "session", name, "profile", p.profile)
	return sess, nil
}

func (st *state) freeNumber() string {
	for i := 0; ; i++ {
		if _, ok := st.sessions[strconv.Itoa(i)]; !ok {
			return strconv.Itoa(i)
		}
	}
}

func (st *state) killSession(name string) error {
	sess, ok := st.sessions[name]
	if !ok {
		return fmt.Errorf("%w: %s", ErrNoSession, name)
	}
	for _, w := range sess.windows {
		_ = w.pane.term.Close()
	}
	st.endSession(sess)
	return nil
}

// endSession forgets sess and closes Done when no session is left.
func (st *state) endSession(sess *session) {
	delete(st.sessions, sess.name)
	for c := range st.clients {
		if c.session == sess.name {
			st.dropClient(c)
		}
	}
	st.srv.log.Info("session ended", "session", sess.name)
	if st.hadOne && len(st.sessions) == 0 {
		select {
		case <-st.srv.done:
		default:
			close(st.srv.done)
		}
	}
}

// paneExited removes the pane whose program exited, and its window and
// session when they become empty.
func (st *state) paneExited(id int) {
	for _, sess := range st.sessions {
		for i, w := range sess.windows {
			if w.pane.id != id {
				continue
			}
			_ = w.pane.term.Close()
			sess.windows = slices.Delete(sess.windows, i, i+1)
			if len(sess.windows) == 0 {
				st.endSession(sess)
			}
			return
		}
	}
}

func (st *state) sessionList() []api.SessionInfo {
	out := make([]api.SessionInfo, 0, len(st.sessions))
	for _, sess := range st.sessions {
		out = append(out, api.SessionInfo{
			Name:     sess.name,
			Windows:  len(sess.windows),
			State:    api.StateRunning,
			LastUsed: sess.lastUsed,
		})
	}
	slices.SortFunc(out, func(a, b api.SessionInfo) int { return cmp.Compare(a.Name, b.Name) })
	return out
}

func (st *state) closeAll() {
	for c := range st.clients {
		st.dropClient(c)
	}
	for _, sess := range st.sessions {
		for _, w := range sess.windows {
			_ = w.pane.term.Close()
		}
	}
	st.sessions = map[string]*session{}
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
