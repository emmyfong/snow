package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func chatMessages() []Message {
	diff := &ChatDiff{Path: "a.go", Old: "x", New: "y"}
	return []Message{
		&ChatSnapshot{
			Pane: 2, Agent: "c1", Conversation: "c1", Title: "Fix it", Model: "anthropic/claude-sonnet-5-5", Effort: "high",
			Mode: "ask", State: ChatStateApproval, Running: true, Turn: 3, Earlier: 4,
			Entries: []ChatEntry{
				{Kind: EntryUser, Turn: 3, Text: "edit a.go"},
				{Kind: EntryAssistant, Turn: 3, Text: "ok", Cut: true},
				{Kind: EntryTool, Turn: 3, Tool: &ChatTool{ID: "t1", Name: "edit", Input: json.RawMessage(`{"path":"a.go"}`), Diff: diff}},
				{Kind: EntryError, Turn: 2, Error: &ChatError{Code: ChatErrRateLimited, Message: "slow down", RetryAfter: 42, Choices: []string{ChatChoiceWait}}},
			},
			Pending: &ChatApproval{ID: "t1", Tool: "edit", Input: json.RawMessage(`{"path":"a.go"}`), Diff: diff, Outside: []string{"/etc"}},
			Queue:   []string{"next"},
			Usage:   &ChatUsage{Requested: "anthropic/claude-sonnet-5-5", Reported: "claude-sonnet-5-5", Input: 10, Output: 5, Cost: 0.01, Priced: true, Window: 200000},
			Cost:    0.42,
		},
		&ChatEvent{Pane: 2, Agent: "c1", Kind: EventText, Turn: 3, Text: "hel"},
		&ChatEvent{Pane: 2, Agent: "c1", Kind: EventTurnEnded, Turn: 3, Reason: EndFailed, Error: &ChatError{Code: ChatErrBadKey, Message: "no", Choices: []string{ChatChoiceLogin}}},
		&ChatEvent{Pane: 2, Agent: "c1", Kind: EventQueue, Queue: []string{"a", "b"}},
		&ChatInput{Pane: 2, Agent: "c1", Kind: InputAnswer, ID: "t1", Decision: DecisionNo, Text: "not that file"},
	}
}

// TestChatMessagesRoundTrip: each chat message survives a frame, agent ID
// included.
func TestChatMessagesRoundTrip(t *testing.T) {
	for _, m := range chatMessages() {
		t.Run(reflect.TypeOf(m).Elem().Name(), func(t *testing.T) {
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
			if reflect.ValueOf(got).Elem().FieldByName("Agent").String() != "c1" {
				t.Fatal("agent ID lost")
			}
		})
	}
}

// TestChatSnapshotArraysNeverNull: an empty snapshot writes [] for its
// arrays.
func TestChatSnapshotArraysNeverNull(t *testing.T) {
	got, err := json.Marshal(&ChatSnapshot{Pane: 1, Agent: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte(`"entries":[]`)) || !bytes.Contains(got, []byte(`"queue":[]`)) {
		t.Fatalf("JSON %s", got)
	}
}

func TestChatInputValidate(t *testing.T) {
	tests := []struct {
		name string
		in   ChatInput
		ok   bool
	}{
		{"message", ChatInput{Pane: 1, Agent: "a", Kind: InputMessage, Text: "hi"}, true},
		{"answer", ChatInput{Pane: 1, Agent: "a", Kind: InputAnswer, ID: "t", Decision: DecisionAlways}, true},
		{"interrupt", ChatInput{Pane: 1, Agent: "a", Kind: InputInterrupt}, true},
		{"retry", ChatInput{Pane: 1, Agent: "a", Kind: InputRetry}, true},
		{"no pane", ChatInput{Agent: "a", Kind: InputInterrupt}, false},
		{"no agent", ChatInput{Pane: 1, Kind: InputInterrupt}, false},
		{"unknown kind", ChatInput{Pane: 1, Agent: "a", Kind: "dance"}, false},
		{"empty message", ChatInput{Pane: 1, Agent: "a", Kind: InputMessage}, false},
		{"answer without id", ChatInput{Pane: 1, Agent: "a", Kind: InputAnswer, Decision: DecisionYes}, false},
		{"unknown decision", ChatInput{Pane: 1, Agent: "a", Kind: InputAnswer, ID: "t", Decision: "maybe"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.Validate()
			if (err == nil) != tt.ok {
				t.Fatalf("Validate() = %v, want ok=%v", err, tt.ok)
			}
			if err != nil && !errors.Is(err, ErrInvalidChatInput) {
				t.Fatalf("error %v is not ErrInvalidChatInput", err)
			}
		})
	}
}

// TestUnknownChatEventKindIgnored: an event kind from a newer server
// decodes and leaves the snapshot as it was.
func TestUnknownChatEventKindIgnored(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte(`{"pane":1,"agent":"a","kind":"question","text":"which?","options":[{"label":"x"}]}`)
	buf.Write(rawFrame(uint32(len(payload)), byte(TypeChatEvent), payload))
	m, err := ReadFrame(&buf)
	if err != nil {
		t.Fatal(err)
	}
	s := &ChatSnapshot{Pane: 1, Agent: "a", State: ChatStateIdle}
	want := *s
	s.Apply(m.(*ChatEvent))
	if !reflect.DeepEqual(*s, want) {
		t.Fatalf("snapshot changed: %+v", s)
	}
}

// TestHandshakeRejectsVersionSix: a version 6 peer and a version 7 peer
// see the mismatch, in both directions.
func TestHandshakeRejectsVersionSix(t *testing.T) {
	if ProtocolVersion != 7 {
		t.Fatalf("ProtocolVersion = %d, want 7", ProtocolVersion)
	}
	for _, v := range []struct{ client, server int }{{6, 7}, {7, 6}} {
		client, server := pair(t)
		res := runServer(server, Welcome{ServerVersion: "s", Protocol: v.server})
		if _, err := ClientHandshake(client, Hello{ClientVersion: "c", Protocol: v.client}); !errors.Is(err, ErrVersionMismatch) {
			t.Fatalf("client %d, server %d: client error %v", v.client, v.server, err)
		}
		if r := <-res; !errors.Is(r.err, ErrVersionMismatch) {
			t.Fatalf("client %d, server %d: server error %v", v.client, v.server, r.err)
		}
	}
}
