package client

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/emmyfong/snow/internal/transport"
)

// ErrServerDidNotStart means the server was started but never answered.
var ErrServerDidNotStart = errors.New("client: server did not start")

// startTimeout bounds how long Connect waits for a server it started.
const startTimeout = 3 * time.Second

// Connect dials the server at path. When no server is there, it calls start
// once and retries with growing pauses until the server answers or
// startTimeout passes.
func Connect(path string, start func() error) (net.Conn, error) {
	c, err := transport.Dial(path)
	if err == nil {
		return c, nil
	}
	if !errors.Is(err, transport.ErrNoServer) {
		return nil, err
	}
	if err := start(); err != nil {
		return nil, fmt.Errorf("start server: %w", err)
	}
	pause := 20 * time.Millisecond
	deadline := time.Now().Add(startTimeout)
	for time.Now().Before(deadline) {
		time.Sleep(pause)
		if c, err = transport.Dial(path); err == nil {
			return c, nil
		}
		pause = min(pause*2, 200*time.Millisecond)
	}
	return nil, fmt.Errorf("%w within %v: %w", ErrServerDidNotStart, startTimeout, err)
}
