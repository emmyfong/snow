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

// TestOldClientIgnoresField: a client built before NeedsYou existed still
// reads the session list, and a session where no agent waits sends no new
// field at all.
func TestOldClientIgnoresField(t *testing.T) {
	when := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	sent := &SessionList{Sessions: []SessionInfo{
		{Name: "api", Windows: 2, State: StateRunning, LastUsed: when, NeedsYou: 2},
		{Name: "docs", Windows: 1, State: StateRunning, LastUsed: when},
	}}
	if err := WriteFrame(&buf, sent); err != nil {
		t.Fatal(err)
	}
	payload := buf.Bytes()[5:] // past the length and the type
	type oldInfo struct {
		Name     string    `json:"name"`
		Windows  int       `json:"windows"`
		Attached int       `json:"attached"`
		State    string    `json:"state"`
		LastUsed time.Time `json:"lastUsed"`
	}
	var old struct {
		Sessions []oldInfo `json:"sessions"`
	}
	if err := json.Unmarshal(payload, &old); err != nil {
		t.Fatalf("an old client cannot read the list: %v", err)
	}
	if len(old.Sessions) != 2 || old.Sessions[0].Name != "api" || old.Sessions[1].Windows != 1 {
		t.Fatalf("old client read %+v", old)
	}
	if b := mustJSON(t, &sent.Sessions[1]); bytes.Contains([]byte(b), []byte("needsYou")) {
		t.Fatalf("a session with no waiting agent encodes %s", b)
	}
}
