package convstore

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// types returns the type of each record.
func types(rs []Record) []Type {
	var ts []Type
	for _, r := range rs {
		ts = append(ts, r.Type())
	}
	return ts
}

func load(t *testing.T, s *Store) *Conversation {
	t.Helper()
	c, err := s.Load(testID)
	if err != nil {
		t.Fatal(err)
	}
	if c.Meta.ID != testID {
		t.Fatalf("meta %#v", c.Meta)
	}
	return c
}

func TestLoadTornLastLine(t *testing.T) {
	s := openStore(t)
	fixture(t, s, "torn.jsonl")
	c := load(t, s)
	if got := types(c.Records); !reflect.DeepEqual(got, []Type{TypeUser, TypeAssistant}) {
		t.Fatalf("records %v", got)
	}
	if len(c.Problems) != 1 || !c.Problems[0].Torn || c.Problems[0].Line != 4 {
		t.Fatalf("problems %v", c.Problems)
	}
	if c.MidTurn {
		t.Fatal("a torn user line made the turn open")
	}
}

// TestLoadNoNewline: a last line that is whole JSON but lost its newline
// is kept.
func TestLoadNoNewline(t *testing.T) {
	s := openStore(t)
	fixture(t, s, "nonewline.jsonl")
	c := load(t, s)
	if len(c.Records) != 1 || len(c.Problems) != 0 || !c.MidTurn {
		t.Fatalf("records %v, problems %v, mid-turn %v", types(c.Records), c.Problems, c.MidTurn)
	}
}

func TestLoadCutOffReply(t *testing.T) {
	s := openStore(t)
	fixture(t, s, "cutoff.jsonl")
	c := load(t, s)
	if len(c.Records) != 2 {
		t.Fatalf("records %v", types(c.Records))
	}
	a, ok := c.Records[1].(*Assistant)
	if !ok || !a.CutOff || len(a.Blocks) != 1 || a.Blocks[0].Text != "Hello wor" {
		t.Fatalf("reply %#v", c.Records[1])
	}
	if !c.MidTurn {
		t.Fatal("a cut-off reply did not leave the turn open")
	}
}

// TestLoadMidTurn: a reply that waits for tool results ends the file; the
// missing result gets a stand-in, and the file is not changed.
func TestLoadMidTurn(t *testing.T) {
	s := openStore(t)
	path := fixture(t, s, "midturn.jsonl")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	c := load(t, s)
	if !c.MidTurn {
		t.Fatal("MidTurn is false")
	}
	want := []Type{TypeUser, TypeAssistant, TypeToolResult, TypeToolResult}
	if got := types(c.Records); !reflect.DeepEqual(got, want) {
		t.Fatalf("records %v, want %v", got, want)
	}
	if r := c.Records[3].(*ToolResult); r.ToolUseID != "c2" || !r.Lost || !r.IsError || r.Content != resultLost {
		t.Fatalf("stand-in %#v", r)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("Load changed the file: %v", err)
	}
}

// TestLoadDamagedLine: a bad line in the middle is skipped and marked; the
// lines after it load.
func TestLoadDamagedLine(t *testing.T) {
	s := openStore(t)
	fixture(t, s, "damaged.jsonl")
	c := load(t, s)
	if got := types(c.Records); !reflect.DeepEqual(got, []Type{TypeUser, TypeDamaged, TypeAssistant}) {
		t.Fatalf("records %v", got)
	}
	if d := c.Records[1].(*Damaged); d.Line != 3 {
		t.Fatalf("damaged %#v", d)
	}
	if len(c.Problems) != 1 || c.Problems[0].Torn || c.Problems[0].Line != 3 {
		t.Fatalf("problems %v", c.Problems)
	}
}

func TestLoadUnknownRecord(t *testing.T) {
	s := openStore(t)
	fixture(t, s, "unknown.jsonl")
	c := load(t, s)
	if got := types(c.Records); !reflect.DeepEqual(got, []Type{TypeUser, "question", TypeAssistant}) {
		t.Fatalf("records %v", got)
	}
	if _, ok := c.Records[1].(*Unknown); !ok {
		t.Fatalf("question is %#v", c.Records[1])
	}
}

