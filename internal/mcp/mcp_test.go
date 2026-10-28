package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"reflect"
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
	read  *api.ReadPane
	sent  *api.SendKeys
	wait  *api.WaitFor
	err   error
	// block makes List, Read, Send, and Wait wait for the context, for
	// cancellation tests.
	block bool
	// readPane and readScrollback record the last Read request.
	readPane, readScrollback int
	// sendPane, sendText, and sendKeys record the last Send request.
	sendPane int
	sendText string
	sendKeys []api.Key
	// waitPane, waitText, waitQuiet, waitTimeout, and waitExisting record
	// the last Wait request.
	waitPane, waitQuiet, waitTimeout int
	waitText                         string
	waitExisting                     bool
}

func (f *fakePanes) List(ctx context.Context) (*api.Panes, error) {
	if f.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return f.panes, f.err
}

func (f *fakePanes) Read(ctx context.Context, pane, scrollback int) (*api.ReadPane, error) {
	if f.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	f.readPane, f.readScrollback = pane, scrollback
	return f.read, f.err
}

func (f *fakePanes) Send(ctx context.Context, pane int, text string, keys []api.Key) (*api.SendKeys, error) {
	if f.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	f.sendPane, f.sendText, f.sendKeys = pane, text, keys
	if f.sent == nil {
		f.sent = &api.SendKeys{Pane: pane}
	}
	return f.sent, f.err
}

func (f *fakePanes) Wait(ctx context.Context, pane int, text string, quiet, timeout time.Duration, includeExisting bool) (*api.WaitFor, error) {
	if f.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	f.waitPane, f.waitText = pane, text
	f.waitQuiet, f.waitTimeout = int(quiet/time.Millisecond), int(timeout/time.Millisecond)
	f.waitExisting = includeExisting
	if f.wait == nil {
		f.wait = &api.WaitFor{Pane: pane, Result: api.WaitFound, Line: "done"}
	}
	return f.wait, f.err
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
	if !ok || len(tools) != 4 {
		t.Fatalf("tools = %v", tools)
	}
	names := map[string]bool{}
	for i, raw := range tools {
		tool, _ := raw.(map[string]any)
		name, _ := tool["name"].(string)
		names[name] = true
		if _, ok := tool["inputSchema"].(map[string]any); !ok {
			t.Errorf("tool %d has no input schema: %v", i, tool)
		}
	}
	if !names["list_panes"] || !names["read_pane"] || !names["send_keys"] || !names["wait_for"] {
		t.Fatalf("tools = %v, want list_panes, read_pane, send_keys, and wait_for", names)
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

// TestMCPReadPane: read_pane passes the pane and scrollback to Snow and
// writes the copy for the agent.
func TestMCPReadPane(t *testing.T) {
	f := &fakePanes{read: &api.ReadPane{
		Pane: 2, Session: "work", Profile: "bash", Folder: "/src", Cols: 80, Rows: 3,
		CursorX: 4, CursorY: 2, CursorVisible: true,
		Screen:     []string{"$ ls", "a.txt", "$ "},
		Scrollback: []string{"tmp", "src"},
		Omitted:    7,
	}}
	c := start(t, Options{Version: "test", Panes: f})

	c.send(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{
		"name": "read_pane", "arguments": map[string]any{"pane": 2, "scrollback": 10},
	}})
	text, failed := toolText(t, c.next(t))
	if failed {
		t.Fatalf("read_pane failed: %s", text)
	}
	if f.readPane != 2 || f.readScrollback != 10 {
		t.Fatalf("Read(%d, %d), want (2, 10)", f.readPane, f.readScrollback)
	}
	for _, want := range []string{`pane 2`, `"work"`, "a.txt", "tmp\nsrc", "7 older lines", "80x3", "cursor at column 5, row 3"} {
		if !strings.Contains(text, want) {
			t.Errorf("output does not hold %q:\n%s", want, text)
		}
	}
}

// TestMCPReadPaneNeedsPane: read_pane without a pane ID tells the agent how
// to find one.
func TestMCPReadPaneNeedsPane(t *testing.T) {
	c := start(t, Options{Version: "test", Panes: &fakePanes{}})
	c.send(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{
		"name": "read_pane", "arguments": map[string]any{},
	}})
	text, failed := toolText(t, c.next(t))
	if !failed || !strings.Contains(text, "list_panes") {
		t.Fatalf("call = %q, failed = %v; want a hint about list_panes", text, failed)
	}
}

