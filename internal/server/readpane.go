package server

import (
	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// read_pane serves an agent in a pane. The owner goroutine only finds the
// pane and copies its facts; the text is read outside it, so a large read
// never stalls the other clients (ADR 0007).

const (
	// readPaneScrollback is how many scrollback lines a request gets when
	// it does not say, and readPaneScrollbackMax the most it can ask for.
	readPaneScrollback    = 50
	readPaneScrollbackMax = 500
	// readPaneMaxText bounds the plain text of one reply, so a huge pane
	// still fits in a frame.
	readPaneMaxText = 32 << 10
)

// readPane answers a ReadPaneRequest with a plain-text copy of the pane.
func (s *Server) readPane(c *client, r *api.ReadPaneRequest) {
	if err := r.Validate(); err != nil {
		s.refuse(c, api.CodeBadCommand, err)
		return
	}
	var (
		tp          *term.Pane
		sessionName string
		profile     string
		dir         string
		ended       bool
		e           *api.Error
	)
	s.call(func(st *state) {
		sess, p, err := st.toolPane(r.Caller, r.Pane)
		if err != nil {
			e = err
			return
		}
		tp = p.term
		sessionName, profile, dir, ended = sess.name, p.profile, p.folder(), p.ended
	})
	if e != nil {
		s.reply(c, e)
		return
	}
	snap := tp.Snapshot(scrollbackLines(r.Scrollback))
	screen, back, omitted := fitRead(snap.Screen, snap.Scrollback, snap.Omitted)
	s.reply(c, &api.ReadPane{
		Pane:          r.Pane,
		Session:       sessionName,
		Profile:       profile,
		Folder:        dir,
		Cols:          snap.Cols,
		Rows:          snap.Rows,
		CursorX:       snap.CursorX,
		CursorY:       snap.CursorY,
		CursorVisible: snap.CursorVisible,
		Screen:        screen,
		Scrollback:    back,
		Omitted:       omitted,
		Ended:         ended,
	})
}

// scrollbackLines returns how many scrollback lines a request asks for.
func scrollbackLines(n int) int {
	switch {
	case n <= 0:
		return readPaneScrollback
	case n > readPaneScrollbackMax:
		return readPaneScrollbackMax
	}
	return n
}

// fitRead starts at omitted and trims a pane copy to limit bytes of plain
// text, oldest first: the scrollback above the screen, then the top screen
// rows. It keeps one screen row, so the reply always says something.
func fitRead(screen, back []string, omitted int) ([]string, []string, int) {
	size := 0
	for _, line := range screen {
		size += len(line) + 1
	}
	for _, line := range back {
		size += len(line) + 1
	}
	for len(back) > 0 && size > readPaneMaxText {
		size -= len(back[0]) + 1
		back = back[1:]
		omitted++
	}
	for len(screen) > 1 && size > readPaneMaxText {
		size -= len(screen[0]) + 1
		screen = screen[1:]
		omitted++
	}
	return screen, back, omitted
}
