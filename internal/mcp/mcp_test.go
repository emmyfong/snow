package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/pkg/api"
)

// mcpClient speaks MCP to a Serve over pipes, as Claude Code does.
type mcpClient struct {
	in   io.WriteCloser
	out  *bufio.Reader
	done chan error
}

// start runs Serve on pipes and stops it when the test ends.
func start(t *testing.T, o Options) *mcpClient {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- Serve(context.Background(), inR, outW, o) }()
	c := &mcpClient{in: inW, out: bufio.NewReader(outR), done: done}
	t.Cleanup(func() {
		_ = inW.Close()
		_ = outR.Close()
		select {
		case err := <-done:
			_ = err
		case <-time.After(5 * time.Second):
			t.Error("snow mcp did not stop after its input closed")
		}
	})
	return c
}

func (c *mcpClient) send(t *testing.T, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if _, err := c.in.Write(append(b, '\n')); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// next reads one message, failing the test when none comes.
func (c *mcpClient) next(t *testing.T) map[string]any {
	t.Helper()
	type read struct {
		line []byte
		err  error
	}
	ch := make(chan read, 1)
	go func() {
		line, err := c.out.ReadBytes('\n')
		ch <- read{line, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("read: %v", r.err)
		}
		var m map[string]any
		if err := json.Unmarshal(r.line, &m); err != nil {
			t.Fatalf("answer is not JSON: %v: %s", err, r.line)
		}
		return m
	case <-time.After(5 * time.Second):
		t.Fatal("no answer from snow mcp")
		return nil
	}
}

// result returns a reply's result object.
func result(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	r, ok := m["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result: %v", m)
	}
	return r
}

// rpcError returns a reply's error object.
func rpcFailure(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	e, ok := m["error"].(map[string]any)
	if !ok {
		t.Fatalf("no error: %v", m)
	}
	return e
}

// toolText returns the text of a tools/call result and whether it failed.
func toolText(t *testing.T, m map[string]any) (text string, isError bool) {
	t.Helper()
	res := result(t, m)
	content, ok := res["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("no content: %v", res)
	}
	first, ok := content[0].(map[string]any)
	if !ok || first["type"] != "text" {
		t.Fatalf("no text content: %v", content)
	}
	text, _ = first["text"].(string)
	failed, _ := res["isError"].(bool)
	return text, failed
}

// fakePanes is a Snow server that never runs.
type fakePanes struct {
	panes *api.Panes
	err   error
	// block makes List wait for the context, for cancellation tests.
	block bool
}

func (f *fakePanes) List(ctx context.Context) (*api.Panes, error) {
	if f.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return f.panes, f.err
}

func testPanes() *api.Panes {
	return &api.Panes{Session: "work", Caller: 1, Panes: []api.PaneSummary{
		{ID: 1, Window: 0, WindowName: "sh", Shown: true, Profile: "bash", Folder: "/src", Rect: api.Rect{W: 80, H: 24}, Focused: true},
		{ID: 2, Window: 1, WindowName: "dev", Profile: "Claude Code", Folder: "/src/api", Rect: api.Rect{W: 60, H: 20}, Agent: true, AgentState: api.AgentWorking},
	}}
}

// TestMCPHandshake opens a session as Claude Code does: initialize, list
// the tools, and call list_panes.
func TestMCPHandshake(t *testing.T) {
	c := start(t, Options{Version: "test", Panes: &fakePanes{panes: testPanes()}})

	c.send(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{
		"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "test", "version": "1"},
	}})
	init := result(t, c.next(t))
	if init["protocolVersion"] != "2025-06-18" {
		t.Fatalf("protocolVersion = %v", init["protocolVersion"])
	}
	info, _ := init["serverInfo"].(map[string]any)
	if info["name"] != "snow" || info["version"] != "test" {
		t.Fatalf("serverInfo = %v", info)
	}
	if instructions, _ := init["instructions"].(string); !strings.Contains(instructions, "list_panes") {
		t.Fatalf("instructions do not name list_panes: %q", instructions)
	}

	c.send(t, map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	c.send(t, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list"})
	tools, ok := result(t, c.next(t))["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools = %v", tools)
	}
	tool, _ := tools[0].(map[string]any)
	if tool["name"] != "list_panes" {
		t.Fatalf("tool = %v", tool)
	}
	if _, ok := tool["inputSchema"].(map[string]any); !ok {
		t.Fatalf("no input schema: %v", tool)
	}

	c.send(t, map[string]any{"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": map[string]any{
		"name": "list_panes", "arguments": map[string]any{},
	}})
	text, failed := toolText(t, c.next(t))
	if failed {
		t.Fatalf("list_panes failed: %s", text)
	}
	for _, want := range []string{`Session "work"`, "pane 2:", "/src/api", "agent: working"} {
		if !strings.Contains(text, want) {
			t.Errorf("output does not hold %q:\n%s", want, text)
		}
	}
}

