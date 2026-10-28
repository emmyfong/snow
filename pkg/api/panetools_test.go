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
