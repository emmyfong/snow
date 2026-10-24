package convstore

import "time"

// partialEvery is the shortest time between two partial writes of one
// conversation.
const partialEvery = time.Second

// AppendPartial saves the text of a reply that is still streaming. It
// writes at most once per partialEvery for this conversation and keeps only
// the latest text: the first call starts the wait, and the text current
// when it ends is written. It does not write itself, but it waits while
// another call holds the file for a write and its fsync.
func (w *Writer) AppendPartial(p Partial) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ready(); err != nil {
		return err
	}
	w.pending = &p
	w.startTimer()
	return nil
}

// startTimer starts the partial wait if partial text waits and no wait
// runs. The caller holds mu.
func (w *Writer) startTimer() {
	if w.pending == nil || w.timer != nil {
		return
	}
	w.gen++
	gen := w.gen
	w.timer = time.AfterFunc(w.every, func() { w.flushPartial(gen) })
}

// flushPartial runs when the partial wait ends. A failure is kept for the
// next call, which returns it.
func (w *Writer) flushPartial(gen int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if gen != w.gen || w.closed || w.pending == nil {
		return
	}
	w.timer = nil
	if err := w.writePartial(w.takePending()); err != nil && w.err == nil {
		w.err = err
	}
}

// takePending clears the waiting partial text and returns it. The caller
// holds mu.
func (w *Writer) takePending() *Partial {
	p := w.pending
	w.pending = nil
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
		w.gen++
	}
	return p
}

func (w *Writer) writePartial(p *Partial) error {
	line, err := Encode(p)
	if err != nil {
		return &WriteError{Path: w.path, Err: err}
	}
	return w.write(line)
}
