package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func chatMessages() []Message {
	diff := &ChatDiff{Path: "a.go", Old: "x", New: "y"}
	return []Message{
		&ChatSnapshot{
			Pane: 2, Agent: "c1", Conversation: "c1", Title: "Fix it", Model: "anthropic/claude-sonnet-5-5", Effort: "high",
			Mode: "ask", State: ChatStateApproval, Running: true, Turn: 3, Earlier: 4,
			Entries: []ChatEntry{
				{Kind: ChatEntryUser, Turn: 3, Text: "edit a.go"},
				{Kind: ChatEntryAssistant, Turn: 3, Text: "ok", Cut: true},
				{Kind: ChatEntryTool, Turn: 3, Tool: &ChatTool{ID: "t1", Name: "edit", Input: json.RawMessage(`{"path":"a.go"}`), Diff: diff}},
				{Kind: ChatEntryError, Turn: 2, Error: &ChatError{Code: ChatErrRateLimited, Message: "slow down", RetryAfter: 42, Choices: []string{ChatChoiceWait}}},
			},
			Pending: &ChatApproval{ID: "t1", Tool: "edit", Input: json.RawMessage(`{"path":"a.go"}`), Diff: diff, Outside: []string{"/etc"}},
			Queue:   []string{"next"},
			Usage:   &ChatUsage{Requested: "anthropic/claude-sonnet-5-5", Reported: "claude-sonnet-5-5", Input: 10, Output: 5, Cost: 0.01, Priced: true, Window: 200000},
			Cost:    0.42,
		},
		&ChatEvent{Pane: 2, Agent: "c1", Kind: ChatEventText, Turn: 3, Text: "hel"},
		&ChatEvent{Pane: 2, Agent: "c1", Kind: ChatEventTurnEnded, Turn: 3, Reason: ChatEndFailed, Error: &ChatError{Code: ChatErrBadKey, Message: "no", Choices: []string{ChatChoiceLogin}}},
		&ChatEvent{Pane: 2, Agent: "c1", Kind: ChatEventQueue, Queue: []string{"a", "b"}},
		&ChatInput{Pane: 2, Agent: "c1", Kind: ChatInputAnswer, ID: "t1", Decision: ChatDecisionNo, Text: "not that file"},
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
		{"message", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputMessage, Text: "hi"}, true},
		{"answer", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputAnswer, ID: "t", Decision: ChatDecisionAlways}, true},
		{"interrupt", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputInterrupt}, true},
		{"retry", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputRetry}, true},
		{"model", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputModel, Model: "anthropic/claude-opus-5-5", Effort: "high"}, true},
		{"model without effort", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputModel, Model: "openai/gpt-6.1-sol"}, true},
		{"model saved as default", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputModel, Model: "openai/gpt-6.1-sol", SaveDefault: true}, true},
		{"model without a provider", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputModel, Model: "opus"}, false},
		{"unknown effort", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputModel, Model: "openai/gpt-6.1-sol", Effort: "turbo"}, false},
		{"default on a message", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputMessage, Text: "hi", SaveDefault: true}, false},
		{"no pane", ChatInput{Agent: "a", Kind: ChatInputInterrupt}, false},
		{"no agent", ChatInput{Pane: 1, Kind: ChatInputInterrupt}, false},
		{"unknown kind", ChatInput{Pane: 1, Agent: "a", Kind: "dance"}, false},
		{"empty message", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputMessage}, false},
		{"answer without id", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputAnswer, Decision: ChatDecisionYes}, false},
		{"unknown decision", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputAnswer, ID: "t", Decision: "maybe"}, false},
		{"new", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputNew}, true},
		{"resume", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputResume, Conversation: "c1"}, true},
		{"resume of nothing", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputResume}, false},
		{"rename", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputRename, Text: "Fix it"}, true},
		{"rename blank", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputRename, Text: "  "}, false},
		{"rename longest", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputRename, Text: strings.Repeat("é", MaxChatTitle)}, true},
		{"rename too long", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputRename, Text: strings.Repeat("é", MaxChatTitle+1)}, false},
		{"mode ask", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputMode, Mode: ChatModeAsk}, true},
		{"mode auto", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputMode, Mode: ChatModeAuto}, true},
		{"mode unknown", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputMode, Mode: "yolo"}, false},
		{"mode none", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputMode}, false},
		{"effort", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputEffort, Effort: "high"}, true},
		{"effort default", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputEffort}, true},
		{"effort unknown", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputEffort, Effort: "extreme"}, false},
		{"longest message", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputMessage, Text: strings.Repeat("x", MaxChatMessage)}, true},
		{"message too long", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputMessage, Text: strings.Repeat("x", MaxChatMessage+1)}, false},
		{"reason too long", ChatInput{Pane: 1, Agent: "a", Kind: ChatInputAnswer, ID: "t", Decision: ChatDecisionNo, Text: strings.Repeat("x", MaxChatMessage+1)}, false},
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

// TestChatMessageNumbers pins the numbers of the chat messages: a released
// type is never renumbered, or old clients read the wrong message.
func TestChatMessageNumbers(t *testing.T) {
	for _, tt := range []struct {
		got  MsgType
		want MsgType
	}{{TypeChatSnapshot, 22}, {TypeChatEvent, 23}, {TypeChatInput, 24}, {TypeModelListRequest, 25}, {TypeModelList, 26}} {
		if tt.got != tt.want {
			t.Errorf("type is %d, want %d", tt.got, tt.want)
		}
	}
}
