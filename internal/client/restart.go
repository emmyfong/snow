package client

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/emmyfong/snow/internal/transport"
	"github.com/emmyfong/snow/pkg/api"
)

// ErrServerKeptRunning means the server did not stop when asked: it is
// older than the restart request and must be stopped by hand.
var ErrServerKeptRunning = errors.New("the server kept running")

// stopTimeout bounds the wait for an old server to save and exit.
const stopTimeout = 10 * time.Second

// AskRestart asks on out whether to restart a server of another version,
// and reads the answer from in. Only y or yes restarts.
func AskRestart(in io.Reader, out io.Writer, m *api.VersionMismatchError) bool {
	_, _ = fmt.Fprintf(out, "The running server is version %s; this is %s.\n"+
		"Restart it now? Sessions are saved and come back as saved sessions. [y/N] ",
		m.ServerVersion, m.ClientVersion)
	line, _ := bufio.NewReader(in).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}

// RestartServer asks the server at path to save every session and exit,
// and waits until it is gone, so the next connect starts this version. A
// server that keeps running is older than the request: the error is then
// ErrServerKeptRunning.
func RestartServer(path, version string) error {
	raw, err := transport.Dial(path)
	if errors.Is(err, transport.ErrNoServer) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("restart server: %w", err)
	}
	conn := api.NewConn(raw)
	// The server answers any Hello, and accepts Shutdown after it even
	// when the protocols differ, which is the usual reason to restart.
	_, herr := api.ClientHandshake(conn, api.Hello{ClientVersion: version, Protocol: api.ProtocolVersion, Cols: 80, Rows: 24})
	if herr != nil && !errors.Is(herr, api.ErrVersionMismatch) {
		_ = raw.Close()
		return fmt.Errorf("restart server: %w", herr)
	}
	if err := conn.Send(&api.Shutdown{}); err != nil {
		_ = raw.Close()
		return fmt.Errorf("restart server: %w", err)
	}
	// Stay connected until the server closes the connection. Closing first
	// can make the server's next write fail and drop the connection before
	// it reads the Shutdown.
	deadline := time.Now().Add(stopTimeout)
	if err := raw.SetReadDeadline(deadline); err == nil {
		for {
			if _, err := conn.Receive(); err != nil {
				break
			}
		}
	}
	_ = raw.Close()
	for time.Now().Before(deadline) {
		c, err := transport.Dial(path)
		if errors.Is(err, transport.ErrNoServer) {
			return nil
		}
		if err == nil {
			_ = c.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}
	return ErrServerKeptRunning
}
