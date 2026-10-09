package api

import (
	"errors"
	"fmt"
	"time"
)

// Handshake errors. Callers match them with errors.Is.
var (
	ErrVersionMismatch   = errors.New("api: protocol version mismatch")
	ErrUnexpectedMessage = errors.New("api: unexpected message")
)

// VersionMismatchError carries both sides' versions, so a client can tell
// the user which server is running and offer to restart it.
type VersionMismatchError struct {
	ClientVersion, ServerVersion   string
	ClientProtocol, ServerProtocol int
}

// Error describes both sides' versions.
func (e *VersionMismatchError) Error() string {
	return fmt.Sprintf("api: client %s (protocol %d) cannot talk to server %s (protocol %d)",
		e.ClientVersion, e.ClientProtocol, e.ServerVersion, e.ServerProtocol)
}

// Is makes errors.Is(err, ErrVersionMismatch) true.
func (e *VersionMismatchError) Is(target error) bool { return target == ErrVersionMismatch }

// handshakeTimeout bounds each side's wait during the handshake, so a peer
// that connects and stays silent cannot pin a goroutine forever.
var handshakeTimeout = 5 * time.Second

// withDeadline runs f under the handshake deadline, then clears it.
func withDeadline(c *Conn, f func() error) error {
	if err := c.SetDeadline(time.Now().Add(handshakeTimeout)); err != nil {
		return fmt.Errorf("set handshake deadline: %w", err)
	}
	err := f()
	if clearErr := c.SetDeadline(time.Time{}); err == nil && clearErr != nil {
		err = fmt.Errorf("clear handshake deadline: %w", clearErr)
	}
	return err
}

// ClientHandshake sends h and reads the server's Welcome within the handshake
// timeout. On a protocol mismatch it returns the Welcome together with a
// *VersionMismatchError. A server Error is returned as a *Error.
func ClientHandshake(c *Conn, h Hello) (*Welcome, error) {
	var w *Welcome
	err := withDeadline(c, func() error {
		var err error
		w, err = clientHandshake(c, h)
		return err
	})
	return w, err
}

func clientHandshake(c *Conn, h Hello) (*Welcome, error) {
	if err := c.Send(&h); err != nil {
		return nil, err
	}
	m, err := c.Receive()
	if err != nil {
		return nil, fmt.Errorf("read welcome: %w", err)
	}
	switch m := m.(type) {
	case *Welcome:
		if m.Protocol != h.Protocol {
			return m, mismatch(&h, m)
		}
		return m, nil
	case *Error:
		return nil, m
	default:
		return nil, fmt.Errorf("%w: %T before welcome", ErrUnexpectedMessage, m)
	}
}

// ServerHandshake reads the client's Hello within the handshake timeout and
// always answers with w, so a client on another protocol still learns the
// server's version. A first message that is not a readable Hello gets an
// Error with CodeBadHandshake before the error is returned.
func ServerHandshake(c *Conn, w Welcome) (*Hello, error) {
	var h *Hello
	err := withDeadline(c, func() error {
		var err error
		h, err = serverHandshake(c, w)
		return err
	})
	return h, err
}

func serverHandshake(c *Conn, w Welcome) (*Hello, error) {
	m, err := c.Receive()
	if errors.Is(err, ErrBadPayload) || errors.Is(err, ErrUnknownType) {
		return nil, rejectHandshake(c, fmt.Errorf("read hello: %w", err))
	}
	if err != nil {
		return nil, fmt.Errorf("read hello: %w", err)
	}
	h, ok := m.(*Hello)
	if !ok {
		return nil, rejectHandshake(c, fmt.Errorf("%w: %T before hello", ErrUnexpectedMessage, m))
	}
	if err := c.Send(&w); err != nil {
		return h, err
	}
	if h.Protocol != w.Protocol {
		return h, mismatch(h, &w)
	}
	return h, nil
}

func mismatch(h *Hello, w *Welcome) error {
	return &VersionMismatchError{
		ClientVersion: h.ClientVersion, ServerVersion: w.ServerVersion,
		ClientProtocol: h.Protocol, ServerProtocol: w.Protocol,
	}
}

// rejectHandshake tells the client why the handshake failed, then returns
// cause.
func rejectHandshake(c *Conn, cause error) error {
	sendErr := c.Send(&Error{Code: CodeBadHandshake, Message: "the first message must be a Hello"})
	return errors.Join(cause, sendErr)
}
