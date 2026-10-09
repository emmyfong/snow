package api

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// TestWireFormat pins the exact JSON of every message. Other clients
// implement this format, so a change here is a protocol change.
func TestWireFormat(t *testing.T) {
	when := time.Date(2026, 10, 9, 9, 14, 0, 0, time.UTC)
	tests := []struct {
		msg  Message
		want string
	}{
		{&Hello{ClientVersion: "0.1.0", Protocol: 1, Cols: 80, Rows: 24}, `{"clientVersion":"0.1.0","protocol":1,"cols":80,"rows":24}`},
		{&Welcome{ServerVersion: "0.1.0", Protocol: 1}, `{"serverVersion":"0.1.0","protocol":1}`},
		{&Attach{Session: "api"}, `{"session":"api"}`},
		{&Attach{Session: "api", Create: true}, `{"session":"api","create":true}`},
		{&Input{Pane: 3, Key: &Key{Code: KeyUp}}, `{"pane":3,"key":{"code":"up"}}`},
		{&Input{Pane: 3, Paste: "hi"}, `{"pane":3,"paste":"hi"}`},
		{&Mouse{Pane: 1, X: 4, Y: 2, Button: ButtonLeft, Action: ActionPress}, `{"pane":1,"x":4,"y":2,"button":"left","action":"press"}`},
		{&Resize{Cols: 80, Rows: 24}, `{"cols":80,"rows":24}`},
		{&Command{Op: "zoom"}, `{"op":"zoom"}`},
		{&Command{Op: "split", Target: 2, Args: map[string]string{"dir": "right"}}, `{"op":"split","target":2,"args":{"dir":"right"}}`},
		{&Layout{Session: "api", Active: 1}, `{"session":"api","active":1,"windows":[]}`},
		{&Layout{Session: "api", Active: 1, Windows: []WindowInfo{{Index: 1, Name: "dev"}}}, `{"session":"api","active":1,"windows":[{"index":1,"name":"dev","panes":[]}]}`},
		{&PaneUpdate{Pane: 1}, `{"pane":1,"lines":[]}`},
		{&PaneUpdate{Pane: 1, Lines: []LineUpdate{{Row: 0, Text: "a"}}, Cursor: &Cursor{X: 1, Visible: true}}, `{"pane":1,"lines":[{"row":0,"text":"a"}],"cursor":{"x":1,"y":0,"visible":true}}`},
		{&SessionList{}, `{"sessions":[]}`},
		{&SessionList{Sessions: []SessionInfo{{Name: "api", Windows: 1, State: StateSaved, LastUsed: when}}}, `{"sessions":[{"name":"api","windows":1,"attached":0,"state":"saved","lastUsed":"2026-10-09T09:14:00Z"}]}`},
		{&Bell{Pane: 2}, `{"pane":2}`},
		{&Error{Code: CodeBadHandshake, Message: "expected hello"}, `{"code":"bad-handshake","message":"expected hello"}`},
	}
	for _, tt := range tests {
		t.Run(reflect.TypeOf(tt.msg).Elem().Name(), func(t *testing.T) {
			got, err := json.Marshal(tt.msg)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Fatalf("JSON\n got %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestInputValidate(t *testing.T) {
	tests := []struct {
		name string
		in   Input
		ok   bool
	}{
		{"key", Input{Pane: 1, Key: &Key{Code: KeyEnter}}, true},
		{"paste", Input{Pane: 1, Paste: "x"}, true},
		{"neither", Input{Pane: 1}, false},
		{"both", Input{Pane: 1, Key: &Key{Code: "a", Text: "a"}, Paste: "x"}, false},
		{"invalid key", Input{Pane: 1, Key: &Key{Code: "nope"}}, false},
		{"no pane", Input{Key: &Key{Code: KeyEnter}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.in.Validate(); (err == nil) != tt.ok {
				t.Fatalf("Validate() = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}