// TestMCPOutsideSnow: outside a Snow pane the server offers no tools and
// says why on stderr and in its instructions.
func TestMCPOutsideSnow(t *testing.T) {
	var logs bytes.Buffer
	c := start(t, Options{Version: "test", Log: log.New(&logs, "", 0)})

	c.send(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": "2025-06-18"}})
	init := result(t, c.next(t))
	instructions, _ := init["instructions"].(string)
	if !strings.Contains(instructions, "SNOW_PANE") {
		t.Fatalf("instructions do not say why: %q", instructions)
	}

	c.send(t, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list"})
	tools, ok := result(t, c.next(t))["tools"].([]any)
	if !ok || len(tools) != 0 {
		t.Fatalf("tools = %v, want none", tools)
	}

	c.send(t, map[string]any{"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": map[string]any{
		"name": "list_panes", "arguments": map[string]any{},
	}})
	text, failed := toolText(t, c.next(t))
	if !failed || !strings.Contains(text, "Snow pane") {
		t.Fatalf("call = %q, failed = %v", text, failed)
	}
	if !strings.Contains(logs.String(), "SNOW_PANE") {
		t.Fatalf("stderr does not say why: %q", logs.String())
	}
}

// TestMCPCancel: a cancelled tool call returns before its own work ends.
func TestMCPCancel(t *testing.T) {
	c := start(t, Options{Version: "test", Panes: &fakePanes{block: true}})

	c.send(t, map[string]any{"jsonrpc": "2.0", "id": 7, "method": "tools/call", "params": map[string]any{
		"name": "list_panes", "arguments": map[string]any{},
	}})
	c.send(t, map[string]any{"jsonrpc": "2.0", "method": "notifications/cancelled", "params": map[string]any{"requestId": 7, "reason": "test"}})
	text, failed := toolText(t, c.next(t))
	if !failed || !strings.Contains(text, "canceled") {
		t.Fatalf("call = %q, failed = %v", text, failed)
	}
}

// TestMCPUnknownMethod answers a request Snow does not know with the
// JSON-RPC method-not-found error.
func TestMCPUnknownMethod(t *testing.T) {
	c := start(t, Options{Panes: &fakePanes{}})
	c.send(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "sampling/createMessage"})
	code, _ := rpcFailure(t, c.next(t))["code"].(float64)
	if int(code) != codeMethodNotFound {
		t.Fatalf("code = %v, want %d", code, codeMethodNotFound)
	}
}

// TestMCPParseError answers a line that is not JSON with a parse error and
// a null ID.
func TestMCPParseError(t *testing.T) {
	c := start(t, Options{Panes: &fakePanes{}})
	if _, err := c.in.Write([]byte("not json\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	m := c.next(t)
	if m["id"] != nil {
		t.Fatalf("id = %v, want null", m["id"])
	}
	code, _ := rpcFailure(t, m)["code"].(float64)
	if int(code) != codeParseError {
		t.Fatalf("code = %v, want %d", code, codeParseError)
	}
}

// TestMCPPing answers ping with an empty result.
func TestMCPPing(t *testing.T) {
	c := start(t, Options{Panes: &fakePanes{}})
	c.send(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"})
	if res := result(t, c.next(t)); len(res) != 0 {
		t.Fatalf("result = %v, want {}", res)
	}
}

// TestServeEndsAtEOF: the server stops on its own when the client closes
// its input, after it has written every answer.
func TestServeEndsAtEOF(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")
	if err := Serve(context.Background(), in, &out, Options{Panes: &fakePanes{}}); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if !strings.Contains(out.String(), `"result":{}`) {
		t.Fatalf("output = %q", out.String())
	}
}
