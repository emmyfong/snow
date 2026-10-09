package api

import (
	"sync"
	"time"
)

// Conn sends and receives frames over one connection. Send is safe to call
// from many goroutines; Receive must be called from one goroutine at a time.
type Conn struct {
	rw readWriter
	mu sync.Mutex // serializes whole frames for writers that split a Write
}

type readWriter interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
}

// deadliner is implemented by net.Conn.
type deadliner interface {
	SetDeadline(t time.Time) error
}

// NewConn wraps rw, usually a net.Conn.
func NewConn(rw readWriter) *Conn { return &Conn{rw: rw} }

// Send writes m as one frame.
func (c *Conn) Send(m Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return WriteFrame(c.rw, m)
}

// Receive reads the next frame.
func (c *Conn) Receive() (Message, error) { return ReadFrame(c.rw) }

// SetDeadline sets a read and write deadline when the connection supports
// one, as net.Conn does. The zero time clears it. Otherwise it does nothing.
func (c *Conn) SetDeadline(t time.Time) error {
	if d, ok := c.rw.(deadliner); ok {
		return d.SetDeadline(t)
	}
	return nil
}
