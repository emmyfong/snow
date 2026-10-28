package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

// TestPaneToolsWire pins the JSON of the pane tool messages, like
// TestWireFormat does for the others.
func TestPaneToolsWire(t *testing.T) {
	tests := []struct {
		msg  Message
		want string
	}{
		{&PanesRequest{Caller: 3}, `{"caller":3}`},
		{&Panes{Session: "work", Caller: 3}, `{"session":"work","caller":3,"panes":[]}`},
		{&Panes{Session: "work", Caller: 3, Panes: []PaneSummary{{ID: 1, Window: 0, WindowName: "sh", Shown: true, Profile: "bash", Folder: "/src", Rect: Rect{W: 80, H: 24}, Focused: true}}},
			`{"session":"work","caller":3,"panes":[{"id":1,"window":0,"windowName":"sh","shown":true,"profile":"bash","folder":"/src","rect":{"x":0,"y":0,"w":80,"h":24},"focused":true}]}`},
		{&Panes{Session: "work", Caller: 3, Panes: []PaneSummary{{ID: 3, WindowName: "sh", Profile: "Claude Code", Agent: true, AgentState: AgentWorking, Ended: true}}},
			`{"session":"work","caller":3,"panes":[{"id":3,"window":0,"windowName":"sh","profile":"Claude Code","rect":{"x":0,"y":0,"w":0,"h":0},"agent":true,"agentState":"working","ended":true}]}`},
		{&ReadPaneRequest{Caller: 3, Pane: 2}, `{"caller":3,"pane":2}`},
		{&ReadPaneRequest{Caller: 3, Pane: 2, Scrollback: 10}, `{"caller":3,"pane":2,"scrollback":10}`},
		{&ReadPane{Pane: 2, Session: "work", Profile: "bash", Cols: 80, Rows: 3}, `{"pane":2,"session":"work","profile":"bash","cols":80,"rows":3,"cursorX":0,"cursorY":0,"screen":[]}`},
		{&ReadPane{Pane: 2, Session: "work", Profile: "bash", Cols: 80, Rows: 2, CursorX: 1, CursorY: 1, CursorVisible: true, Folder: "/src", Screen: []string{"$ ls", "a.txt"}, Scrollback: []string{"one"}, Omitted: 4, Ended: true},
			`{"pane":2,"session":"work","profile":"bash","folder":"/src","cols":80,"rows":2,"cursorX":1,"cursorY":1,"cursorVisible":true,"screen":["$ ls","a.txt"],"scrollback":["one"],"omitted":4,"ended":true}`},
		{&SendKeysRequest{Caller: 3, Pane: 2}, `{"caller":3,"pane":2}`},
		{&SendKeysRequest{Caller: 3, Pane: 2, Text: "go test", Keys: []Key{{Code: KeyEnter}}}, `{"caller":3,"pane":2,"text":"go test","keys":[{"code":"enter"}]}`},
		{&SendKeys{Pane: 2}, `{"pane":2}`},
		{&SendKeys{Pane: 2, Dropped: 7}, `{"pane":2,"dropped":7}`},
		{&WaitForRequest{Caller: 3, Pane: 2, Text: "done"}, `{"caller":3,"pane":2,"text":"done"}`},
		{&WaitForRequest{Caller: 3, Pane: 2, QuietMS: 500, TimeoutMS: 1000, IncludeExisting: true}, `{"caller":3,"pane":2,"quietMs":500,"timeoutMs":1000,"includeExisting":true}`},
		{&WaitFor{Pane: 2, Result: WaitFound, Line: "$ go test"}, `{"pane":2,"result":"found","line":"$ go test"}`},
		{&WaitFor{Pane: 2, Result: WaitTimeout}, `{"pane":2,"result":"timeout"}`},
	}
	for _, tt := range tests {
		got, err := json.Marshal(tt.msg)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != tt.want {
			t.Errorf("%T JSON\n got %s\nwant %s", tt.msg, got, tt.want)
		}
	}
}

