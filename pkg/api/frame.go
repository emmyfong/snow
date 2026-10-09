package api

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// MaxFrameSize bounds a payload so a corrupt length cannot exhaust memory.
const MaxFrameSize = 1 << 20

// Frame errors. Callers match them with errors.Is.
var (
	ErrFrameTooLarge = errors.New("api: frame too large")
	ErrUnknownType   = errors.New("api: unknown message type")
	ErrBadPayload    = errors.New("api: malformed payload")
)

const headerSize = 5 // 4-byte length + 1-byte type

// WriteFrame writes m as one frame. It writes the whole frame in one call, so
// a writer shared under a lock never interleaves frames.
func WriteFrame(w io.Writer, m Message) error {
	payload, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("encode %T: %w", m, err)
	}
	if len(payload) > MaxFrameSize {
		return fmt.Errorf("%w: %d bytes", ErrFrameTooLarge, len(payload))
	}
	frame := make([]byte, headerSize, headerSize+len(payload))
	binary.BigEndian.PutUint32(frame, uint32(len(payload))) //nolint:gosec // bounded by MaxFrameSize
	frame[4] = byte(m.Type())
	if _, err := w.Write(append(frame, payload...)); err != nil {
		return fmt.Errorf("write frame: %w", err)
	}
	return nil
}

// ReadFrame reads one frame. It returns io.EOF only when the stream ends
// cleanly between frames.
func ReadFrame(r io.Reader) (Message, error) {
	var header [headerSize]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("read frame header: %w", err)
	}
	size := binary.BigEndian.Uint32(header[:4])
	if size > MaxFrameSize {
		return nil, fmt.Errorf("%w: %d bytes", ErrFrameTooLarge, size)
	}
	m := newMessage(MsgType(header[4]))
	if m == nil {
		return nil, fmt.Errorf("%w: %d", ErrUnknownType, header[4])
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		if errors.Is(err, io.EOF) {
			err = io.ErrUnexpectedEOF
		}
		return nil, fmt.Errorf("read frame payload: %w", err)
	}
	if err := json.Unmarshal(payload, m); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadPayload, err)
	}
	return m, nil
}

// Conn sends and receives frames over one connection. Send is safe to call
// from many goroutines; Receive must be called from one goroutine at a time.
type Conn struct {
	rw io.ReadWriter
	mu sync.Mutex // serializes whole frames on the writer
}

// NewConn wraps rw, usually a net.Conn.
func NewConn(rw io.ReadWriter) *Conn { return &Conn{rw: rw} }

// Send writes m as one frame.
func (c *Conn) Send(m Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return WriteFrame(c.rw, m)
}

// Receive reads the next frame.
func (c *Conn) Receive() (Message, error) { return ReadFrame(c.rw) }
