package server

import (
	"cmp"
	"fmt"
	"github.com/emmyfong/snow/internal/store"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/emmyfong/snow/pkg/api"
)

const defaultCols, defaultRows = 80, 24

// state is everything the model goroutine owns.
type state struct {
	srv         *Server
	sessions    map[string]*session
	clients     map[*client]struct{}
	nextPane    int
	served      bool                   // a session or a client has existed; once neither is left, Done closes
	savedAt     map[string]uint64      // session → contentVersion at its last save
	saved       map[string]store.Info  // sessions with a saved file, running or not
	notes       []string               // notes on saved files that could not be read
	saveProblem string                 // why saving fails, or "" while it works
	endTimers   map[string]*time.Timer // sessions that ended, waiting to be deleted
}

type session struct {
	name       string
	windows    []*window // ordered by index
	active     int       // index of the shown window
	lastUsed   time.Time
	cols, rows int
}

func newState(s *Server) *state {
	return &state{srv: s, sessions: map[string]*session{}, clients: map[*client]struct{}{}, nextPane: 1, savedAt: map[string]uint64{}, saved: s.savedInfos(), notes: s.startNotes, endTimers: map[string]*time.Timer{}}
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
	st.served = true
	st.save(sess)
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
	if _, saved := st.saved[name]; !ok && saved {
		st.unsave(name)
		st.sessionsChanged()
		return nil
	}
	if !ok {
		return fmt.Errorf("%w: %s", ErrNoSession, name)
	}
	for _, w := range sess.windows {
		for _, p := range w.panes {
			st.closePane(p)
		}
	}
	st.endSession(sess)
	st.unsave(name) // killed on purpose: no wait
	return nil
}

// endSession forgets sess and closes Done when no session and no client is
// left. Its clients stay connected, detached, so they can show the home page.
func (st *state) endSession(sess *session) {
	delete(st.sessions, sess.name)
	st.unsaveLater(sess.name)
	for c := range st.clients {
		if c.session == sess.name {
			st.enqueue(c, &api.Error{Code: api.CodeSessionEnded, Message: "session " + sess.name + " ended"})
			unattach(c)
		}
	}
	st.srv.log.Info("session ended", "session", sess.name)
	st.sessionsChanged()
	st.maybeDone()
}

// paneExited removes the pane whose program exited, and its window and
// session when they become empty. A pane that ended because WSL stopped
// stays, waiting for r. A pane that a command already closed is gone, so its
// exit does nothing.
func (st *state) paneExited(id int, ended bool) {
	for _, sess := range st.sessions {
		w, p := sess.findPane(id)
		if p == nil {
			continue
		}
		if ended {
			st.endPane(sess, p)
			return
		}
		// The exit status explains a shell that exits at once, such as a
		// broken profile.
		st.srv.log.Info("pane exited", "session", sess.name, "pane", id, "err", p.term.Err())
		st.removePane(sess, w, id)
		if _, ok := st.sessions[sess.name]; ok {
			st.repaint(sess) // closing a pane unzooms its window
			st.changed(sess)
		}
		return
	}
}

func (st *state) sessionList() []api.SessionInfo {
	attached := map[string]int{}
	for c := range st.clients {
		if c.session != "" {
			attached[c.session]++
		}
	}
	out := st.savedList()
	for _, sess := range st.sessions {
		out = append(out, api.SessionInfo{
			Name:     sess.name,
			Windows:  len(sess.windows),
			Attached: attached[sess.name],
			State:    api.StateRunning,
			LastUsed: sess.lastUsed,
		})
	}
	slices.SortFunc(out, func(a, b api.SessionInfo) int { return cmp.Compare(a.Name, b.Name) })
	return out
}

// sessionsChanged sends the session list to every client that has not
// attached, such as the home page, so it always shows what runs now.
func (st *state) sessionsChanged() {
	var list *api.SessionList
	for c := range st.clients {
		if c.session != "" {
			continue
		}
		if list == nil {
			list = &api.SessionList{Sessions: st.sessionList(), Notes: st.listNotes()}
		}
		st.enqueue(c, list)
	}
}

func (st *state) closeAll() {
	for c := range st.clients {
		st.dropClient(c)
	}
	for _, sess := range st.sessions {
		for _, w := range sess.windows {
			for _, p := range w.panes {
				if err := p.term.Close(); err != nil {
					st.srv.log.Warn("pane close", "pane", p.id, "err", err)
				}
			}
		}
	}
	st.sessions = map[string]*session{}
}

// listNotes is what the home page shows under the sessions.
func (st *state) listNotes() []string {
	notes := slices.Clone(st.notes)
	if st.saveProblem != "" {
		notes = append(notes, st.saveProblem)
	}
	// A note holds file paths and errors; a control character in one would
	// reach the client's terminal as an escape sequence.
	for i, n := range notes {
		notes[i] = strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return '?'
			}
			return r
		}, n)
	}
	return notes
}