// TestMCPSendKeys: send_keys parses the key names and passes them to Snow
// with the text.
func TestMCPSendKeys(t *testing.T) {
	f := &fakePanes{}
	c := start(t, Options{Version: "test", Panes: f})

	c.send(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{
		"name": "send_keys", "arguments": map[string]any{"pane": 2, "text": "go test", "keys": []string{"enter", "ctrl+c"}},
	}})
	text, failed := toolText(t, c.next(t))
	if failed {
		t.Fatalf("send_keys failed: %s", text)
	}
	if f.sendPane != 2 || f.sendText != "go test" {
		t.Fatalf("Send(pane %d, text %q), want (2, go test)", f.sendPane, f.sendText)
	}
	want := []api.Key{{Code: api.KeyEnter}, {Code: "c", Mod: []string{api.ModCtrl}}}
	if !reflect.DeepEqual(f.sendKeys, want) {
		t.Fatalf("keys = %+v, want %+v", f.sendKeys, want)
	}
	if !strings.Contains(text, "pane 2") {
		t.Errorf("output does not name the pane:\n%s", text)
	}
}

// TestMCPSendKeysDropped: a program that did not read the input is reported
// in the result, with a hint to read the pane.
func TestMCPSendKeysDropped(t *testing.T) {
	f := &fakePanes{sent: &api.SendKeys{Pane: 2, Dropped: 7}}
	c := start(t, Options{Version: "test", Panes: f})
	c.send(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{
		"name": "send_keys", "arguments": map[string]any{"pane": 2, "keys": []string{"enter"}},
	}})
	text, failed := toolText(t, c.next(t))
	if failed {
		t.Fatalf("send_keys failed: %s", text)
	}
	if !strings.Contains(text, "did not read 7 bytes") || !strings.Contains(text, "Read the pane") {
		t.Fatalf("output %q, want the dropped bytes and a hint", text)
	}
}

// TestMCPSendKeysBadKey: a key name ParseKey does not know fails the call
// with a hint at the key grammar, before it reaches Snow.
func TestMCPSendKeysBadKey(t *testing.T) {
	f := &fakePanes{}
	c := start(t, Options{Version: "test", Panes: f})
	c.send(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{
		"name": "send_keys", "arguments": map[string]any{"pane": 2, "keys": []string{"hyper+a"}},
	}})
	text, failed := toolText(t, c.next(t))
	if !failed || !strings.Contains(text, "hyper") || !strings.Contains(text, "modifier") {
		t.Fatalf("call = %q, failed = %v; want a hint about the modifier", text, failed)
	}
	if f.sendPane != 0 {
		t.Fatalf("a bad key name reached Snow: pane %d", f.sendPane)
	}
}

// TestMCPSendKeysNeedsInput: send_keys without text or keys tells the agent
// what it needs.
func TestMCPSendKeysNeedsInput(t *testing.T) {
	c := start(t, Options{Version: "test", Panes: &fakePanes{}})
	c.send(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{
		"name": "send_keys", "arguments": map[string]any{"pane": 2},
	}})
	text, failed := toolText(t, c.next(t))
	if !failed || !strings.Contains(text, "text, keys, or both") {
		t.Fatalf("call = %q, failed = %v; want a hint about text and keys", text, failed)
	}
}

// TestMCPWaitFor: wait_for passes the pane, the text, and the limits to
// Snow and writes why the wait ended.
func TestMCPWaitFor(t *testing.T) {
	f := &fakePanes{wait: &api.WaitFor{Pane: 2, Result: api.WaitFound, Line: "ok  github.com/x  0.2s"}}
	c := start(t, Options{Version: "test", Panes: f})

	c.send(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{
		"name": "wait_for", "arguments": map[string]any{"pane": 2, "text": "ok ", "timeoutMs": 1500, "includeExisting": true},
	}})
	text, failed := toolText(t, c.next(t))
	if failed {
		t.Fatalf("wait_for failed: %s", text)
	}
	if f.waitPane != 2 || f.waitText != "ok " || f.waitQuiet != 0 || f.waitTimeout != 1500 || !f.waitExisting {
		t.Fatalf("Wait = pane %d, text %q, quiet %d, timeout %d, existing %v", f.waitPane, f.waitText, f.waitQuiet, f.waitTimeout, f.waitExisting)
	}
	if !strings.Contains(text, "Pane 2") || !strings.Contains(text, "ok  github.com/x  0.2s") {
		t.Fatalf("output %q, want the pane and the matching line", text)
	}
}

// TestMCPWaitForQuiet: quietMs waits for the pane to stop changing.
func TestMCPWaitForQuiet(t *testing.T) {
	f := &fakePanes{wait: &api.WaitFor{Pane: 2, Result: api.WaitQuiet}}
	c := start(t, Options{Version: "test", Panes: f})
	c.send(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{
		"name": "wait_for", "arguments": map[string]any{"pane": 2, "quietMs": 750},
	}})
	text, failed := toolText(t, c.next(t))
	if failed {
		t.Fatalf("wait_for failed: %s", text)
	}
	if f.waitQuiet != 750 || f.waitText != "" {
		t.Fatalf("Wait = quiet %d, text %q; want 750 and no text", f.waitQuiet, f.waitText)
	}
	if !strings.Contains(text, "stopped changing") {
		t.Fatalf("output %q, want the quiet result", text)
	}
}

