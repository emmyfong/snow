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

// Causes of a WriteError for a call that is wrong, not for a disk failure.
var (
	errClosed     = errors.New("writer is closed")
	errAppendMeta = errors.New("only Create writes a meta record")
)

// Writer appends records to one conversation file. It is the only writer
// of that file in its Store. Its methods are safe to call from several
// goroutines, and every one of them can block on the disk, for as long as
// an fsync takes: call them from the conversation's own goroutine, never
// from the server's state goroutine (ADR 0007).
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

// Create makes the file of a new conversation and writes meta as its first
// record, flushed to disk, before it returns the Writer. It stamps
// meta.Version and gives meta a new ID when it has none. A file that
// exists gives an error that wraps fs.ErrExist: Create never adds to one.
func (s *Store) Create(meta Meta) (*Writer, error) {
	if meta.ID == "" {
		meta.ID = NewID(time.Now())
	}
	meta.Version = Version
	path, err := s.Path(meta.ID)
	if err != nil {
		return nil, err
	}
	line, err := Encode(&meta)
	if err != nil {
		return nil, &WriteError{Path: path, Err: err}
	}
	if err := s.claim(meta.ID); err != nil {
		return nil, err
	}
	w := &Writer{s: s, id: meta.ID, path: path, every: partialEvery}
	if err := w.create(line); err != nil {
		s.release(meta.ID)
		return nil, &WriteError{Path: path, Err: err}
	}
	return w, nil
}

// create makes the file and writes the meta line. On failure it removes
// the file, which holds no conversation yet.
func (w *Writer) create(meta []byte) error {
	f, err := os.OpenFile(w.path, os.O_WRONLY|os.O_APPEND|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	w.f = f
	err = w.write(meta)
	if err == nil {
		// Flush the folder entry too, or a power cut can lose the whole file
		// with every record that was flushed into it.
		if err = private.SyncDir(filepath.Dir(w.path)); err != nil {
			err = fmt.Errorf("flush folder: %w", err)
		}
	}
	if err != nil {
		return errors.Join(err, f.Close(), private.Remove(w.path))
	}
	return nil
}

// Writer opens an existing conversation to append to it, for example to
// resume it. A second Writer for the same id gives an error that wraps
// ErrInUse until the first is closed. A missing file gives a WriteError
// that wraps fs.ErrNotExist; Create makes new files.
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
	f, err := os.OpenFile(w.path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	if w.newLine, err = endsMidLine(w.path); err != nil {
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

// ID returns the ID of the conversation this Writer appends to.
func (w *Writer) ID() string { return w.id }

// Append writes r as one whole line and flushes it to disk before it
// returns, so it blocks until the disk has the record. Partial text that waits is dropped by an Assistant record, which
// replaces it, and written first before a User or Interruption record,
// which ends the reply, so a cut-off reply keeps it. Other records leave it
// waiting.
func (w *Writer) Append(r Record) error {
	if r != nil && r.Type() == TypeMeta {
		return &WriteError{Path: w.path, Err: errAppendMeta}
	}
	line, err := Encode(r)
	if err != nil {
		return &WriteError{Path: w.path, Err: err}
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
