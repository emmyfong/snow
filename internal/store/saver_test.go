package store

import (
	"errors"
	"io/fs"
	"os"
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

func newSaver(t *testing.T, st *countingStore) *Saver {
	t.Helper()
	return NewSaver(st, delay, func(name string, err error) {
		t.Errorf("save %q failed: %v", name, err)
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
	})
}

func TestSaverDeleteCancelsPending(t *testing.T) {
	st := &countingStore{Store: openStore(t)}
	synctest.Test(t, func(t *testing.T) {
		s := newSaver(t, st)
		s.Changed(sample("api"))
		if err := s.Delete("api"); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * delay)
		synctest.Wait()
		if n := st.count(); n != 0 {
			t.Fatalf("%d writes after Delete, want 0", n)
		}
		if exists(t, st.Store, "api") {
			t.Fatal("snapshot exists after Delete")
		}
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
		if err := s.Delete("api"); err != nil {
			t.Fatal(err)
		}
		if exists(t, st.Store, "api") {
			t.Fatal("snapshot exists after Delete")
		}
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
			if err == nil {
				t.Error("error callback got a nil error")
			}
			failed = append(failed, name)
		})
		s.Changed(sample("api"))
		time.Sleep(2 * delay)
		synctest.Wait()
		mu.Lock()
		defer mu.Unlock()
		if len(failed) != 1 || failed[0] != "api" {
			t.Fatalf("error callback got %q, want one call for \"api\"", failed)
		}
	})
}
