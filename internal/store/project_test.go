package store

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"testing"
	"time"
)

func TestProjectLayoutRoundTrip(t *testing.T) {
	p, err := OpenProjects(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.LoadLayout("/src/snow"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing file: %v", err)
	}
	want := json.RawMessage(`{"kind":"chat"}`)
	if err := p.SaveLayout("/src/snow", want); err != nil {
		t.Fatal(err)
	}
	got, err := p.LoadLayout("/src/snow")
	if err != nil || string(got) != string(want) {
		t.Fatalf("got %s, %v", got, err)
	}
	if _, err := p.LoadLayout("/src/other"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("another key: %v", err)
	}
}

// TestProjectLayoutCorrupt: a file that cannot be read back gives a
// DecodeError and stays on disk.
func TestProjectLayoutCorrupt(t *testing.T) {
	tests := []struct {
		name, body string
	}{
		{"not json", "{"},
		{"no version", `{"key":"k","layout":{}}`},
		{"other key", `{"version":1,"key":"other","layout":{}}`},
		{"no layout", `{"version":1,"key":"k"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := OpenProjects(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p.path("k"), []byte(tt.body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := p.LoadLayout("k"); !errors.As(err, new(*DecodeError)) {
				t.Fatalf("err %v, want a DecodeError", err)
			}
			if _, err := os.Stat(p.path("k")); err != nil {
				t.Fatalf("the file is gone: %v", err)
			}
		})
	}
}

func TestProjectLayoutNewer(t *testing.T) {
	p, err := OpenProjects(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.path("k"), []byte(`{"version":9,"key":"k","layout":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := p.LoadLayout("k"); !errors.Is(err, ErrNewerVersion) {
		t.Fatalf("err %v, want ErrNewerVersion", err)
	}
}

// TestProjectSaverDebounces: a burst of changes writes once, the last one.
func TestProjectSaverDebounces(t *testing.T) {
	dir := t.TempDir()
	p, err := OpenProjects(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := NewProjectSaver(p, 50*time.Millisecond, func(_ string, err error) { t.Error(err) })
	key := func() string { return "k" }
	s.Changed("id", key, json.RawMessage(`1`))
	s.Changed("id", key, json.RawMessage(`2`))
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatal("wrote before the delay")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if got, err := p.LoadLayout("k"); err == nil {
			if string(got) != "2" {
				t.Fatalf("saved %s, want the last change", got)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("nothing saved")
		}
		time.Sleep(10 * time.Millisecond)
	}
	s.Close()
}

// TestProjectSaverCloseFlushes: Close writes what waits for its delay, and
// later changes do nothing.
func TestProjectSaverCloseFlushes(t *testing.T) {
	p, err := OpenProjects(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := NewProjectSaver(p, time.Hour, nil)
	s.Changed("id", func() string { return "k" }, json.RawMessage(`{}`))
	s.Close()
	if _, err := p.LoadLayout("k"); err != nil {
		t.Fatalf("not flushed: %v", err)
	}
	s.Changed("id", func() string { return "k2" }, json.RawMessage(`{}`))
	if len(s.pending) != 0 {
		t.Fatal("a change after Close is pending")
	}
}
