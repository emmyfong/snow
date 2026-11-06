package clauderuntime

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/pkg/api"
)

// TestRecorderKeepsUpWithStreamedText: a reply or its thinking streamed in
// many small pieces does not fill the save queue, which would end the run,
// and the saved history still rebuilds it whole and in order.
func TestRecorderKeepsUpWithStreamedText(t *testing.T) {
	for _, kind := range []struct {
		event string
		entry string
	}{{api.ChatEventText, api.ChatEntryAssistant}, {api.ChatEventThinking, api.ChatEntryThinking}} {
		t.Run(kind.event, func(t *testing.T) { recordStreamed(t, kind.event, kind.entry) })
	}
}

func recordStreamed(t *testing.T, kind, entry string) {
	store, err := convstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	w, err := store.Create(convstore.Meta{Folder: t.TempDir(), Model: "claude-code/default",
		Runtime: &convstore.RuntimeMeta{Kind: "claude-code", Connection: "selected"}})
	if err != nil {
		t.Fatal(err)
	}
	id := w.ID()
	r := newRecorder(w)
	if err := r.append(api.ChatEvent{Kind: api.ChatEventTurnStarted, Pane: 1, Turn: 1, Text: "go"}, "s"); err != nil {
		t.Fatal(err)
	}
	const pieces = 5000
	for i := range pieces {
		if err := r.append(api.ChatEvent{Kind: kind, Pane: 1, Text: string(rune('a' + i%26))}, "s"); err != nil {
			t.Fatalf("piece %d: %v", i, err)
		}
	}
	if err := r.append(api.ChatEvent{Kind: api.ChatEventTurnEnded, Pane: 1, Turn: 1, Reason: api.ChatEndDone}, "s"); err != nil {
		t.Fatal(err)
	}
	if err := r.close(); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := api.ChatSnapshot{Pane: 1}
	for _, record := range loaded.Records {
		if record, ok := record.(*convstore.RuntimeEvent); ok {
			var event api.ChatEvent
			if json.Unmarshal(record.Display, &event) != nil {
				t.Fatal("invalid saved display event")
			}
			snapshot.Apply(&event)
		}
	}
	var want strings.Builder
	for i := range pieces {
		want.WriteRune(rune('a' + i%26))
	}
	var got string
	for _, e := range snapshot.Entries {
		if e.Kind == entry {
			got += e.Text
		}
	}
	if got != want.String() {
		t.Fatalf("saved reply has %d characters, want %d in order", len(got), pieces)
	}
}
