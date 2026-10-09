package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenLogWrites(t *testing.T) {
	dir := t.TempDir()
	log, closer, err := OpenLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("hello")
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, logName))
	if err != nil || !strings.Contains(string(b), "hello") {
		t.Fatalf("log = %q, %v", b, err)
	}
}

// TestOpenLogRotates: a long-running server must not fill the disk; a log
// over the limit moves aside, replacing an older one.
func TestOpenLogRotates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, logName)
	if err := os.WriteFile(path, make([]byte, maxLogSize+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".old", []byte("oldest"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, closer, err := OpenLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = closer.Close() })
	if fi, err := os.Stat(path); err != nil || fi.Size() != 0 {
		t.Fatalf("new log: %v, %v; want empty", fi, err)
	}
	if fi, err := os.Stat(path + ".old"); err != nil || fi.Size() != maxLogSize+1 {
		t.Fatalf("old log: %v, %v; want the previous log", fi, err)
	}
}
