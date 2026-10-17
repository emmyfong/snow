package server

import (
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/internal/transport"
)

// pane is one running program in a window.
type pane struct {
	id      int
	profile string
	term    *term.Pane
	ended   bool        // its WSL distribution stopped; it waits for r to restart
	dir     string      // the folder it was asked to start in, for a shell that never reports one
	closed  atomic.Bool // Snow closed it; set before its program is ended
	// fetching is set while a goroutine reads the busy pane's frame for the
	// frame loop; owned by the model goroutine.
	fetching bool
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
	p := &pane{id: id, profile: profile, term: tp, dir: dir}
	go func() {
		<-tp.Done()
		// A pane Snow closed did not end with WSL; skip asking.
		ended := !p.closed.Load() && s.wslStopped(profile)
		s.post(func(st *state) { st.paneExited(id, ended) })
	}()
	return p, nil
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

// folder returns the pane's folder: the one its shell last reported, or the
// one it started in.
func (p *pane) folder() string {
	if f := p.term.Folder(); f != "" {
		return f
	}
	return p.dir
}
