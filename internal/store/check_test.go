package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeRaw puts b in the folder of st as file name.
func writeRaw(t *testing.T, st *Store, name string, b []byte) string {
	t.Helper()
	path := filepath.Join(st.dir, name)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func rawSnapshot(t *testing.T, s Snapshot) []byte {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestNewerSnapshotIsNotBroken: a file saved by a newer Snow is not
// corrupt; it is reported as newer, so it stays in place for that version.
func TestNewerSnapshotIsNotBroken(t *testing.T) {
	st := openStore(t)
	s := sample("api")
	s.Version = Version + 1
	writeRaw(t, st, "api.json", rawSnapshot(t, s))
	_, errs := st.List()
	var de *DecodeError
	if len(errs) != 1 || !errors.Is(errs[0], ErrNewerVersion) || errors.As(errs[0], &de) {
		t.Fatalf("List errors %v, want one ErrNewerVersion that is not a DecodeError", errs)
	}
	if _, err := st.Load("api"); !errors.Is(err, ErrNewerVersion) {
		t.Fatalf("Load error %v, want ErrNewerVersion", err)
	}
}

// TestSnapshotChecks: a file that is too big, too large inside, or named
// for another session cannot be read back, and so is broken. Too many
// saved lines are cut to the last MaxScrollback.
func TestSnapshotChecks(t *testing.T) {
	st := openStore(t)
	many := sample("many")
	many.Version = Version
	for range maxWindows + 1 {
		many.Windows = append(many.Windows, many.Windows[0])
	}
	other := sample("other")
	other.Version = Version
	tests := []struct {
		file string
		b    []byte
	}{
		{"many.json", rawSnapshot(t, many)},
		{"api.json", rawSnapshot(t, other)}, // the file says "other"
		{"big.json", []byte(`{"version":1,"name":"big","pad":"` + strings.Repeat("x", maxFileSize) + `"}`)},
	}
	for _, tt := range tests {
		writeRaw(t, st, tt.file, tt.b)
	}
	infos, errs := st.List()
	if len(infos) != 0 || len(errs) != len(tests) {
		t.Fatalf("List = %v, %v; want every file reported", infos, errs)
	}
	for _, err := range errs {
		if de := (*DecodeError)(nil); !errors.As(err, &de) {
			t.Errorf("error %v is not a DecodeError", err)
		}
	}

	long := sample("long")
	long.Version = Version
	long.Windows[0].Panes[0].Scrollback = make([]string, MaxScrollback+10)
	long.Windows[0].Panes[0].Scrollback[MaxScrollback+9] = "last"
	writeRaw(t, openStoreAt(t, st.dir+"2"), "long.json", rawSnapshot(t, long))
	got, err := openStoreAt(t, st.dir+"2").Load("long")
	if err != nil {
		t.Fatal(err)
	}
	if lines := got.Windows[0].Panes[0].Scrollback; len(lines) != MaxScrollback || lines[len(lines)-1] != "last" {
		t.Fatalf("loaded %d lines, want the last %d", len(lines), MaxScrollback)
	}
}

func openStoreAt(t *testing.T, dir string) *Store {
	t.Helper()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// TestBrokenNamesAreUnique: two broken files with the same name in the same
// second are both kept.
func TestBrokenNamesAreUnique(t *testing.T) {
	st := openStore(t)
	at := time.Date(2026, 10, 8, 9, 14, 0, 0, time.UTC)
	var moved []string
	for range 2 {
		p := writeRaw(t, st, "bad.json", []byte("{not json"))
		m, err := st.MoveBroken(p, at)
		if err != nil {
			t.Fatal(err)
		}
		moved = append(moved, m)
	}
	if moved[0] == moved[1] {
		t.Fatalf("both moves went to %s", moved[0])
	}
	for _, m := range moved {
		if _, err := os.Stat(m); err != nil {
			t.Fatalf("%s: %v", m, err)
		}
	}
}