func TestLoadErrors(t *testing.T) {
	meta := func(m string) string { return m + "\n" }
	tests := []struct {
		name  string
		setup func(t *testing.T, path string)
		want  error
	}{
		{"newer version", writeFile(meta(`{"type":"meta","version":99,"id":"` + testID + `"}`)), ErrNewerVersion},
		{"no meta", writeFile(`{"type":"user","text":"hi"}` + "\n"), ErrNoMeta},
		{"empty", writeFile(""), ErrNoMeta},
		{"bad meta", writeFile("{\n"), ErrNoMeta},
		{"other id", writeFile(meta(`{"type":"meta","version":1,"id":"other"}`)), ErrIDMismatch},
		{"no version", writeFile(meta(`{"type":"meta","id":"` + testID + `"}`)), ErrNoMeta},
		{"missing", func(*testing.T, string) {}, fs.ErrNotExist},
		{"a directory", func(t *testing.T, path string) {
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
		}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := openStore(t)
			path, _ := s.Path(testID)
			tt.setup(t, path)
			_, err := s.Load(testID)
			var le *LoadError
			if !errors.As(err, &le) || le.Path != path {
				t.Fatalf("got %v, want a LoadError for %s", err, path)
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}
	t.Run("unreadable", func(t *testing.T) {
		s := openStore(t)
		path := writeRecords(t, s)
		want := makeUnreadable(t, path)
		var le *LoadError
		if _, err := s.Load(testID); !errors.As(err, &le) || le.Path != path || !errors.Is(err, want) {
			t.Fatalf("got %v, want a LoadError for %s wrapping %v", err, path, want)
		}
	})
}

// writeFile returns a setup that writes body to the file.
func writeFile(body string) func(*testing.T, string) {
	return func(t *testing.T, path string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoadLongLine(t *testing.T) {
	s := openStore(t)
	w := openWriter(t, s)
	long := strings.Repeat("x", 2<<20)
	if err := w.Append(&User{At: at, Text: long}); err != nil {
		t.Fatal(err)
	}
	c := load(t, s)
	if len(c.Records) != 1 || c.Records[0].(*User).Text != long {
		t.Fatalf("records %v", types(c.Records))
	}
}

// TestLoadAfterKill: every record that a killed process acknowledged loads.
func TestLoadAfterKill(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "conversations")
	acked := killAppender(t, dir)
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	c := load(t, s)
	if len(c.Records) < acked {
		t.Fatalf("loaded %d records, %d were acknowledged; problems %v", len(c.Records), acked, c.Problems)
	}
}

// TestLoadKeepsFile: a load that fails leaves the folder as it was.
func TestLoadKeepsFile(t *testing.T) {
	s := openStore(t)
	path, _ := s.Path(testID)
	body := `{"type":"meta","version":99,"id":"` + testID + `"}` + "\n"
	writeFile(body)(t, path)
	if _, err := s.Load(testID); err == nil {
		t.Fatal("a newer file loaded")
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != testID+ext {
		t.Fatalf("folder holds %v, %v", entries, err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != body {
		t.Fatalf("file changed: %q, %v", got, err)
	}
}

// writeRecords writes testMeta and recs to the file of testID as they are,
// without a Writer, and returns its path.
func writeRecords(t *testing.T, s *Store, recs ...Record) string {
	t.Helper()
	var b []byte
	for _, r := range append([]Record{testMeta()}, recs...) {
		line, err := Encode(r)
		if err != nil {
			t.Fatal(err)
		}
		b = append(b, line...)
	}
	path, _ := s.Path(testID)
	writeFile(string(b))(t, path)
	return path
}

// TestLoadRecordMidTurn: a record that does not end a turn, such as a
// rename, a model switch, or a type from a later version, stays in place.
// It does not turn waiting partial text into a cut-off reply, or add a
// stand-in for a tool result that comes after it.
func TestLoadRecordMidTurn(t *testing.T) {
	call := &Assistant{At: at, Stop: StopToolUse, Blocks: []Block{{Kind: BlockToolUse, ID: "t1", Name: "read"}}}
	result := &ToolResult{At: at, ToolUseID: "t1", Content: "ok"}
	reply := &Assistant{At: at, Stop: "end_turn", Blocks: []Block{{Kind: BlockText, Text: "whole"}}}
	partial := &Partial{At: at, Text: "who"}
	unknown := &Unknown{Kind: "question", Raw: []byte(`{"type":"question"}`)}
	for _, mid := range []Record{&Rename{At: at, Title: "t"}, &ModelSwitch{At: at, Model: "openai/gpt-6.1-sol"}, unknown} {
		t.Run(string(mid.Type()), func(t *testing.T) {
			tests := []struct {
				name string
				recs []Record
				want []Type
			}{
				{"between a call and its result", []Record{&User{At: at}, call, mid, result, reply},
					[]Type{TypeUser, TypeAssistant, mid.Type(), TypeToolResult, TypeAssistant}},
				{"between partial text and its reply", []Record{&User{At: at}, partial, mid, reply},
					[]Type{TypeUser, mid.Type(), TypeAssistant}},
			}
			for _, tt := range tests {
				s := openStore(t)
				writeRecords(t, s, tt.recs...)
				c := load(t, s)
				if got := types(c.Records); !reflect.DeepEqual(got, tt.want) {
					t.Errorf("%s: records %v, want %v", tt.name, got, tt.want)
				}
				if c.MidTurn {
					t.Errorf("%s: MidTurn after a whole reply", tt.name)
				}
			}
		})
	}
}

// TestLoadBadLastLineWithNewline: a crash cannot leave a bad line that ends
// with a newline, so it is damage: marked in place, not dropped as torn.
func TestLoadBadLastLineWithNewline(t *testing.T) {
	s := openStore(t)
	path := writeRecords(t, s, &User{At: at, Text: "hi"})
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("{\"type\":\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	c := load(t, s)
	if got := types(c.Records); !reflect.DeepEqual(got, []Type{TypeUser, TypeDamaged}) {
		t.Fatalf("records %v", got)
	}
	if len(c.Problems) != 1 || c.Problems[0].Torn {
		t.Fatalf("problems %v", c.Problems)
	}
}
