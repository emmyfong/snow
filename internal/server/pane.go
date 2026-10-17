package server

import (
	"os"
	"path/filepath"

	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/internal/transport"
)

// pane is one running program in a window.
type pane struct {
	id      int
	profile string
	term    *term.Pane
	ended   bool // its WSL distribution stopped; it waits for r to restart
}

// startPane starts profile's program in dir at cols by rows, below
// preamble, and tells the model goroutine when it exits. An empty profile means the default. The
// launcher decides which folders a profile can use.
func (s *Server) startPane(id, cols, rows int, profile, dir, preamble string) (*pane, error) {
	spec, profile, err := s.launch.spec(profile, dir)
	if err != nil {
		return nil, err
	}
	if s.opts.Socket != "" {
		spec.Env = append(spec.Env, transport.PaneEnv(s.opts.Socket, id))
	}
	tp, err := term.Start(spec, max(cols, 1), max(rows, 1), term.Options{Scrollback: s.opts.Scrollback, Preamble: preamble})
	if err != nil {
		return nil, err
	}
	go func() {
		<-tp.Done()
		ended := s.wslStopped(profile)
		s.post(func(st *state) { st.paneExited(id, ended) })
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
