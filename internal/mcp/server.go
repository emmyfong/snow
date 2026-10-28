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
	"strconv"
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

// Serve runs the MCP server on in and out until the client closes in. It
// writes only MCP messages to out. A tool call reaches the Snow server on
// the side, so a slow tool never stops the next message from being read.
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
	r := bufio.NewReader(in)
	for {
		line, err := readLine(r)
		if err != nil {
			s.cancel()
			s.wg.Wait()
			close(s.out)
			s.writeWG.Wait()
			if errors.Is(err, io.EOF) {
				err = nil
			}
			return errors.Join(err, s.failure())
		}
		if len(line) > 0 {
			s.handle(line)
		}
	}
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

// startCall runs one tool call in its own goroutine, so a long wait never
// stops a cancellation from being read.
func (s *server) startCall(r request) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(r.Params, &p); err != nil || p.Name == "" {
		s.send(encode(r.ID, nil, &rpcError{Code: codeInvalidParams, Message: "tools/call needs a tool name"}))
		return
	}
	id := idKey(r.ID)
	ctx, cancel := context.WithCancel(s.ctx)
	s.mu.Lock()
	s.calls[id] = cancel
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer cancel()
		defer func() {
			s.mu.Lock()
			delete(s.calls, id)
			s.mu.Unlock()
		}()
		result := s.runTool(ctx, p.Name, p.Arguments)
		s.send(encode(r.ID, result, nil))
	}()
}

// runTool calls one tool and turns its outcome into an MCP result. A tool
// error goes back as a failed result, which the agent reads, not as a
// protocol error.
func (s *server) runTool(ctx context.Context, name string, args json.RawMessage) (result callResult) {
	defer func() {
		if v := recover(); v != nil {
			result = errorResult(fmt.Sprintf("snow mcp: internal error: %v", v))
		}
	}()
	text, err := s.callTool(ctx, name, args)
	if err != nil {
		return errorResult(err.Error())
	}
	return textResult(text)
}

// callTool runs the named tool.
func (s *server) callTool(ctx context.Context, name string, args json.RawMessage) (string, error) {
	if s.panes == nil {
		return "", errors.New(s.outside)
	}
	switch name {
	case "list_panes":
		return s.listPanes(ctx)
	case "read_pane":
		return s.readPane(ctx, args)
	case "send_keys":
		return s.sendKeys(ctx, args)
	case "wait_for":
		return s.waitFor(ctx, args)
	}
	return "", fmt.Errorf("unknown tool %q; tools/list names the tools", name)
}

// cancelled stops the tool call the notification names, if it still runs.
func (s *server) cancelled(params json.RawMessage) {
	var p struct {
		RequestID json.RawMessage `json:"requestId"`
	}
	if err := json.Unmarshal(params, &p); err != nil || len(p.RequestID) == 0 {
		return
	}
	s.mu.Lock()
	cancel := s.calls[idKey(p.RequestID)]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// dialSnow builds the Snow client from the pane's environment. It returns
// why no tools run when the caller is not in a Snow pane.
func dialSnow(pane, socket string) (Panes, string) {
	switch {
	case pane == "" || socket == "":
		return nil, "snow mcp runs inside a Snow pane, and this is not one: SNOW_PANE and SNOW_SOCKET are not set. Start Claude Code in a Snow pane."
	}
	caller, err := strconv.Atoi(pane)
	if err != nil || caller < 1 {
		return nil, fmt.Sprintf("snow mcp: SNOW_PANE %q is not a pane number; start Claude Code in a Snow pane.", pane)
	}
	return &snow{socket: socket, caller: caller, dial: nil}, ""
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

// initResult is the reply to initialize.
type initResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    capabilities `json:"capabilities"`
	ServerInfo      serverInfo   `json:"serverInfo"`
	Instructions    string       `json:"instructions,omitempty"`
}

type capabilities struct {
	Tools toolsCapability `json:"tools"`
}

type toolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// listResult is the reply to tools/list.
type listResult struct {
	Tools []toolDef `json:"tools"`
}

// callResult is the reply to tools/call.
type callResult struct {
	Content []content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

type content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func textResult(text string) callResult {
	return callResult{Content: []content{{Type: "text", Text: text}}}
}

func errorResult(text string) callResult {
	return callResult{Content: []content{{Type: "text", Text: text}}, IsError: true}
}
