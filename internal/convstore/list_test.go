package convstore

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// writeConv saves a conversation with id in folder and profile, made at
// start, with recs after its meta record.
func writeConv(t *testing.T, s *Store, id, folder, profile string, start time.Time, recs ...Record) {
	t.Helper()
	w, err := s.Writer(id)
	if err != nil {
		t.Fatal(err)
	}
	meta := &Meta{Version: Version, ID: id, Folder: folder, Profile: profile, Created: start, Model: "anthropic/claude-opus-5-5"}
	for _, r := range append([]Record{meta}, recs...) {
		if err := w.Append(r); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func ids(es []Entry) []string {
	var out []string
	for _, e := range es {
		out = append(out, e.ID)
	}
	return out
}

func TestListPerFolder(t *testing.T) {
	s := openStore(t)
	writeConv(t, s, "a-bash-old", "/a", "bash", at)
	writeConv(t, s, "a-bash-new", "/a", "bash", at, &User{At: at.Add(time.Hour), Text: "hi"})
	writeConv(t, s, "a-zsh", "/a", "zsh", at.Add(2*time.Hour))
	writeConv(t, s, "b-bash", "/b", "bash", at.Add(3*time.Hour))
	writeConv(t, s, "a-win", `C:\a`, "pwsh", at.Add(4*time.Hour))
	tests := []struct {
		folder, profile string
		want            []string
	}{
		{"/a", "bash", []string{"a-bash-new", "a-bash-old"}},
		{"/a", "zsh", []string{"a-zsh"}},
		{"/b", "bash", []string{"b-bash"}},
		{"/b", "zsh", nil},
		{`C:\a`, "pwsh", []string{"a-win"}},
	}
	for _, tt := range tests {
		got, err := s.List(tt.folder, tt.profile)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(ids(got), tt.want) {
			t.Errorf("List(%q, %q) = %v, want %v", tt.folder, tt.profile, ids(got), tt.want)
		}
	}
}

func TestListTitle(t *testing.T) {
	s := openStore(t)
	long := "Please   read\nthe file " + strings.Repeat("abcdefghij", 10)
	writeConv(t, s, "renamed", "/a", "", at,
		&User{At: at, Text: "first"}, &Rename{At: at, Title: "Old"}, &Rename{At: at, Title: "Picked title"}, &User{At: at, Text: "second"})
	writeConv(t, s, "long", "/a", "", at, &User{At: at, Text: long})
	writeConv(t, s, "empty", "/a", "", at)
	got := map[string]string{}
	es, err := s.ListAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range es {
		got[e.ID] = e.Title
	}
	if got["renamed"] != "Picked title" {
		t.Errorf("renamed title %q", got["renamed"])
	}
	if title := got["long"]; len([]rune(title)) != maxTitle || !strings.HasPrefix(title, "Please read the file abc") || !strings.HasSuffix(title, "…") {
		t.Errorf("long title %q (%d characters)", title, len([]rune(title)))
	}
	if got["empty"] != "" {
		t.Errorf("empty title %q", got["empty"])
	}
}

func TestListModelAndTurns(t *testing.T) {
	s := openStore(t)
	last := at.Add(time.Hour)
	writeConv(t, s, testID, "/a", "", at,
		&User{At: at, Text: "one"},
		&Assistant{At: at, Blocks: []Block{{Kind: BlockText, Text: "a"}}},
		&ModelSwitch{At: at, Model: "openai/gpt-6.1-sol"},
		&User{At: at, Text: "two"},
		&ModelSwitch{At: at, Model: "anthropic/claude-sonnet-5-5"},
		&User{At: at, Text: "three"},
		&Assistant{At: last, Blocks: []Block{{Kind: BlockText, Text: "b"}}})
	es, err := s.ListAll()
	if err != nil || len(es) != 1 {
		t.Fatalf("%v, %v", es, err)
	}
	e := es[0]
	if e.Model != "anthropic/claude-sonnet-5-5" || e.Turns != 3 || !e.LastActivity.Equal(last) || e.Folder != "/a" {
		t.Fatalf("entry %+v", e)
	}
}

// TestListBadFile: a file that cannot be read is in every list, with its
// error, and stays on disk.
func TestListBadFile(t *testing.T) {
	s := openStore(t)
	writeConv(t, s, "good", "/a", "", at)
	path := fixture(t, s, "torn.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"meta","version":99,"id":"`+testID+`"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	es, err := s.List("/elsewhere", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 1 || es[0].ID != testID || !errors.Is(es[0].Err, ErrNewerVersion) {
		t.Fatalf("entries %+v", es)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the bad file is gone: %v", err)
	}
}

func TestListTornLine(t *testing.T) {
	s := openStore(t)
	fixture(t, s, "torn.jsonl")
	es, err := s.ListAll()
	if err != nil || len(es) != 1 {
		t.Fatalf("%v, %v", es, err)
	}
	if e := es[0]; e.Err != nil || e.Turns != 1 || e.Title != "Read a.go" {
		t.Fatalf("entry %+v", e)
	}
}

// TestListAll: every folder, newest first; files that are not
// conversations are left out.
func TestListAll(t *testing.T) {
	s := openStore(t)
	writeConv(t, s, "one", "/a", "bash", at)
	writeConv(t, s, "two", "/b", "zsh", at.Add(time.Hour))
	for _, name := range []string{"notes.txt", "BAD.jsonl", "x.jsonl.tmp"} {
		if err := os.WriteFile(s.dir+"/"+name, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	es, err := s.ListAll()
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(es); !reflect.DeepEqual(got, []string{"two", "one"}) {
		t.Fatalf("ListAll = %v", got)
	}
}
