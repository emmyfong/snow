package server

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/store"
	"github.com/emmyfong/snow/pkg/api"
)

// persistServer returns a server that saves sessions in a temporary folder,
// with short delays so tests run fast, and that folder's store.
func persistServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	opts := testOptions()
	opts.SessionsDir = t.TempDir()
	opts.saveDelay = 20 * time.Millisecond
	opts.busyEvery = 100 * time.Millisecond
	s := New(opts)
	t.Cleanup(s.Close)
	st, err := store.Open(opts.SessionsDir)
	if err != nil {
		t.Fatal(err)
	}
	return s, st
}

// saved waits until session's snapshot satisfies ok and returns it.
func saved(t *testing.T, st *store.Store, session string, ok func(store.Snapshot) bool) store.Snapshot {
	t.Helper()
	var snap store.Snapshot
	eventually(t, "snapshot of "+session, func() bool {
		var err error
		snap, err = st.Load(session)
		return err == nil && ok(snap)
	})
	return snap
}

func TestChangeIsSaved(t *testing.T) {
	s, st := persistServer(t)
	if _, err := s.CreateSession("a"); err != nil {
		t.Fatal(err)
	}
	snap := saved(t, st, "a", func(store.Snapshot) bool { return true })
	if len(snap.Windows) != 1 || len(snap.Windows[0].Panes) != 1 || snap.Windows[0].Panes[0].Profile != "sh" && snap.Windows[0].Panes[0].Profile != "cmd" {
		t.Fatalf("snapshot %+v, want one window with one shell pane", snap)
	}
	if len(snap.Windows[0].Layout) == 0 {
		t.Fatal("the snapshot has no layout")
	}
}

func TestKillDeletesSnapshot(t *testing.T) {
	s, st := persistServer(t)
	if _, err := s.CreateSession("a"); err != nil {
		t.Fatal(err)
	}
	saved(t, st, "a", func(store.Snapshot) bool { return true })
	if err := s.KillSession("a"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "snapshot deleted", func() bool {
		_, err := st.Load("a")
		return errors.Is(err, fs.ErrNotExist)
	})
}

// TestBusySessionIsSaved: output alone is not a change, but the periodic
// save catches it, so a reboot keeps recent output.
func TestBusySessionIsSaved(t *testing.T) {
	s, st := persistServer(t)
	c := connect(t, s, 80, 24)
	c.Send(&api.Attach{Session: "work", Create: true})
	c.WaitScreen(strings.TrimRight(prompt(), " "))
	c.Send(&api.Input{Pane: 1, Paste: "echo busy" + "mark\r"})
	saved(t, st, "work", func(snap store.Snapshot) bool {
		return strings.Contains(strings.Join(snap.Windows[0].Panes[0].Scrollback, "\n"), "busymark")
	})
}

// TestCloseSavesEverySession: a clean exit, such as a restart, writes every
// session at once, without waiting for the save delay.
func TestCloseSavesEverySession(t *testing.T) {
	opts := testOptions()
	opts.SessionsDir = t.TempDir()
	opts.saveDelay = time.Hour
	opts.busyEvery = time.Hour
	s := New(opts)
	for _, name := range []string{"a", "b"} {
		if _, err := s.CreateSession(name); err != nil {
			t.Fatal(err)
		}
	}
	s.Close()
	st, err := store.Open(opts.SessionsDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b"} {
		if _, err := st.Load(name); err != nil {
			t.Errorf("session %s not saved on close: %v", name, err)
		}
	}
}
