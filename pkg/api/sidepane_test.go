package api

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestCheckKind(t *testing.T) {
	tests := []struct {
		name string
		ok   bool
	}{
		{"files", true},
		{"git-log", true},
		{"pane2", true},
		{"", false},
		{"Files", false},
		{"2files", false},
		{"-files", false},
		{"files pane", false},
		{"files_pane", false},
		{strings.Repeat("a", MaxKindName), true},
		{strings.Repeat("a", MaxKindName+1), false},
	}
	for _, tt := range tests {
		if err := CheckKind(tt.name); (err == nil) != tt.ok || (err != nil && !errors.Is(err, ErrInvalidKind)) {
			t.Errorf("CheckKind(%q) = %v, want ok=%v", tt.name, err, tt.ok)
		}
	}
}

// noteState is a side pane state for these tests.
type noteState struct {
	Text string `json:"text"`
}

func (noteState) Kind() string { return "note" }

func (n noteState) Validate() error {
	if n.Text == "bad" {
		return errors.New("bad note")
	}
	return nil
}

// otherState is a state of another kind.
type otherState struct{}

func (otherState) Kind() string    { return "other" }
func (otherState) Validate() error { return nil }

func TestSideStateRoundTrip(t *testing.T) {
	msg, err := NewSidePaneState(4, 2, noteState{Text: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if err := msg.Validate(); err != nil {
		t.Fatal(err)
	}
	if msg.Kind != "note" || msg.Pane != 4 || msg.Rev != 2 {
		t.Fatalf("envelope %+v", msg)
	}
	var got noteState
	if err := DecodeSideState(msg, &got); err != nil || got.Text != "hi" {
		t.Fatalf("decoded %+v, %v", got, err)
	}
	if err := DecodeSideState(msg, &otherState{}); !errors.Is(err, ErrInvalidSidePane) {
		t.Fatalf("decode as another kind: %v", err)
	}
	if _, err := NewSidePaneState(4, 3, noteState{Text: "bad"}); err == nil {
		t.Fatal("an invalid state was wrapped")
	}
}

func TestSidePaneValidate(t *testing.T) {
	tests := []struct {
		name string
		msg  interface{ Validate() error }
		ok   bool
	}{
		{"state", &SidePaneState{Pane: 1, Kind: "note", State: json.RawMessage(`{}`)}, true},
		{"state no pane", &SidePaneState{Kind: "note", State: json.RawMessage(`{}`)}, false},
		{"state bad kind", &SidePaneState{Pane: 1, Kind: "Note", State: json.RawMessage(`{}`)}, false},
		{"state not JSON", &SidePaneState{Pane: 1, Kind: "note", State: json.RawMessage(`{`)}, false},
		{"action", &SidePaneAction{Pane: 1, Action: "open"}, true},
		{"action no pane", &SidePaneAction{Action: "open"}, false},
		{"action no name", &SidePaneAction{Pane: 1}, false},
	}
	for _, tt := range tests {
		err := tt.msg.Validate()
		if (err == nil) != tt.ok || (err != nil && !errors.Is(err, ErrInvalidSidePane)) {
			t.Errorf("%s: Validate() = %v, want ok=%v", tt.name, err, tt.ok)
		}
	}
	args := map[string]string{}
	for i := range MaxSideArgs + 1 {
		args[strings.Repeat("k", i+1)] = ""
	}
	if err := (&SidePaneAction{Pane: 1, Action: "open", Args: args}).Validate(); !errors.Is(err, ErrInvalidSidePane) {
		t.Errorf("too many args: %v", err)
	}
}
