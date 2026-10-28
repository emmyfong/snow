package mcp

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/emmyfong/snow/internal/transport"
	"github.com/emmyfong/snow/pkg/api"
)

// Panes is the Snow side of the tools. The agent's pane is fixed when snow
// mcp starts, so every call reaches only the panes of that pane's session.
type Panes interface {
	// List returns the terminal panes of the caller's session.
	List(ctx context.Context) (*api.Panes, error)
}

// snow calls the Snow server over its socket. It opens one connection per
// call: the server keeps no client for an idle agent, and a restarted
// server costs nothing.
type snow struct {
	socket string
	caller int
	// dial replaces transport.DialTimeout for tests.
	dial func() (net.Conn, error)
}

// dialTimeout bounds reaching the server; handshakeTimeout bounds the
// opening exchange. A tool call itself may take minutes, so the connection
// has no deadline after the handshake.
const (
	dialTimeout      = 5 * time.Second
	handshakeTimeout = 10 * time.Second
)

// List implements Panes.
func (c *snow) List(ctx context.Context) (*api.Panes, error) {
	var out *api.Panes
	err := c.call(ctx, &api.PanesRequest{Caller: c.caller}, func(m api.Message) bool {
		p, ok := m.(*api.Panes)
		if ok {
			out = p
		}
		return ok
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// call sends req over a new connection and waits for the reply match
// accepts, or for an Error. Other messages, such as the session lists every
// connection gets, are skipped.
func (c *snow) call(ctx context.Context, req api.Message, match func(api.Message) bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	raw, err := c.connect()
	if err != nil {
		return err
	}
	defer func() { _ = raw.Close() }() // the socket is done either way
	conn := api.NewConn(raw)
	if err := raw.SetDeadline(time.Now().Add(handshakeTimeout)); err != nil {
		return fmt.Errorf("snow mcp: set a deadline on the Snow socket: %w", err)
	}
	if _, err := api.ClientHandshake(conn, api.Hello{
		ClientVersion: "snow mcp",
		Protocol:      api.ProtocolVersion,
		Cols:          80,
		Rows:          24,
	}); err != nil {
		return fmt.Errorf("snow mcp: open a connection to Snow: %w", err)
	}
	// Read before sending: the server may send the session list first, and
	// a socket that is not drained would stop its writer.
	done := make(chan error, 1)
	go func() {
		for {
			m, err := conn.Receive()
			if err != nil {
				done <- err
				return
			}
			if e, ok := m.(*api.Error); ok {
				done <- fmt.Errorf("snow: %s: %s", e.Code, e.Message)
				return
			}
			if match(m) {
				done <- nil
				return
			}
		}
	}()
	if err := conn.Send(req); err != nil {
		_ = raw.Close()
		return fmt.Errorf("snow mcp: send %T to Snow: %w", req, err)
	}
	if err := raw.SetDeadline(time.Time{}); err != nil {
		_ = raw.Close()
		return fmt.Errorf("snow mcp: clear the Snow socket deadline: %w", err)
	}
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("snow mcp: %w", err)
		}
		return nil
	case <-ctx.Done():
		// Closing the connection ends the receive goroutine. Its send
		// never blocks, because done holds one value.
		_ = raw.Close()
		return ctx.Err()
	}
}

// connect opens the socket.
func (c *snow) connect() (net.Conn, error) {
	if c.dial != nil {
		return c.dial()
	}
	raw, err := transport.DialTimeout(c.socket, dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("snow mcp: reach Snow at %s: %w", c.socket, err)
	}
	return raw, nil
}
