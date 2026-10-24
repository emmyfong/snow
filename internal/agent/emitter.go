package agent

import "sync"

// emitter delivers events in order from its own goroutine, so a slow
// receiver never blocks the conversation, and no lock is held while an
// event is delivered.
type emitter struct {
	deliver func(Event)

	mu     sync.Mutex
	wake   *sync.Cond
	queue  []Event
	closed bool
	done   chan struct{}
}

func newEmitter(deliver func(Event)) *emitter {
	e := &emitter{deliver: deliver, done: make(chan struct{})}
	e.wake = sync.NewCond(&e.mu)
	go e.loop()
	return e
}

// emit queues ev. It never blocks for long.
func (e *emitter) emit(ev Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return
	}
	e.queue = append(e.queue, ev)
	e.wake.Signal()
}

func (e *emitter) loop() {
	defer close(e.done)
	for {
		e.mu.Lock()
		for len(e.queue) == 0 && !e.closed {
			e.wake.Wait()
		}
		batch := e.queue
		e.queue = nil
		closed := e.closed
		e.mu.Unlock()
		for _, ev := range batch {
			e.deliver(ev)
		}
		if closed {
			return
		}
	}
}

// close delivers the events still queued, then stops.
func (e *emitter) close() {
	e.mu.Lock()
	e.closed = true
	e.wake.Signal()
	e.mu.Unlock()
	<-e.done
}
