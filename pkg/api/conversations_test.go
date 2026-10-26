package api

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
	"time"
)

// TestConversationListValidate: a request names a chat pane.
func TestConversationListValidate(t *testing.T) {
	if err := (&ConversationListRequest{Pane: 2}).Validate(); err != nil {
		t.Fatal(err)
	}
	err := (&ConversationListRequest{}).Validate()
	if !errors.Is(err, ErrInvalidConversationList) {
		t.Fatalf("no pane: %v", err)
	}
}

// TestConversationListRoundTrip: the list survives a frame, and an empty
// list encodes as [], never null.
func TestConversationListRoundTrip(t *testing.T) {
	when := time.Date(2026, 10, 26, 9, 14, 0, 0, time.UTC)
	for _, m := range []Message{
		&ConversationListRequest{Pane: 2, All: true},
		&ConversationList{Pane: 2, All: true, Items: []ConversationInfo{
			{ID: "c1", Title: "Fix the bug", Folder: "/src", Model: "a/b", LastActivity: when, Turns: 3, Open: true},
			{ID: "c2", Err: "unreadable"},
		}},
	} {
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
	}
	b := mustJSON(t, &ConversationList{Pane: 1})
	if want := `{"pane":1,"items":[]}`; b != want {
		t.Fatalf("empty list is %s, want %s", b, want)
	}
}

// TestConversationMessageNumbers pins the numbers: a released type is
// never renumbered.
func TestConversationMessageNumbers(t *testing.T) {
	if TypeConversationListRequest != 33 || TypeConversationList != 34 {
		t.Fatalf("numbers %d, %d", TypeConversationListRequest, TypeConversationList)
	}
}
