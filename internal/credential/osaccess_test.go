package credential

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

// TestLockedStoreNoFallback: a store that is there but refuses returns its
// error; the key does not go to the file, and the next call asks again.
func TestLockedStoreNoFallback(t *testing.T) {
	dir := t.TempDir()
	o := newFakeOS()
	o.locked = true
	var notices []string
	s := newStore(env(nil), o, dir, func(m string) { notices = append(notices, m) })
	if _, err := s.Set("anthropic", mustKey(t, secret)); !errors.Is(err, errLocked) {
		t.Fatalf("Set to a locked store = %v, want its error", err)
	}
	if _, _, err := s.Get("anthropic"); !errors.Is(err, errLocked) {
		t.Fatalf("Get from a locked store = %v, want its error", err)
	}
	if _, err := os.Stat(filepath.Join(dir, fileName)); !errors.Is(err, os.ErrNotExist) || len(notices) != 0 {
		t.Fatalf("the key went to the file: %v, notices %q", err, notices)
	}
	o.mu.Lock()
	o.locked = false
	o.mu.Unlock()
	if src, err := s.Set("anthropic", mustKey(t, secret)); err != nil || src != SourceOS {
		t.Fatalf("Set after unlock = %v, %v; want the OS store", src, err)
	}
}

// TestOSStoreTimeout: a store that does not answer sends the call to the
// file in time, and the notice says why.
func TestOSStoreTimeout(t *testing.T) {
	dir := t.TempDir()
	writeKeys(t, dir, map[string]string{"anthropic": "k-file"})
	o := newFakeOS()
	o.block = make(chan struct{})
	defer close(o.block)
	var notices []string
	s := newStore(env(nil), o, dir, func(m string) { notices = append(notices, m) })
	s.timeout = 20 * time.Millisecond
	start := time.Now()
	if k, src, err := s.Get("anthropic"); err != nil || src != SourceFile || k.Reveal() != "k-file" {
		t.Fatalf("Get = %v, %v; want the file's key", src, err)
	}
	if src, err := s.Set("openai", mustKey(t, secret)); err != nil || src != SourceFile {
		t.Fatalf("Set = %v, %v; want the file", src, err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("calls took %v", d)
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "did not answer") {
		t.Fatalf("notices = %q", notices)
	}
}

// TestSetMovesKeyToOSStore: a key saved to a working OS store leaves no
// old copy in the file, and other keys stay.
func TestSetMovesKeyToOSStore(t *testing.T) {
	dir := t.TempDir()
	writeKeys(t, dir, map[string]string{"anthropic": "old", "openai": "k2"})
	s := newStore(env(nil), newFakeOS(), dir, nil)
	if src, err := s.Set("anthropic", mustKey(t, secret)); err != nil || src != SourceOS {
		t.Fatalf("Set = %v, %v", src, err)
	}
	keys, err := readFile(filepath.Join(dir, fileName))
	if err != nil || keys["openai"] != "k2" {
		t.Fatalf("file = %d keys, %v", len(keys), err)
	}
	if _, ok := keys["anthropic"]; ok {
		t.Fatal("the old copy stays in the file")
	}
}

// TestFileBOM: a file saved by an editor that adds a byte order mark still
// reads.
func TestFileBOM(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte("\ufeffanthropic = k-file\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	o := newFakeOS()
	o.broken = true
	if k, _, err := newStore(env(nil), o, dir, nil).Get("anthropic"); err != nil || k.Reveal() != "k-file" {
		t.Fatalf("Get = %v", err)
	}
}

// TestKeyringAdapter: the go-keyring adapter, on its in-memory backend,
// saves and finds keys, reports a missing key as ErrNotFound, and a
// platform with no store as unavailable.
func TestKeyringAdapter(t *testing.T) {
	keyring.MockInit()
	s := newStore(env(nil), keyringStore{}, t.TempDir(), nil)
	if src, err := s.Set("anthropic", mustKey(t, secret)); err != nil || src != SourceOS {
		t.Fatalf("Set = %v, %v", src, err)
	}
	if k, src, err := s.Get("anthropic"); err != nil || src != SourceOS || k.Reveal() != secret {
		t.Fatalf("Get = %v, %v", src, err)
	}
	if err := s.Delete("anthropic"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Get("anthropic"); !errors.Is(err, ErrNoKey) {
		t.Fatalf("Get after Delete = %v", err)
	}
	if _, err := (keyringStore{}).Get("anthropic"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("adapter Get = %v, want ErrNotFound", err)
	}

	keyring.MockInitWithError(keyring.ErrUnsupportedPlatform)
	var notices []string
	s = newStore(env(nil), keyringStore{}, t.TempDir(), func(m string) { notices = append(notices, m) })
	if src, err := s.Set("anthropic", mustKey(t, secret)); err != nil || src != SourceFile || len(notices) != 1 {
		t.Fatalf("Set with no store = %v, %v, %d notices", src, err, len(notices))
	}
}
