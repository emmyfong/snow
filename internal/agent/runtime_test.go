package agent

import (
	"errors"
	"testing"

	"github.com/emmyfong/snow/internal/convstore"
)

func TestDirectAPILoopRefusesExternalConversation(t *testing.T) {
	store, err := convstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	meta := convstore.Meta{Folder: t.TempDir(), Model: "claude-code/default", Runtime: &convstore.RuntimeMeta{Kind: "claude-code", Connection: "selected"}}
	if _, err := Start(Config{Store: store}, meta); !errors.Is(err, ErrInvalid) {
		t.Fatalf("direct loop accepted external metadata: %v", err)
	}
	w, err := store.Create(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(w.ID())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ResumeLoaded(Config{Store: store}, loaded); !errors.Is(err, ErrInvalid) {
		t.Fatalf("direct loop resumed external model history: %v", err)
	}
	claimed, err := store.Writer(w.ID())
	if err != nil {
		t.Fatalf("rejected resume claimed external file: %v", err)
	}
	if err := claimed.Close(); err != nil {
		t.Fatal(err)
	}
}
