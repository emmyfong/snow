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
	windows    []*window // ordered by index
	active     int       // index of the shown window
	lastUsed   time.Time
	cols, rows int
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
	sess := &session{name: name, lastUsed: time.Now(), cols: cols, rows: rows}
	w, err := st.newWindow(sess, "", dir)
	if err != nil {
		return nil, fmt.Errorf("start pane for %s: %w", name, err)
	}
	st.sessions[name] = sess
	st.hadOne = true
	st.srv.log.Info("session created", "session", name, "profile", w.name)
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
		for _, p := range w.panes {
			_ = p.term.Close()
		}
	}
	st.endSession(sess)
	return nil
}

// endSession forgets sess and closes Done when no session is left.
func (st *state) endSession(sess *session) {
	delete(st.sessions, sess.name)
	for c := range st.clients {
		if c.session == sess.name {
			st.enqueue(c, &api.Error{Code: api.CodeSessionEnded, Message: "session " + sess.name + " ended"})
			st.closeClient(c)
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
// session when they become empty. A pane that a command already closed is
// gone, so its exit does nothing.
func (st *state) paneExited(id int) {
	for _, sess := range st.sessions {
		w, p := sess.findPane(id)
		if p == nil {
			continue
		}
		// The exit status explains a shell that exits at once, such as a
		// broken profile.
		st.srv.log.Info("pane exited", "session", sess.name, "pane", id, "err", p.term.Err())
		st.removePane(sess, w, id)
		if _, ok := st.sessions[sess.name]; ok {
			st.changed(sess)
		}
		return
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
			for _, p := range w.panes {
				_ = p.term.Close()
			}
		}
	}
	st.sessions = map[string]*session{}
}
