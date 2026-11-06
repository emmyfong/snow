package clauderuntime

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

var (
	// ErrRequest means a response names no current permission request.
	ErrRequest = errors.New("runtime request is no longer pending")
	// ErrControl means the runtime refused a control operation.
	ErrControl = errors.New("runtime refused control operation")
)

// Permission is a runtime-owned tool decision. RequestID is distinct from the
// tool call ID and is scoped to this Client's process, never a global grant.
type Permission struct {
	Subtype   string          `json:"subtype"`
	Tool      string          `json:"tool_name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	AgentID   string          `json:"agent_id,omitempty"`
}

type controlReply struct {
	Subtype   string          `json:"subtype"`
	RequestID string          `json:"request_id"`
	Response  json.RawMessage `json:"response"`
}

// Client correlates control messages while forwarding ordinary runtime events.
// Its lock protects request identity only. No pipe write occurs under the lock.
type Client struct {
	stream *Stream
	frames chan Frame
	done   chan struct{}
	mu     sync.Mutex
	calls  map[string]chan controlReply
	tools  map[string]Permission
	seen   map[string]bool
	err    error // read after done closes
}

// NewClient takes ownership of stream and starts its control-message reader.
func NewClient(stream *Stream) *Client {
	c := &Client{stream: stream, frames: make(chan Frame, 64), done: make(chan struct{}),
		calls: map[string]chan controlReply{}, tools: map[string]Permission{}, seen: map[string]bool{}}
	go c.read()
	return c
}

// Frames contains ordered chat events, permission requests and cancellations.
// Control responses are consumed here and never appear as assistant text.
func (c *Client) Frames() <-chan Frame { return c.frames }

// Done closes after protocol handling and process cleanup finish.
func (c *Client) Done() <-chan struct{} { return c.done }

// Err returns the terminal error after Done closes.
func (c *Client) Err() error { return c.err }

// Initialize performs the handshake used by Claude's official SDK. The returned
// object reports capabilities; success does not certify an account or quota.
func (c *Client) Initialize(ctx context.Context) (json.RawMessage, error) {
	return c.call(ctx, map[string]any{"subtype": "initialize", "hooks": nil})
}

// Interrupt requests a main-turn interrupt. Its acknowledgement alone does not
// establish that child tasks or other owned work have stopped.
func (c *Client) Interrupt(ctx context.Context) error {
	_, err := c.call(ctx, map[string]any{"subtype": "interrupt"})
	return err
}

// StopTask requests cancellation of one runtime task, without stopping its
// parent or siblings. The controller must also observe its terminal task event.
func (c *Client) StopTask(ctx context.Context, id string) error {
	if id == "" {
		return ErrRequest
	}
	_, err := c.call(ctx, map[string]any{"subtype": "stop_task", "task_id": id})
	return err
}

func (c *Client) call(ctx context.Context, request map[string]any) (json.RawMessage, error) {
	id := "snow_" + rand.Text()
	reply := make(chan controlReply, 1)
	c.mu.Lock()
	if len(c.calls) >= 32 {
		c.mu.Unlock()
		return nil, ErrBackpressure
	}
	c.calls[id] = reply
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.calls, id)
		c.mu.Unlock()
	}()
	if err := c.stream.Send(ctx, map[string]any{"type": "control_request", "request_id": id, "request": request}); err != nil {
		return nil, err
	}
	select {
	case response := <-reply:
		if response.Subtype != "success" {
			return nil, fmt.Errorf("%w: %s", ErrControl, request["subtype"])
		}
		var object map[string]json.RawMessage
		if json.Unmarshal(response.Response, &object) != nil || object == nil {
			return nil, fmt.Errorf("%w: invalid control response", ErrProtocol)
		}
		return response.Response, nil
	case <-ctx.Done():
		return nil, errors.Join(ErrDelivery, ctx.Err())
	case <-c.done:
		return nil, errors.Join(ErrClosed, c.err)
	}
}

// Prompt sends the user's original text verbatim through the streaming SDK
// envelope. The controller queues additional messages in Snow, rather than
// relying on hidden runtime queuing.
func (c *Client) Prompt(ctx context.Context, session, text string) error {
	return c.stream.Send(ctx, map[string]any{"type": "user", "session_id": session,
		"parent_tool_use_id": nil, "client_composed": true,
		"message": map[string]string{"role": "user", "content": text}})
}

// Answer resolves exactly one pending tool request. A nil updated input allows
// the original input unchanged. Duplicate and cancelled responses are rejected.
func (c *Client) Answer(ctx context.Context, id string, allow bool, updated json.RawMessage, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if allow && updated != nil {
		var object map[string]json.RawMessage
		if json.Unmarshal(updated, &object) != nil || object == nil {
			return fmt.Errorf("%w: permission input is not an object", ErrProtocol)
		}
	}
	c.mu.Lock()
	permission, ok := c.tools[id]
	if ok {
		delete(c.tools, id)
	}
	c.mu.Unlock()
	if !ok {
		return ErrRequest
	}
	var response map[string]any
	if allow {
		if updated == nil {
			updated = permission.Input
		}
		response = map[string]any{"behavior": "allow", "updatedInput": updated}
	} else {
		response = map[string]any{"behavior": "deny", "message": reason}
	}
	return c.stream.Send(ctx, map[string]any{"type": "control_response", "response": map[string]any{
		"subtype": "success", "request_id": id, "response": response}})
}

// Stop terminates the whole owned runtime process tree and clears pending
// requests. It never sends a model prompt or a final-summary request.
func (c *Client) Stop() error {
	err := c.stream.Stop()
	<-c.done
	return errors.Join(err, c.err)
}
