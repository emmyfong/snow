package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

// Options configures Serve.
type Options struct {
	// Version is Snow's version, for initialize.
	Version string
	// Pane is SNOW_PANE: the pane Claude Code runs in. Socket is
	// SNOW_SOCKET: the Snow server. Both come from the pane's environment.
	// Outside a Snow pane they are empty, and the server offers no tools.
	Pane   string
	Socket string
	// Panes replaces the Snow server for tests. Nil talks to Socket.
	Panes Panes
	// Log receives what goes wrong. Nil logs to stderr.
	Log *log.Logger
}

// Serve runs the MCP server on in and out until the client closes in or
// stdout fails. It writes only MCP messages to out. A tool call reaches the
// Snow server on the side, so a slow tool never stops the next message from
// being read.
func Serve(ctx context.Context, in io.Reader, out io.Writer, o Options) error {
	logger := o.Log
	if logger == nil {
		logger = log.New(os.Stderr, "snow mcp: ", 0)
	}
	s := &server{
		version: o.Version,
		log:     logger,
		panes:   o.Panes,
		out:     make(chan []byte, 16),
		calls:   map[string]context.CancelFunc{},
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	defer s.cancel()
	if s.panes == nil {
		s.panes, s.outside = dialSnow(o.Pane, o.Socket)
	}
	if s.outside != "" {
		logger.Print(s.outside)
	}

	s.writeWG.Add(1)
	go s.writeLoop(out)

	// Read on its own goroutine, so a stdout that fails stops the server
	// even while the client keeps its input open.
	lines := make(chan []byte)
	readErr := make(chan error, 1)
	go func() { readErr <- readLoop(in, lines, s.ctx.Done()) }()
	for {
		select {
		case line := <-lines:
			s.handle(line)
		case err := <-readErr:
			return s.stop(err)
		case <-s.ctx.Done():
			// stdout failed, or the caller went away: no reply can reach
			// the client.
			return s.stop(ctx.Err())
		}
	}
}

// readLoop reads one message at a time from the client and hands it on. It
// ends when the client closes its input or the server stops. A read that
// blocks when the server stops stays blocked until the client closes its
// input; the process exits first.
func readLoop(in io.Reader, lines chan<- []byte, stop <-chan struct{}) error {
	r := bufio.NewReader(in)
	for {
		line, err := readLine(r)
		if err != nil {
			return err
		}
		if len(line) == 0 {
			continue
		}
		select {
		case lines <- line:
		case <-stop:
			return nil
		}
	}
}

// stop ends the server: it stops the tool calls, lets the writer finish, and
// reports why.
func (s *server) stop(err error) error {
	s.cancel()
	s.wg.Wait()
	close(s.out)
	s.writeWG.Wait()
	if errors.Is(err, io.EOF) {
		err = nil
	}
	return errors.Join(err, s.failure())
}

// server is one running MCP server.
type server struct {
	version string
	panes   Panes
	// outside says why no tools run: empty inside a Snow pane.
	outside string
	log     *log.Logger

	ctx    context.Context
	cancel context.CancelFunc

	out     chan []byte
	writeWG sync.WaitGroup

	// mu guards calls, the running tool calls by request ID.
	mu    sync.Mutex
	calls map[string]context.CancelFunc
	// wg counts the tool calls, so Serve waits for them before it closes
	// stdout.
	wg sync.WaitGroup

	errMu sync.Mutex
	err   error
}

// writeLoop is the only goroutine that writes to stdout, so two replies
// never interleave.
func (s *server) writeLoop(out io.Writer) {
	defer s.writeWG.Done()
	for line := range s.out {
		if _, err := out.Write(append(line, '\n')); err != nil {
			s.fail(fmt.Errorf("mcp: write to stdout: %w", err))
			return
		}
	}
}

// send queues one wire line. It drops the line once the server stops.
func (s *server) send(line []byte) {
	select {
	case s.out <- line:
	case <-s.ctx.Done():
	}
}

// fail remembers err and stops every running call.
func (s *server) fail(err error) {
	s.errMu.Lock()
	if s.err == nil {
		s.err = err
	}
	s.errMu.Unlock()
	s.cancel()
}

// failure returns the first error that stopped the server, or nil.
func (s *server) failure() error {
	s.errMu.Lock()
	defer s.errMu.Unlock()
	return s.err
}

// handle runs one client message. A request that changes a pane runs in its
// own goroutine; everything else is answered here.
func (s *server) handle(line []byte) {
	var r request
	if err := json.Unmarshal(line, &r); err != nil {
		s.send(encode(nil, nil, &rpcError{Code: codeParseError, Message: "parse error: " + err.Error()}))
		return
	}
	notification := len(r.ID) == 0
	if r.JSONRPC != jsonrpcVersion || r.Method == "" {
		if !notification {
			s.send(encode(r.ID, nil, &rpcError{Code: codeInvalidRequest, Message: "not a JSON-RPC 2.0 request"}))
		}
		return
	}
	switch r.Method {
	case "initialize":
		if !notification {
			s.send(encode(r.ID, s.initialize(r.Params), nil))
		}
	case "notifications/initialized":
	case "notifications/cancelled":
		s.cancelled(r.Params)
	case "ping":
		if !notification {
			s.send(encode(r.ID, struct{}{}, nil))
		}
	case "tools/list":
		if !notification {
			s.send(encode(r.ID, listResult{Tools: s.toolList()}, nil))
		}
	case "tools/call":
		if !notification {
			s.startCall(r)
		}
	default:
		if !notification {
			s.send(encode(r.ID, nil, &rpcError{Code: codeMethodNotFound, Message: "method not found: " + r.Method}))
		}
	}
}

// initialize answers the client's opening request.
func (s *server) initialize(params json.RawMessage) initResult {
	var in struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	// A missing or unreadable version only decides which version Snow names.
	_ = json.Unmarshal(params, &in)
	out := initResult{
		ProtocolVersion: knownVersion(in.ProtocolVersion),
		Capabilities:    capabilities{Tools: toolsCapability{}},
		ServerInfo:      serverInfo{Name: "snow", Version: s.version},
	}
	out.Instructions = "Snow's pane tools reach only the terminal panes of the session that holds your pane. " +
		"Call list_panes first: a pane keeps its ID while it runs, and a restarted pane gets a new one. " +
		"Claude Code asks the user before a tool changes a pane."
	if s.outside != "" {
		out.Instructions = s.outside
	}
	return out
}

// knownVersion returns v when Snow knows it, or else Snow's own version.
func knownVersion(v string) string {
	switch v {
	case "2024-11-05", "2025-03-26", "2025-06-18":
		return v
	}
	return protocolVersion
}
