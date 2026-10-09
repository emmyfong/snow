package server

import (
	"os"
	"path/filepath"

	"github.com/emmyfong/snow/internal/term"
)

// pane is one running program in a window.
type pane struct {
	id      int
	profile string
	term    *term.Pane
}

// startPane starts the default program in dir at cols by rows and tells the
// model goroutine when it exits.
func (s *Server) startPane(id, cols, rows int, dir string) (*pane, error) {
	spec, profile := s.spec(usableDir(dir))
	tp, err := term.Start(spec, cols, rows, term.Options{Scrollback: s.opts.Scrollback})
	if err != nil {
		return nil, err
	}
	go func() {
		<-tp.Done()
		s.post(func(st *state) { st.paneExited(id) })
	}()
	return &pane{id: id, profile: profile, term: tp}, nil
}

// usableDir returns dir if it is an existing absolute folder, or else the
// user's home folder. The server, not the client, starts the program, and
// it may not see the client's folder.
func usableDir(dir string) string {
	if filepath.IsAbs(dir) {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return dir
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}
