package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// A tool call runs in its own goroutine, so a long wait never stops a
// cancellation from being read, and the server keeps one cancel function per
// request ID.

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
