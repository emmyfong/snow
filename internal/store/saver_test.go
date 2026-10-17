package store

import (
	"errors"
	"io/fs"
	"os"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

const delay = 2 * time.Second

// countingStore is a real Store that counts the saves that reach it.
type countingStore struct {
	*Store
	mu    sync.Mutex
	saves int
}

func (c *countingStore) Save(s Snapshot) error {
	c.mu.Lock()
	c.saves++
	c.mu.Unlock()
	return c.Store.Save(s)
}

func (c *countingStore) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saves
}

// newSaver returns a Saver over st that fails the test on any error. Close
// it inside the synctest bubble: the bubble waits for its goroutine.
func newSaver(t *testing.T, st Writer) *Saver {
	t.Helper()
	return NewSaver(st, delay, func(name string, err error) {
		if err != nil {
			t.Errorf("save %q failed: %v", name, err)
		}
	})
}

func exists(t *testing.T, st *Store, name string) bool {
	t.Helper()
	_, err := st.Load(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

// synctest runs the test on a fake clock: Sleep returns as soon as every
// goroutine in the bubble is blocked, so the 3-second test takes no real time.
func TestDebounce(t *testing.T) {
	st := &countingStore{Store: openStore(t)}
	synctest.Test(t, func(t *testing.T) {
		s := newSaver(t, st)
		for i := range 10 {
			snap := sample("api")
			snap.Active = i
			s.Changed(snap)
			time.Sleep(100 * time.Millisecond)
		}

		// The last change was at 0.9 s, so the write is due at 2.9 s.
		time.Sleep(1800 * time.Millisecond)
		synctest.Wait()
		if n := st.count(); n != 0 {
			t.Fatalf("%d writes before the delay passed, want 0", n)
		}

		time.Sleep(200 * time.Millisecond)
		synctest.Wait()
		if n := st.count(); n != 1 {
			t.Fatalf("ten changes in one second caused %d writes, want 1", n)
		}
		got, err := st.Load("api")
		if err != nil {
			t.Fatal(err)
		}
		if got.Active != 9 {
			t.Fatalf("saved change %d, want the last change, 9", got.Active)
		}
		_ = s.Close()
	})
}

func TestSaverDeleteCancelsPending(t *testing.T) {
	st := &countingStore{Store: openStore(t)}
	synctest.Test(t, func(t *testing.T) {
		s := newSaver(t, st)
		s.Changed(sample("api"))
		s.Delete("api")
		time.Sleep(2 * delay)
		synctest.Wait()
		if n := st.count(); n != 0 {
			t.Fatalf("%d writes after Delete, want 0", n)
		}
		if exists(t, st.Store, "api") {
			t.Fatal("snapshot exists after Delete")
		}
		_ = s.Close()
	})
}

func TestSaverDeleteRemovesSaved(t *testing.T) {
	st := &countingStore{Store: openStore(t)}
	synctest.Test(t, func(t *testing.T) {
		s := newSaver(t, st)
		s.Changed(sample("api"))
		time.Sleep(2 * delay)
		synctest.Wait()
		if !exists(t, st.Store, "api") {
			t.Fatal("snapshot missing after the delay")
		}
		s.Delete("api")
		if err := s.Flush(); err != nil {
			t.Fatal(err)
		}
		if exists(t, st.Store, "api") {
			t.Fatal("snapshot exists after Delete")
		}
		_ = s.Close()
	})
}

func TestSaverFlush(t *testing.T) {
	st := &countingStore{Store: openStore(t)}
	synctest.Test(t, func(t *testing.T) {
		s := newSaver(t, st)
		s.Changed(sample("api"))
		s.Changed(sample("web"))
		if err := s.Flush(); err != nil {
			t.Fatal(err)
		}
		if n := st.count(); n != 2 {
			t.Fatalf("Flush made %d writes, want 2", n)
		}
		time.Sleep(2 * delay)
		synctest.Wait()
		if n := st.count(); n != 2 {
			t.Fatalf("%d writes after Flush and the delay, want still 2", n)
		}
		_ = s.Close()
	})
}

func TestSaverReportsErrors(t *testing.T) {
	st := openStore(t)
	if err := os.RemoveAll(st.dir); err != nil {
		t.Fatal(err)
	}
	synctest.Test(t, func(t *testing.T) {
		var mu sync.Mutex
		var failed []string
		s := NewSaver(st, delay, func(name string, err error) {
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed = append(failed, name)
			}
		})
		s.Changed(sample("api"))
		time.Sleep(2 * delay)
		synctest.Wait()
		mu.Lock()
		got := slices.Clone(failed)
		mu.Unlock()
		if len(got) != 1 || got[0] != "api" {
			t.Fatalf("report got failures %q, want one for \"api\"", got)
		}
		_ = s.Close()
	})
}

// TestSaverKeepsCallOrder: a session killed and made again under the same
// name keeps the new file; the old delete runs first.
func TestSaverKeepsCallOrder(t *testing.T) {
	st := &countingStore{Store: openStore(t)}
	synctest.Test(t, func(t *testing.T) {
		s := newSaver(t, st)
		s.Changed(sample("api"))
		time.Sleep(2 * delay)
		synctest.Wait()
		s.Delete("api")
		s.Changed(sample("api"))
		time.Sleep(2 * delay)
		synctest.Wait()
		if !exists(t, st.Store, "api") {
			t.Fatal("the new session's save was undone by the old delete")
		}
		_ = s.Close()
	})
}

// slowWriter blocks every Save until release is closed.
type slowWriter struct {
	*Store
	release chan struct{}
}

func (w *slowWriter) Save(s Snapshot) error { <-w.release; return w.Store.Save(s) }

// TestSaverCallsNeverWaitForDisk: while a slow write runs, Changed, Delete,
// and Move return at once; the server calls them from a goroutine that must
// never block.
func TestSaverCallsNeverWaitForDisk(t *testing.T) {
	w := &slowWriter{Store: openStore(t), release: make(chan struct{})}
	s := NewSaver(w, time.Millisecond, func(string, error) {})
	s.Changed(sample("a"))
	time.Sleep(50 * time.Millisecond) // the save of a is now on disk, blocked
	returned := make(chan struct{})
	go func() {
		s.Delete("b")
		s.Changed(sample("c"))
		s.Move("d", "e", nil)
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("a call waited for the slow write")
	}
	close(w.release)
	_ = s.Close()
}

func TestSaverMove(t *testing.T) {
	st := openStore(t)
	synctest.Test(t, func(t *testing.T) {
		s := newSaver(t, st)
		s.Changed(sample("api"))
		if err := s.Flush(); err != nil {
			t.Fatal(err)
		}
		moved := make(chan error, 1)
		s.Move("api", "web", func(err error) { moved <- err })
		if err := <-moved; err != nil {
			t.Fatal(err)
		}
		if exists(t, st, "api") || !exists(t, st, "web") {
			t.Fatal("Move left the old name or lost the new one")
		}
		_ = s.Close()
	})
}
