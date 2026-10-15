package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/emmyfong/snow/internal/transport"
)

// defaultIdleExit is how long a new server waits for its first session before
// it exits, so a failed client start never leaves an idle server behind.
const defaultIdleExit = 10 * time.Second

// Run serves at the socket path until the last session ends, ctx is canceled,
// or no session appears within the idle timeout. It removes the socket when
// it returns.
func Run(ctx context.Context, opts Options, path string) error {
	l, err := transport.Listen(path)
	if err != nil {
		return fmt.Errorf("start server: %w", err)
	}
	s := New(opts)
	defer s.Close()
	go s.accept(l)
	defer func() { _ = l.Close() }()

	idle := opts.IdleExit
	if idle <= 0 {
		idle = defaultIdleExit
	}
	idleTimer := time.NewTimer(idle)
	defer idleTimer.Stop()
	for {
		select {
		case <-s.Done():
			s.log.Info("last session ended; exiting")
			return nil
		case <-ctx.Done():
			s.log.Info("stopping", "reason", ctx.Err())
			return nil
		case <-idleTimer.C:
			if len(s.Sessions()) == 0 {
				s.log.Info("no session started; exiting")
				return nil
			}
		}
	}
}

// Accept failures, such as running out of file descriptors, usually last a
// while. Waiting between retries, as net/http does, keeps a failing accept
// loop from spinning the CPU and filling the log.
const (
	minAcceptDelay = 5 * time.Millisecond
	maxAcceptDelay = time.Second
)

// accept serves each connection in its own goroutine until l closes.
func (s *Server) accept(l net.Listener) {
	var delay time.Duration
	for {
		c, err := l.Accept()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		if err != nil {
			delay = min(max(2*delay, minAcceptDelay), maxAcceptDelay)
			s.log.Warn("accept", "err", err, "retry", delay)
			time.Sleep(delay)
			continue
		}
		delay = 0
		go s.serveConn(c)
	}
}
