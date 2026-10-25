package api

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
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
		{&PaneUpdate{Pane: 1, Mouse: true}, `{"pane":1,"lines":[],"mouse":true}`},
		{&SessionList{}, `{"sessions":[]}`},
		{&SessionList{Sessions: []SessionInfo{{Name: "api", Windows: 1, State: StateSaved, LastUsed: when}}}, `{"sessions":[{"name":"api","windows":1,"attached":0,"state":"saved","lastUsed":"2026-10-09T09:14:00Z"}]}`},
		{&Bell{Pane: 2}, `{"pane":2}`},
		{&ScrollbackRequest{Pane: 1, From: 40, Count: 24}, `{"pane":1,"from":40,"count":24}`},
		{&Scrollback{Pane: 1, Total: 3}, `{"pane":1,"from":0,"total":3,"lines":[]}`},
		{&Scrollback{Pane: 1, From: 1, Total: 3, Lines: []string{"a"}}, `{"pane":1,"from":1,"total":3,"lines":["a"]}`},
		{&SearchRequest{Pane: 1, Query: "error"}, `{"pane":1,"query":"error"}`},
		{&SearchResult{Pane: 1, Query: "x"}, `{"pane":1,"query":"x","matches":[]}`},
		{&Detach{}, `{}`},
		{&Shutdown{}, `{}`},
		{&KillSession{Session: "api"}, `{"session":"api"}`},
		{&RenameSession{Session: "api", Name: "web"}, `{"session":"api","name":"web"}`},
		{&SearchResult{Pane: 1, Query: "x", Matches: []Match{{Line: 3, Col: 4, Len: 1}}, Truncated: true}, `{"pane":1,"query":"x","matches":[{"line":3,"col":4,"len":1}],"truncated":true}`},
		{&ProfileList{}, `{"profiles":[],"default":""}`},
		{&ProfileList{Profiles: []string{"pwsh", "Ubuntu"}, Default: "pwsh"}, `{"profiles":["pwsh","Ubuntu"],"default":"pwsh"}`},
		{&Layout{Session: "api", Active: 1, Windows: []WindowInfo{{Index: 1, Name: "dev", Panes: []PaneInfo{{ID: 2, Profile: "sh", Folder: "/src"}}}}}, `{"session":"api","active":1,"windows":[{"index":1,"name":"dev","panes":[{"id":2,"kind":"","rect":{"x":0,"y":0,"w":0,"h":0},"profile":"sh","folder":"/src"}]}]}`},
		{&ChatSnapshot{Pane: 2, Agent: "c1", Conversation: "x", Model: "a/b", Mode: "ask", State: ChatStateIdle}, `{"pane":2,"agent":"c1","conversation":"x","model":"a/b","mode":"ask","state":"idle","entries":[],"queue":[]}`},
		{&ChatEvent{Pane: 2, Agent: "c1", Kind: EventText, Text: "hi"}, `{"pane":2,"agent":"c1","kind":"text","text":"hi"}`},
		{&ChatInput{Pane: 2, Agent: "c1", Kind: InputMessage, Text: "go"}, `{"pane":2,"agent":"c1","kind":"message","text":"go"}`},
		{&ChatInput{Pane: 2, Agent: "c1", Kind: InputAnswer, ID: "t1", Decision: DecisionNo, Text: "why"}, `{"pane":2,"agent":"c1","kind":"answer","text":"why","id":"t1","decision":"no"}`},
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

func TestScrollbackRequestValidate(t *testing.T) {
	tests := []struct {
		name string
		r    ScrollbackRequest
		ok   bool
	}{
		{"first page", ScrollbackRequest{Pane: 1, From: 0, Count: 24}, true},
		{"largest page", ScrollbackRequest{Pane: 1, Count: MaxScrollbackLines}, true},
		{"no pane", ScrollbackRequest{Count: 1}, false},
		{"negative from", ScrollbackRequest{Pane: 1, From: -1, Count: 1}, false},
		{"empty", ScrollbackRequest{Pane: 1}, false},
		{"too many", ScrollbackRequest{Pane: 1, Count: MaxScrollbackLines + 1}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.r.Validate()
			if (err == nil) != tt.ok {
				t.Fatalf("Validate() = %v, want ok=%v", err, tt.ok)
			}
			if err != nil && !errors.Is(err, ErrInvalidHistory) {
				t.Fatalf("error %v does not wrap ErrInvalidHistory", err)
			}
		})
	}
}

func TestSearchRequestValidate(t *testing.T) {
	tests := []struct {
		name string
		r    SearchRequest
		ok   bool
	}{
		{"query", SearchRequest{Pane: 1, Query: "error"}, true},
		{"longest query", SearchRequest{Pane: 1, Query: strings.Repeat("a", MaxQueryBytes)}, true},
		{"no pane", SearchRequest{Query: "x"}, false},
		{"empty query", SearchRequest{Pane: 1}, false},
		{"query too long", SearchRequest{Pane: 1, Query: strings.Repeat("a", MaxQueryBytes+1)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.r.Validate()
			if (err == nil) != tt.ok {
				t.Fatalf("Validate() = %v, want ok=%v", err, tt.ok)
			}
			if err != nil && !errors.Is(err, ErrInvalidHistory) {
				t.Fatalf("error %v does not wrap ErrInvalidHistory", err)
			}
		})
	}
}

func TestMouseValidate(t *testing.T) {
	ok := Mouse{Pane: 1, X: 0, Y: 3, Button: ButtonLeft, Action: ActionPress}
	tests := []struct {
		name string
		edit func(*Mouse)
		ok   bool
	}{
		{"valid", func(*Mouse) {}, true},
		{"wheel", func(m *Mouse) { m.Button = ButtonWheelUp }, true},
		{"motion with no button", func(m *Mouse) { m.Button, m.Action = ButtonNone, ActionMotion }, true},
		{"no pane", func(m *Mouse) { m.Pane = 0 }, false},
		{"negative x", func(m *Mouse) { m.X = -1 }, false},
		{"negative y", func(m *Mouse) { m.Y = -1 }, false},
		{"unknown button", func(m *Mouse) { m.Button = "back" }, false},
		{"unknown action", func(m *Mouse) { m.Action = "hover" }, false},
	}
	for _, tt := range tests {
		m := ok
		tt.edit(&m)
		err := m.Validate()
		if (err == nil) != tt.ok || (err != nil && !errors.Is(err, ErrInvalidMouse)) {
			t.Errorf("%s: Validate() = %v, want ok=%v", tt.name, err, tt.ok)
		}
	}
}
