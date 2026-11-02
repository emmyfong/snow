package api

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// TestAgentListWire pins the JSON of the agent list: other clients read
// it, and an empty list is [], never null.
func TestAgentListWire(t *testing.T) {
	when := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		msg  *AgentList
		want string
	}{
		{&AgentList{}, `{"agents":[]}`},
		{&AgentList{Agents: []AgentInfo{{Session: "api", Window: 1, WindowName: "claude", Pane: 3, Kind: AgentKindClaudeCode, Since: when}}},
			`{"agents":[{"session":"api","window":1,"windowName":"claude","pane":3,"kind":"claude-code","since":"2026-10-27T09:00:00Z"}]}`},
		{&AgentList{Agents: []AgentInfo{{Session: "api", Pane: 4, Kind: AgentKindChat, Folder: "/src", State: AgentNeedsYou, Since: when, Waits: AgentWaitsApproval, Model: "a/b", Cost: 0.5}}},
			`{"agents":[{"session":"api","window":0,"windowName":"","pane":4,"kind":"chat","folder":"/src","state":"needs-you","since":"2026-10-27T09:00:00Z","waits":"approval","model":"a/b","cost":0.5}]}`},
	}
	for _, tt := range tests {
		got, err := json.Marshal(tt.msg)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != tt.want {
			t.Errorf("JSON\n got %s\nwant %s", got, tt.want)
		}
	}
}

// TestAgentListRoundTrip: the list survives a frame.
func TestAgentListRoundTrip(t *testing.T) {
	sent := &AgentList{Agents: []AgentInfo{{Session: "api", Pane: 2, Kind: AgentKindChat, State: AgentWorking, Since: time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)}}}
	var buf bytes.Buffer
	if err := WriteFrame(&buf, sent); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFrame(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, sent) {
		t.Fatalf("got %+v, want %+v", got, sent)
	}
}
