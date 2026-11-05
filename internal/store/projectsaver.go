package store

import (
	"encoding/json"
	"sync"
	"time"
)

// ProjectSaver writes project layouts a fixed delay after the last change
// of each, so a burst of splits and resizes costs one write. The project
// key is worked out only when the write happens, because finding it may
// read the disk. Calls never wait for disk; one write runs at a time. It
// is safe for concurrent use.
type ProjectSaver struct {
	p      *Projects
	delay  time.Duration
	report func(key string, err error)

	mu      sync.Mutex
	pending map[string]*projectSave // by the caller's id for the project
	writing sync.Mutex              // one write at a time
	closed  bool
}

type projectSave struct {
	timer  *time.Timer
	key    func() string
	layout json.RawMessage
}

// NewProjectSaver returns a saver that writes to p. report gets each
// failed write; it runs on the saver's goroutine.
func NewProjectSaver(p *Projects, delay time.Duration, report func(key string, err error)) *ProjectSaver {
	return &ProjectSaver{p: p, delay: delay, report: report, pending: map[string]*projectSave{}}
}

// Changed saves layout for the project id names after the delay, unless
// another change for id comes first. key returns the project key; it runs
// off the caller's goroutine.
func (s *ProjectSaver) Changed(id string, key func() string, layout json.RawMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if old := s.pending[id]; old != nil {
		old.timer.Stop()
	}
	ps := &projectSave{key: key, layout: layout}
	ps.timer = time.AfterFunc(s.delay, func() { s.fire(id, ps) })
	s.pending[id] = ps
}

// fire writes ps if it is still the latest change for id.
func (s *ProjectSaver) fire(id string, ps *projectSave) {
	s.mu.Lock()
	if s.pending[id] != ps {
		s.mu.Unlock()
		return
	}
	delete(s.pending, id)
	s.mu.Unlock()
	s.write(ps)
}

func (s *ProjectSaver) write(ps *projectSave) {
	s.writing.Lock()
	defer s.writing.Unlock()
	key := ps.key()
	if err := s.p.SaveLayout(key, ps.layout); err != nil && s.report != nil {
		s.report(key, err)
	}
}

// Close writes every pending change now and stops the saver. Later calls
// to Changed do nothing.
func (s *ProjectSaver) Close() {
	s.mu.Lock()
	s.closed = true
	var due []*projectSave
	for id, ps := range s.pending {
		if ps.timer.Stop() {
			due = append(due, ps)
		}
		delete(s.pending, id)
	}
	s.mu.Unlock()
	for _, ps := range due {
		s.write(ps)
	}
	// A timer that fired before Stop may still be writing.
	s.writing.Lock()
	s.writing.Unlock() //nolint:staticcheck // waits for that write
}
