package clauderuntime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/clauderuntime/runtimetest"
)

func TestControlRequestsAndPermissions(t *testing.T) {
	s := startTestStream(t, "control")
	c := NewClient(s)
	t.Cleanup(func() { _ = c.Stop() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := c.Initialize(ctx)
	if err != nil || !strings.HasPrefix(string(result), `{"commands":[]`) {
		t.Fatalf("initialize: %s, %v", result, err)
	}
	if frame := nextClientFrame(t, c); frame.Type != "system" {
		t.Fatalf("control acknowledgement leaked into chat: %+v", frame)
	}
	if err := c.Prompt(ctx, "saved-session", "/literal @file\noriginal text"); err != nil {
		t.Fatal(err)
	}
	if frame := nextClientFrame(t, c); frame.Type != "control_request" || frame.RequestID != "approve-1" {
		t.Fatalf("permission identity: %+v", frame)
	}
	if err := c.Answer(ctx, "other-session-request", true, nil, ""); !errors.Is(err, ErrRequest) {
		t.Fatalf("unknown request accepted: %v", err)
	}
	if err := c.Answer(ctx, "approve-1", true, json.RawMessage(`[]`), ""); !errors.Is(err, ErrProtocol) {
		t.Fatalf("non-object tool input accepted: %v", err)
	}
	if err := c.Answer(ctx, "approve-1", false, nil, "Denied by user"); err != nil {
		t.Fatal(err)
	}
	if err := c.Answer(ctx, "approve-1", true, nil, ""); !errors.Is(err, ErrRequest) {
		t.Fatalf("duplicate reply accepted: %v", err)
	}
	if frame := nextClientFrame(t, c); frame.RequestID != "cancel-2" {
		t.Fatalf("second request: %+v", frame)
	}
	if frame := nextClientFrame(t, c); frame.Type != "control_cancel_request" {
		t.Fatalf("missing cancellation: %+v", frame)
	}
	if err := c.Answer(ctx, "cancel-2", true, nil, ""); !errors.Is(err, ErrRequest) {
		t.Fatalf("cancelled request accepted: %v", err)
	}
	if err := c.Interrupt(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.StopTask(ctx, "selected-child"); err != nil {
		t.Fatal(err)
	}
	if frame := nextClientFrame(t, c); frame.Type != "result" {
		t.Fatalf("lost final event: %+v", frame)
	}
}

func TestControlRejectsReusedPermissionIdentity(t *testing.T) {
	c := NewClient(startTestStream(t, "reuse-control"))
	t.Cleanup(func() { _ = c.Stop() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := c.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	nextClientFrame(t, c)
	if err := c.Answer(ctx, "approve-1", true, nil, ""); err != nil {
		t.Fatal(err)
	}
	select {
	case <-c.Done():
	case <-ctx.Done():
		t.Fatal("reused permission ID did not terminate the protocol")
	}
	if !errors.Is(c.Err(), ErrProtocol) {
		t.Fatalf("reused request error: %v", c.Err())
	}
}

func nextClientFrame(t *testing.T, c *Client) Frame {
	t.Helper()
	select {
	case frame, ok := <-c.Frames():
		if !ok {
			t.Fatal("client ended before frame")
		}
		return frame
	case <-time.After(10 * time.Second):
		t.Fatal("client frame did not arrive")
		return Frame{}
	}
}

func controlHelper(reuse bool) {
	peer := runtimetest.NewPeer()
	read, write, ack := peer.Read, peer.Write, peer.Ack
	permission := func(id string) {
		write(map[string]any{"type": "control_request", "request_id": id, "request": map[string]any{
			"subtype": "can_use_tool", "tool_name": "Bash", "tool_use_id": "tool-1", "input": map[string]string{"command": "echo test"}}})
	}
	initialize := read()
	ack(initialize)
	if !reuse {
		write(map[string]string{"type": "system", "session_id": "saved-session"})
		prompt := read()
		var message struct {
			Content string `json:"content"`
		}
		if string(prompt["client_composed"]) != "true" || string(prompt["session_id"]) != `"saved-session"` ||
			json.Unmarshal(prompt["message"], &message) != nil || message.Content != "/literal @file\noriginal text" {
			os.Exit(3)
		}
	}
	permission("approve-1")
	answer := read()
	var response struct {
		RequestID string `json:"request_id"`
		Response  struct {
			Behavior string `json:"behavior"`
		} `json:"response"`
	}
	if json.Unmarshal(answer["response"], &response) != nil || response.RequestID != "approve-1" {
		os.Exit(4)
	}
	if reuse {
		permission("approve-1")
	} else {
		if response.Response.Behavior != "deny" {
			os.Exit(5)
		}
		permission("cancel-2")
		write(map[string]string{"type": "control_cancel_request", "request_id": "cancel-2"})
		for _, subtype := range []string{"interrupt", "stop_task"} {
			control := read()
			var request map[string]string
			if json.Unmarshal(control["request"], &request) != nil || request["subtype"] != subtype ||
				subtype == "stop_task" && request["task_id"] != "selected-child" {
				os.Exit(6)
			}
			ack(control)
		}
		write(map[string]string{"type": "result"})
	}
	for {
		read()
	}
}
