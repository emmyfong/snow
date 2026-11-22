package server

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/pkg/api"
)

// TestConvWorkerResolveIsThrottled: a pane with no conversation does not
// list the whole store on every poll. It looks again on a slower timer, and
// a forced read looks at once. The interval is long and the test moves the
// last lookup back by hand, so slow file work on Windows cannot end it early.
func TestConvWorkerResolveIsThrottled(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "conversations")
	store, err := convstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	folder := t.TempDir()
	w := &convWorker[api.UsageState]{
		store: store, folder: folder, profile: "sh",
		resolveEvery: time.Hour,
		build: func(*convstore.Conversation) api.UsageState {
			return api.UsageState{Conversation: "found"}
		},
		noConv: func(st api.UsageState) api.UsageState {
			st.Conversation, st.Loading = "", false
			return st
		},
		fail: func(st api.UsageState, err error) api.UsageState {
			st.Error = err.Error()
			return st
		},
		apply: func(api.UsageState) {},
	}
	if st := w.read(true); st.Conversation != "" {
		t.Fatalf("state %+v; want no conversation yet", st)
	}

	writer, err := store.Create(convstore.Meta{
		ID: "conv-1", Folder: folder, Profile: "sh",
		Model: "anthropic/claude-sonnet-4-5",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Append(&convstore.User{At: time.Now(), Text: "hi"}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	// A poll right after the miss must not list the store again.
	if st := w.read(false); st.Conversation != "" {
		t.Fatalf("state %+v; want the interval to hold off the lookup", st)
	}
	w.resolved = w.resolved.Add(-w.resolveEvery)
	if st := w.read(false); st.Conversation != "found" {
		t.Fatalf("state %+v; want a lookup once the interval passed", st)
	}
}
