package api

import (
	"errors"
	"fmt"
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

func (e *VersionMismatchError) Error() string {
	return fmt.Sprintf("api: client %s (protocol %d) cannot talk to server %s (protocol %d)",
		e.ClientVersion, e.ClientProtocol, e.ServerVersion, e.ServerProtocol)
}

// Is makes errors.Is(err, ErrVersionMismatch) true.
func (e *VersionMismatchError) Is(target error) bool { return target == ErrVersionMismatch }

// Error makes a server's Error message usable as a Go error.
func (e *Error) Error() string { return fmt.Sprintf("server: %s: %s", e.Code, e.Message) }

// ClientHandshake sends h and reads the server's Welcome. On a protocol
// mismatch it returns the Welcome together with a *VersionMismatchError.
func ClientHandshake(c *Conn, h Hello) (*Welcome, error) {
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

// ServerHandshake reads the client's Hello and always answers with w, so a
// client on another protocol still learns the server's version.
func ServerHandshake(c *Conn, w Welcome) (*Hello, error) {
	m, err := c.Receive()
	if err != nil {
		return nil, fmt.Errorf("read hello: %w", err)
	}
	h, ok := m.(*Hello)
	if !ok {
		return nil, fmt.Errorf("%w: %T before hello", ErrUnexpectedMessage, m)
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
