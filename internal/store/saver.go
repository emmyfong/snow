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
	Move(from, to string) error
}

// Saver writes a session a fixed delay after its last change, so a burst of
// changes costs one write, and deletes and moves snapshots. One goroutine
// does all the disk work, in the order the calls were made, so a delete can
// never undo a later save. The calls themselves never wait for disk: the
// server makes them from a goroutine that must not block. It is safe for
// concurrent use. Close stops it.
type Saver struct {
	w      Writer
	delay  time.Duration
	report func(session string, err error)

	mu      sync.Mutex
	pending map[string]*pendingSave // saves waiting for their delay
	queue   []job                   // disk work, in call order
	busy    bool                    // the writer is running a job
	idle    *sync.Cond              // signalled when the queue empties
	wake    chan struct{}
	closed  bool
	stopped chan struct{}
}

type pendingSave struct {
	snap  Snapshot
	timer *time.Timer
}

// job is one piece of disk work. done, when set, gets its result.
type job struct {
	save     *Snapshot
	delete   string
	from, to string
	done     func(error)
}

// NewSaver returns a Saver that writes to w delay after the last change of
// each session. report gets the result of every save and delete, nil when
// it worked, on the Saver's goroutine; it must not be nil.
func NewSaver(w Writer, delay time.Duration, report func(session string, err error)) *Saver {
	s := &Saver{w: w, delay: delay, report: report, pending: map[string]*pendingSave{},
		wake: make(chan struct{}, 1), stopped: make(chan struct{})}
	s.idle = sync.NewCond(&s.mu)
	go s.run()
	return s
}

// Changed records the latest state of a session and restarts its timer. The
// Saver keeps snap until it writes it, so the caller must not change its
// slices afterwards.
func (s *Saver) Changed(snap Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if old := s.pending[snap.Name]; old != nil {
		old.timer.Stop()
	}
	p := &pendingSave{snap: snap}
	p.timer = time.AfterFunc(s.delay, func() { s.due(p) })
	s.pending[snap.Name] = p
}

// due queues a save whose delay has passed.
func (s *Saver) due(p *pendingSave) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// A timer that fired while Changed, Delete, Move, or Flush held mu is
	// stale: they replaced or removed its entry.
	if s.pending[p.snap.Name] != p {
		return
	}
	delete(s.pending, p.snap.Name)
	s.push(job{save: &p.snap})
}

// Delete drops any pending save for session and queues the removal of its
// file, for a session that was killed.
func (s *Saver) Delete(session string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.drop(session)
	s.push(job{delete: session})
}

// Move queues renaming the saved session from to to. done, if not nil,
// gets the result on the Saver's goroutine.
func (s *Saver) Move(from, to string, done func(error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.drop(from)
	s.push(job{from: from, to: to, done: done})
}

// Flush queues every pending save now, then waits until the queue is done,
// for a clean server exit. It returns the errors of the saves it queued.
func (s *Saver) Flush() error {
	s.mu.Lock()
	var errs []error
	var errMu sync.Mutex
	for _, name := range slices.Sorted(maps.Keys(s.pending)) {
		p := s.pending[name]
		p.timer.Stop()
		delete(s.pending, name)
		s.push(job{save: &p.snap, done: func(err error) {
			if err != nil {
				errMu.Lock()
				errs = append(errs, err)
				errMu.Unlock()
			}
		}})
	}
	for len(s.queue) > 0 || s.busy {
		s.idle.Wait()
	}
	s.mu.Unlock()
	errMu.Lock()
	defer errMu.Unlock()
	return errors.Join(errs...)
}

// Close writes what is pending, like Flush, and stops the Saver. Later
// calls do nothing.
func (s *Saver) Close() error {
	err := s.Flush()
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		close(s.wake)
	}
	s.mu.Unlock()
	<-s.stopped
	return err
}

// drop cancels a pending save. The caller holds mu.
func (s *Saver) drop(session string) {
	if p := s.pending[session]; p != nil {
		p.timer.Stop()
		delete(s.pending, session)
	}
}

// push adds a job and wakes the writer. The caller holds mu.
func (s *Saver) push(j job) {
	if s.closed {
		return
	}
	s.queue = append(s.queue, j)
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// run is the writer: it does the queued jobs one at a time, without mu, so
// callers never wait for disk.
func (s *Saver) run() {
	defer close(s.stopped)
	for range s.wake {
		for {
			s.mu.Lock()
			if len(s.queue) == 0 {
				s.idle.Broadcast()
				s.mu.Unlock()
				break
			}
			j := s.queue[0]
			s.queue = s.queue[1:]
			s.busy = true
			s.mu.Unlock()
			s.do(j)
			s.mu.Lock()
			s.busy = false
			s.mu.Unlock()
		}
	}
}

func (s *Saver) do(j job) {
	var err error
	switch {
	case j.save != nil:
		err = s.w.Save(*j.save)
		s.report(j.save.Name, err)
	case j.delete != "":
		err = s.w.Delete(j.delete)
		s.report(j.delete, err)
	default:
		err = s.w.Move(j.from, j.to)
	}
	if j.done != nil {
		j.done(err)
	}
}

// Abandon stops the Saver without writing what is pending: saves still
// waiting for their delay are dropped. Work already queued finishes. It is
// for a stop by signal, when the disk holds the state to keep.
func (s *Saver) Abandon() {
	s.mu.Lock()
	for name := range s.pending {
		s.drop(name)
	}
	s.mu.Unlock()
	_ = s.Close()
}
