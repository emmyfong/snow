package convstore

import (
	"encoding/json"
	"testing"
)

func TestRuntimeHistoryKeepsBackendIdentity(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	w, err := store.Create(Meta{Folder: t.TempDir(), Model: "claude-code/default", Runtime: &RuntimeMeta{Kind: "claude-code", Connection: "selected"}})
	if err != nil {
		t.Fatal(err)
	}
	id := w.ID()
	if err := w.AppendRuntime([]RuntimeEvent{
		{Session: "backend-1", User: "Keep the original prompt", Display: json.RawMessage(`{"kind":"turn-started"}`)},
		{Session: "backend-1", Display: json.RawMessage(`{"kind":"text","text":"partial reply"}`)},
	}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(id)
	if err != nil || loaded.Meta.Runtime == nil || loaded.Meta.Runtime.Connection != "selected" {
		t.Fatalf("lost runtime binding: %v", err)
	}
	var events []*RuntimeEvent
	for _, record := range loaded.Records {
		if event, ok := record.(*RuntimeEvent); ok {
			events = append(events, event)
		}
	}
	if len(events) != 2 || events[1].Session != "backend-1" {
		t.Fatalf("backend identity/events changed: %+v", events)
	}
	entries, err := store.ListAll()
	if err != nil || len(entries) != 1 || entries[0].Turns != 1 || entries[0].Title != "Keep the original prompt" {
		t.Fatalf("runtime history index: %+v, %v", entries, err)
	}
}
