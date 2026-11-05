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
	closed  bool
	writing sync.Mutex     // one write at a time
	running sync.WaitGroup // saves taken and not yet written
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

// fire writes ps unless a later change for id, or Close, took its place.
func (s *ProjectSaver) fire(id string, ps *projectSave) {
	if !s.take(id, ps) {
		return
	}
	s.write(ps)
}

// take removes ps from the pending saves and counts it as running. It
// reports false when ps is no longer pending.
func (s *ProjectSaver) take(id string, ps *projectSave) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending[id] != ps {
		return false
	}
	delete(s.pending, id)
	s.running.Add(1)
	return true
}

func (s *ProjectSaver) write(ps *projectSave) {
	defer s.running.Done()
	key := ps.key() // may read the disk: outside the write lock
	s.writing.Lock()
	defer s.writing.Unlock()
	if err := s.p.SaveLayout(key, ps.layout); err != nil && s.report != nil {
		s.report(key, err)
	}
}

// Close writes every pending change now, waits for writes under way, and
// stops the saver. Later calls to Changed do nothing.
func (s *ProjectSaver) Close() {
	s.mu.Lock()
	s.closed = true
	var due []*projectSave
	for id, ps := range s.pending {
		// A timer that already fired finds its save gone and writes
		// nothing; this write takes its place.
		ps.timer.Stop()
		delete(s.pending, id)
		s.running.Add(1)
		due = append(due, ps)
	}
	s.mu.Unlock()
	for _, ps := range due {
		s.write(ps)
	}
	s.running.Wait()
}
