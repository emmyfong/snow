package store

import (
	"errors"
	"maps"
	"slices"
	"sync"
	"time"
)

// Writer is where a Saver writes snapshots. *Store implements it.
type Writer interface {
	Save(Snapshot) error
	Delete(session string) error
}

// Saver writes a session a fixed delay after its last change, so a burst of
// changes costs one write. It is safe for concurrent use.
type Saver struct {
	w     Writer
	delay time.Duration
	onErr func(session string, err error)

	// mu guards pending. io serializes calls to w. Every path takes io while
	// it still holds mu, so writes and deletes reach the disk in the order
	// their callers made them: a Delete can never be undone by an older save
	// that was already running.
	mu      sync.Mutex
	io      sync.Mutex
	pending map[string]*pendingSave
}

type pendingSave struct {
	snap  Snapshot
	timer *time.Timer
}

// NewSaver returns a Saver that writes to w delay after the last change of
// each session. Background saves run on their own goroutine and report
// failures to onErr, which must not be nil.
func NewSaver(w Writer, delay time.Duration, onErr func(session string, err error)) *Saver {
	return &Saver{w: w, delay: delay, onErr: onErr, pending: map[string]*pendingSave{}}
}

// Changed records the latest state of a session and restarts its timer. The
// Saver keeps snap until it writes it, so the caller must not change its
// slices afterwards.
func (s *Saver) Changed(snap Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old := s.pending[snap.Name]; old != nil {
		old.timer.Stop()
	}
	p := &pendingSave{snap: snap}
	p.timer = time.AfterFunc(s.delay, func() { s.fire(p) })
	s.pending[snap.Name] = p
}

func (s *Saver) fire(p *pendingSave) {
	name := p.snap.Name
	s.mu.Lock()
	// A timer that fired while Changed, Delete, or Flush held mu is stale:
	// they replaced or removed its entry.
	if s.pending[name] != p {
		s.mu.Unlock()
		return
	}
	delete(s.pending, name)
	s.io.Lock()
	s.mu.Unlock()
	err := s.w.Save(p.snap)
	s.io.Unlock()
	if err != nil {
		s.onErr(name, err)
	}
}

// Delete drops any pending save for session and removes its file. The
// server calls it when a session is killed.
func (s *Saver) Delete(session string) error {
	s.mu.Lock()
	if p := s.pending[session]; p != nil {
		p.timer.Stop()
		delete(s.pending, session)
	}
	s.io.Lock()
	s.mu.Unlock()
	defer s.io.Unlock()
	return s.w.Delete(session)
}

// Flush writes every pending save now, for a clean server exit.
func (s *Saver) Flush() error {
	s.mu.Lock()
	batch := s.pending
	s.pending = map[string]*pendingSave{}
	for _, p := range batch {
		p.timer.Stop()
	}
	s.io.Lock()
	s.mu.Unlock()
	defer s.io.Unlock()
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(batch)) {
		if err := s.w.Save(batch[name].snap); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
