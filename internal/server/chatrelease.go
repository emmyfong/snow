package server

import (
	"time"
)

// releaseWait is the longest a resume waits for a conversation that a closed
// pane is still saving. A variable so a test can shorten it.
var releaseWait = 10 * time.Second

// releasing records that p's conversation, if it has one, is closing, and
// returns the function that says it is closed. The model goroutine calls it
// when it drops the pane, so any later resume sees the record.
func (s *Server) releasing(p *pane) (released func()) {
	if p.chat == nil {
		return func() {}
	}
	id := p.chat.conv.ID()
	done := make(chan struct{})
	s.releases.Store(id, done)
	return func() {
		s.releases.CompareAndDelete(id, done)
		close(done)
	}
}

// awaitRelease waits until a closing pane has let go of conversation id, so
// a resume right after the user closes a pane finds the file free. It
// returns at once for a conversation nothing is closing.
func (s *Server) awaitRelease(id string) {
	v, ok := s.releases.Load(id)
	if !ok {
		return
	}
	select {
	case <-v.(chan struct{}):
	case <-time.After(releaseWait):
	case <-s.quit:
	}
}