// TestMCPWaitForClamp: a request over the cap is clamped before it reaches
// Snow, so it cannot overflow a duration.
func TestMCPWaitForClamp(t *testing.T) {
	f := &fakePanes{}
	c := start(t, Options{Version: "test", Panes: f})
	c.send(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{
		"name": "wait_for", "arguments": map[string]any{"pane": 2, "text": "ok", "timeoutMs": 1 << 40},
	}})
	if _, failed := toolText(t, c.next(t)); failed {
		t.Fatal("wait_for failed")
	}
	if f.waitTimeout != api.WaitMaxMS {
		t.Fatalf("timeout = %d, want the %d ms cap", f.waitTimeout, api.WaitMaxMS)
	}
}

// TestMCPWaitForTimeout: a timeout is a normal result, not a failed call.
func TestMCPWaitForTimeout(t *testing.T) {
	f := &fakePanes{wait: &api.WaitFor{Pane: 2, Result: api.WaitTimeout}}
	c := start(t, Options{Version: "test", Panes: f})
	c.send(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{
		"name": "wait_for", "arguments": map[string]any{"pane": 2, "text": "never"},
	}})
	text, failed := toolText(t, c.next(t))
	if failed {
		t.Fatalf("a timeout was reported as a failed call: %s", text)
	}
	if !strings.Contains(text, "timeout") || !strings.Contains(text, "read the pane") {
		t.Fatalf("output %q, want the timeout and a hint to read the pane", text)
	}
}

// TestMCPWaitForBadArgs: wait_for checks its arguments before it reaches
// Snow and says what it needs.
func TestMCPWaitForBadArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{"no pane", map[string]any{"text": "ok"}, "list_panes"},
		{"no condition", map[string]any{"pane": 2}, "exactly one"},
		{"both conditions", map[string]any{"pane": 2, "text": "ok", "quietMs": 100}, "exactly one"},
		{"negative quiet", map[string]any{"pane": 2, "quietMs": -1}, "zero or more"},
		{"negative timeout", map[string]any{"pane": 2, "text": "ok", "timeoutMs": -1}, "zero or more"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakePanes{}
			c := start(t, Options{Version: "test", Panes: f})
			c.send(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{
				"name": "wait_for", "arguments": tt.args,
			}})
			text, failed := toolText(t, c.next(t))
			if !failed || !strings.Contains(text, tt.want) {
				t.Fatalf("call = %q, failed = %v; want a hint about %q", text, failed, tt.want)
			}
			if f.waitPane != 0 {
				t.Fatalf("a bad request reached Snow: pane %d", f.waitPane)
			}
		})
	}
}

// TestWaitForInterrupt: a cancelled wait_for returns at once and leaves no
// work behind.
func TestWaitForInterrupt(t *testing.T) {
	f := &fakePanes{block: true}
	c := start(t, Options{Version: "test", Panes: f})
	c.send(t, map[string]any{"jsonrpc": "2.0", "id": 9, "method": "tools/call", "params": map[string]any{
		"name": "wait_for", "arguments": map[string]any{"pane": 2, "text": "never"},
	}})
	// Give the call time to reach the fake, then cancel it.
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	c.send(t, map[string]any{"jsonrpc": "2.0", "method": "notifications/cancelled", "params": map[string]any{"requestId": 9, "reason": "test"}})
	text, failed := toolText(t, c.next(t))
	if !failed || !strings.Contains(text, "canceled") {
		t.Fatalf("call = %q, failed = %v", text, failed)
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("a cancelled wait took %v to return", took)
	}
}

// TestFormatWait: every result reads the way an agent can act on.
func TestFormatWait(t *testing.T) {
	exit := 4
	tests := []struct {
		name string
		res  *api.WaitFor
		want string
	}{
		{"found", &api.WaitFor{Pane: 2, Result: api.WaitFound, Line: "$ go test"}, "Pane 2 printed the text:\n$ go test"},
		{"quiet", &api.WaitFor{Pane: 2, Result: api.WaitQuiet}, "Pane 2 stopped changing."},
		{"exited", &api.WaitFor{Pane: 2, Result: api.WaitExited, Exit: &exit}, "status 4"},
		{"exited unknown", &api.WaitFor{Pane: 2, Result: api.WaitExited}, "Pane 2's program exited."},
		{"timeout", &api.WaitFor{Pane: 2, Result: api.WaitTimeout}, "Pane 2 reached the timeout first."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatWait(tt.res); !strings.Contains(got, tt.want) {
				t.Fatalf("formatWait = %q, want %q", got, tt.want)
			}
		})
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
