package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestAgentStateValidate(t *testing.T) {
	tests := []struct {
		m  AgentState
		ok bool
	}{
		{AgentState{Pane: 1, State: AgentNeedsYou}, true},
		{AgentState{Pane: 3, State: AgentWorking}, true},
		{AgentState{Pane: 3, State: AgentIdle}, true},
		{AgentState{Pane: 3, State: ""}, true}, // clears
		{AgentState{Pane: 0, State: AgentIdle}, false},
		{AgentState{Pane: 2, State: "sleeping"}, false},
	}
	for _, tt := range tests {
		err := tt.m.Validate()
		if (err == nil) != tt.ok || (err != nil && !errors.Is(err, ErrInvalidAgentState)) {
			t.Errorf("Validate(%+v) = %v, want ok %v", tt.m, err, tt.ok)
		}
	}
}

func TestAgentStateRoundTrip(t *testing.T) {
	m := &AgentState{Pane: 7, State: AgentNeedsYou, At: time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)}
	var buf bytes.Buffer
	if err := WriteFrame(&buf, m); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFrame(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, m) {
		t.Fatalf("round trip = %#v, want %#v", got, m)
	}
	if TypeAgentState != 35 {
		t.Fatalf("TypeAgentState = %d, want 35", TypeAgentState)
	}
}

func TestMoreUrgent(t *testing.T) {
	tests := []struct{ a, b, want string }{
		{AgentIdle, AgentNeedsYou, AgentNeedsYou},
		{AgentWorking, AgentIdle, AgentWorking},
		{AgentNeedsYou, AgentWorking, AgentNeedsYou},
		{"", AgentIdle, AgentIdle},
		{"", "", ""},
	}
	for _, tt := range tests {
		if got := MoreUrgent(tt.a, tt.b); got != tt.want {
			t.Errorf("MoreUrgent(%q, %q) = %q, want %q", tt.a, tt.b, got, tt.want)
		}
	}
}

// TestLayoutAgentIsOptional: a pane or window with no agent adds nothing
// to the wire, so older clients see the layout they knew.
func TestLayoutAgentIsOptional(t *testing.T) {
	b, err := json.Marshal(WindowInfo{Index: 1, Name: "sh", Panes: []PaneInfo{{ID: 1, Kind: PaneTerminal}}})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("agent")) {
		t.Fatalf("encoded %s, want no agent field", b)
	}
}
