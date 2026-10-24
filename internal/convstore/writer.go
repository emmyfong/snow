package convstore

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/emmyfong/snow/internal/private"
)

// partialEvery is the shortest time between two partial writes of one
// conversation.
const partialEvery = time.Second

// errClosed is the cause of a write to a closed Writer.
var errClosed = errors.New("writer is closed")

// Writer appends records to one conversation file. It is the only writer
// of that file in the process. Its methods are safe to call from several
// goroutines.
type Writer struct {
	s     *Store
	id    string
	path  string
	every time.Duration

	mu      sync.Mutex
	f       *os.File
	newLine bool        // the file ends in the middle of a line
	pending *Partial    // the latest partial text, not written yet
	timer   *time.Timer // writes pending when it fires
	gen     int         // the timer's number; a stopped timer that already fired sees a newer one
	err     error       // a failed partial write, returned by the next call
	closed  bool
}

// Writer opens the conversation id for appending, and creates its file if
// it does not exist. A second Writer for the same id gives an error that
// wraps ErrInUse until the first is closed.
func (s *Store) Writer(id string) (*Writer, error) {
	path, err := s.Path(id)
	if err != nil {
		return nil, err
	}
	if err := s.claim(id); err != nil {
		return nil, err
	}
	w := &Writer{s: s, id: id, path: path, every: partialEvery}
	if err := w.open(); err != nil {
		s.release(id)
		return nil, &WriteError{Path: path, Err: err}
	}
	return w, nil
}

func (w *Writer) open() error {
	_, statErr := os.Lstat(w.path)
	created := errors.Is(statErr, os.ErrNotExist)
	f, err := os.OpenFile(w.path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	if created {
		// Flush the folder entry too, or a power cut can lose the whole file
		// with every record that was flushed into it.
		if err := private.SyncDir(filepath.Dir(w.path)); err != nil {
			return errors.Join(fmt.Errorf("flush folder: %w", err), f.Close())
		}
	} else if w.newLine, err = endsMidLine(w.path); err != nil {
		return errors.Join(err, f.Close())
	}
	w.f = f
	return nil
}

// endsMidLine reports whether the file at path ends without a newline: a
// line torn by a crash, which the next record must not join.
func endsMidLine(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("check last line: %w", err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return false, fmt.Errorf("check last line: %w", err)
	}
	if info.Size() == 0 {
		return false, nil
	}
	var last [1]byte
	if _, err := f.ReadAt(last[:], info.Size()-1); err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("check last line: %w", err)
	}
	return last[0] != '\n', nil
}

// Path returns the file this Writer appends to.
func (w *Writer) Path() string { return w.path }

// Append writes r as one whole line and flushes it to disk before it
// returns. Partial text that waits is dropped by an Assistant record, which
// replaces it, and written first before a User or Interruption record,
// which ends the reply, so a cut-off reply keeps it. Other records leave it
// waiting.
func (w *Writer) Append(r Record) error {
	line, err := Encode(r)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ready(); err != nil {
		return err
	}
	switch r.Type() {
	case TypeAssistant:
		p := w.takePending()
		if err := w.write(line); err != nil {
			// Keep the text: without its reply on disk it is all there is.
			w.pending = p
			w.startTimer()
			return err
		}
		return nil
	case TypeUser, TypeInterruption:
		if p := w.takePending(); p != nil {
			if err := w.writePartial(p); err != nil {
				return err
			}
		}
	}
	return w.write(line)
}

// AppendPartial saves the text of a reply that is still streaming. It
// writes at most once per partialEvery for this conversation and keeps only
// the latest text: the first call starts the wait, and the text current
// when it ends is written.
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

// Close writes partial text that is still waiting, closes the file, and
// lets another Writer open the conversation.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	var errs []error
	if w.err != nil {
		errs = append(errs, w.err)
	}
	if w.pending != nil {
		errs = append(errs, w.writePartial(w.takePending()))
	}
	w.closed = true
	if err := w.f.Close(); err != nil {
		errs = append(errs, &WriteError{Path: w.path, Err: fmt.Errorf("close: %w", err)})
	}
	w.s.release(w.id)
	return errors.Join(errs...)
}

// ready returns the error that stops a write, if any. The caller holds mu.
func (w *Writer) ready() error {
	if w.closed {
		return &WriteError{Path: w.path, Err: errClosed}
	}
	if w.err != nil {
		err := w.err
		w.err = nil
		return err
	}
	return nil
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
		return err
	}
	return w.write(line)
}

// write appends one line and flushes it. The caller holds mu. A line that
// follows a torn one, or a failed write that may have left half a line,
// starts with a newline: a blank line is skipped on load.
func (w *Writer) write(line []byte) error {
	if w.newLine {
		line = append([]byte{'\n'}, line...)
	}
	if _, err := w.f.Write(line); err != nil {
		w.newLine = true
		return &WriteError{Path: w.path, Err: fmt.Errorf("write: %w", err)}
	}
	w.newLine = false
	if err := w.f.Sync(); err != nil {
		return &WriteError{Path: w.path, Err: fmt.Errorf("flush: %w", err)}
	}
	return nil
}
