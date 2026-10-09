package server

import "github.com/emmyfong/snow/internal/term"

// pane is one running program in a window.
type pane struct {
	id      int
	profile string
	term    *term.Pane
}

// startPane starts the default program at cols by rows and tells the model
// goroutine when it exits.
func (s *Server) startPane(id, cols, rows int) (*pane, error) {
	spec, profile := s.spec()
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