// TestPaneToolsRoundTrip: each pane tool message survives a frame.
func TestPaneToolsRoundTrip(t *testing.T) {
	msgs := []Message{
		&PanesRequest{Caller: 2},
		&Panes{Session: "a", Caller: 2, Panes: []PaneSummary{{ID: 1, WindowName: "w", Profile: "sh"}}},
		&ReadPaneRequest{Caller: 2, Pane: 1, Scrollback: 5},
		&ReadPane{Pane: 1, Session: "a", Profile: "sh", Cols: 80, Rows: 2, Screen: []string{"$ "}, Scrollback: []string{"x"}},
		&SendKeysRequest{Caller: 2, Pane: 1, Text: "go test", Keys: []Key{{Code: KeyEnter}, {Code: "c", Mod: []string{ModCtrl}}}},
		&SendKeys{Pane: 1, Dropped: 7},
		&WaitForRequest{Caller: 2, Pane: 1, Text: "ok", TimeoutMS: 500, IncludeExisting: true},
		&WaitForRequest{Caller: 2, Pane: 1, QuietMS: 250},
		&WaitFor{Pane: 1, Result: WaitFound, Line: "ok"},
		&WaitFor{Pane: 1, Result: WaitExited, Exit: exitCode(4)},
	}
	for _, m := range msgs {
		var buf bytes.Buffer
		if err := WriteFrame(&buf, m); err != nil {
			t.Fatal(err)
		}
		got, err := ReadFrame(&buf)
		if err != nil {
			t.Fatalf("%T: %v", m, err)
		}
		if !reflect.DeepEqual(got, m) {
			t.Errorf("round trip = %#v, want %#v", got, m)
		}
	}
}

func TestPanesRequestValidate(t *testing.T) {
	if err := (&PanesRequest{Caller: 1}).Validate(); err != nil {
		t.Fatalf("caller 1: %v", err)
	}
	if err := (&PanesRequest{}).Validate(); !errors.Is(err, ErrInvalidPaneTool) {
		t.Fatalf("no caller: %v, want ErrInvalidPaneTool", err)
	}
}

func TestReadPaneRequestValidate(t *testing.T) {
	tests := []struct {
		name string
		req  ReadPaneRequest
		bad  bool
	}{
		{"ok", ReadPaneRequest{Caller: 1, Pane: 2}, false},
		{"scrollback", ReadPaneRequest{Caller: 1, Pane: 2, Scrollback: 50}, false},
		{"no caller", ReadPaneRequest{Pane: 2}, true},
		{"no pane", ReadPaneRequest{Caller: 1}, true},
		{"negative scrollback", ReadPaneRequest{Caller: 1, Pane: 2, Scrollback: -1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if got := errors.Is(err, ErrInvalidPaneTool); got != tt.bad {
				t.Fatalf("Validate() = %v, want an invalid pane tool error: %v", err, tt.bad)
			}
		})
	}
}

func TestSendKeysRequestValidate(t *testing.T) {
	tests := []struct {
		name string
		req  SendKeysRequest
		bad  bool
	}{
		{"text", SendKeysRequest{Caller: 1, Pane: 2, Text: "ls"}, false},
		{"keys", SendKeysRequest{Caller: 1, Pane: 2, Keys: []Key{{Code: KeyEnter}}}, false},
		{"text and keys", SendKeysRequest{Caller: 1, Pane: 2, Text: "ls", Keys: []Key{{Code: KeyEnter}}}, false},
		{"no caller", SendKeysRequest{Pane: 2, Text: "ls"}, true},
		{"no pane", SendKeysRequest{Caller: 1, Text: "ls"}, true},
		{"no input", SendKeysRequest{Caller: 1, Pane: 2}, true},
		{"bad key", SendKeysRequest{Caller: 1, Pane: 2, Keys: []Key{{Code: "A"}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if got := errors.Is(err, ErrInvalidPaneTool); got != tt.bad {
				t.Fatalf("Validate() = %v, want an invalid pane tool error: %v", err, tt.bad)
			}
		})
	}
}

// exitCode returns a pointer to code, for the optional Exit field.
func exitCode(code int) *int { return &code }

func TestWaitForRequestValidate(t *testing.T) {
	tests := []struct {
		name string
		req  WaitForRequest
		bad  bool
	}{
		{"text", WaitForRequest{Caller: 1, Pane: 2, Text: "ok"}, false},
		{"quiet", WaitForRequest{Caller: 1, Pane: 2, QuietMS: 500}, false},
		{"timeout", WaitForRequest{Caller: 1, Pane: 2, Text: "ok", TimeoutMS: 1000}, false},
		{"include existing", WaitForRequest{Caller: 1, Pane: 2, Text: "ok", IncludeExisting: true}, false},
		{"no caller", WaitForRequest{Pane: 2, Text: "ok"}, true},
		{"no pane", WaitForRequest{Caller: 1, Text: "ok"}, true},
		{"no condition", WaitForRequest{Caller: 1, Pane: 2}, true},
		{"text and quiet", WaitForRequest{Caller: 1, Pane: 2, Text: "ok", QuietMS: 500}, true},
		{"negative quiet", WaitForRequest{Caller: 1, Pane: 2, QuietMS: -1}, true},
		{"negative timeout", WaitForRequest{Caller: 1, Pane: 2, Text: "ok", TimeoutMS: -1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if got := errors.Is(err, ErrInvalidPaneTool); got != tt.bad {
				t.Fatalf("Validate() = %v, want an invalid pane tool error: %v", err, tt.bad)
			}
		})
	}
}
