package convstore

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/private/privatetest"
)

func TestAppendWritesWholeLine(t *testing.T) {
	w := openWriter(t, openStore(t))
	for _, r := range everyRecord[1:] {
		if err := w.Append(r); err != nil {
			t.Fatal(err)
		}
	}
	got := records(t, w.Path())[1:]
	if !reflect.DeepEqual(got, everyRecord[1:]) {
		t.Fatalf("read back %d records:\n%#v", len(got), got)
	}
}

// TestAppendAfterTornLine: a record after a line torn by a crash starts on
// its own line, and the torn line stays as it was.
func TestAppendAfterTornLine(t *testing.T) {
	s := openStore(t)
	path, err := s.Path(testID)
	if err != nil {
		t.Fatal(err)
	}
	torn := `{"type":"user","at":"2026-10-23T19:15:00Z","text":"cut`
	if err := os.WriteFile(path, []byte(`{"type":"user","text":"whole"}`+"\n"+torn), 0o600); err != nil {
		t.Fatal(err)
	}
	w, err := s.Writer(testID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	if err := w.Append(&User{At: at, Text: "next"}); err != nil {
		t.Fatal(err)
	}
	lines, endsTorn := fileLines(t, path)
	if endsTorn || len(lines) != 3 || string(lines[1]) != torn {
		t.Fatalf("lines %q (torn end %v)", lines, endsTorn)
	}
	if r, err := Decode(lines[2]); err != nil || r.(*User).Text != "next" {
		t.Fatalf("new record %q: %v", lines[2], err)
	}
}

// TestPartialRateLimit: many partials within one wait give one write, with
// the latest text, when the wait ends. The wait is short here, so a stalled
// runner cannot end it early; TestPartialEvery checks the real one.
func TestPartialRateLimit(t *testing.T) {
	w := openWriter(t, openStore(t))
	w.every = 300 * time.Millisecond
	for i := range 50 {
		if err := w.AppendPartial(Partial{At: at, Text: fmt.Sprint("text ", i)}); err != nil {
			t.Fatal(err)
		}
	}
	lines, _ := fileLines(t, w.Path())
	if len(lines) != 1 {
		t.Fatalf("%d lines written before the wait ended", len(lines)-1)
	}
	time.Sleep(3 * w.every)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if got := partials(t, w.Path()); !reflect.DeepEqual(got, []string{"text 49"}) {
		t.Fatalf("partials %q, want one with the latest text", got)
	}
}

// TestPartialEvery: a Writer waits one second between partial writes.
func TestPartialEvery(t *testing.T) {
	if w := openWriter(t, openStore(t)); partialEvery != time.Second || w.every != partialEvery {
		t.Fatalf("partialEvery %v, writer waits %v", partialEvery, w.every)
	}
}

// TestPartialThenRecord: a whole reply replaces partial text that waits; a
// user message, an interruption, and Close write it first, so a cut-off
// reply keeps it; any other record leaves it waiting.
func TestPartialThenRecord(t *testing.T) {
	tests := []struct {
		name string
		then func(*Writer) error
		want []Type
	}{
		{"assistant", func(w *Writer) error { return w.Append(&Assistant{At: at}) }, []Type{TypeAssistant}},
		{"interruption", func(w *Writer) error { return w.Append(&Interruption{At: at}) }, []Type{TypePartial, TypeInterruption}},
		{"user", func(w *Writer) error { return w.Append(&User{At: at}) }, []Type{TypePartial, TypeUser}},
		{"rename", func(w *Writer) error { return w.Append(&Rename{At: at}) }, []Type{TypeRename, TypePartial}},
		{"close", (*Writer).Close, []Type{TypePartial}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := openWriter(t, openStore(t))
			w.every = 50 * time.Millisecond
			if err := w.AppendPartial(Partial{At: at, Text: "half"}); err != nil {
				t.Fatal(err)
			}
			if err := tt.then(w); err != nil {
				t.Fatal(err)
			}
			time.Sleep(4 * w.every)
			var got []Type
			for _, r := range records(t, w.Path())[1:] {
				got = append(got, r.Type())
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("records %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFileMode(t *testing.T) {
	s := openStore(t)
	w := openWriter(t, s)
	if err := w.Append(&User{At: at, Text: "hi"}); err != nil {
		t.Fatal(err)
	}
	privatetest.OwnerOnly(t, s.dir, 0o700)
	privatetest.OwnerOnly(t, w.Path(), 0o600)
}

// TestSecondWriter: one conversation has one live writer; it can open
// again after Close.
func TestSecondWriter(t *testing.T) {
	s := openStore(t)
	w := openWriter(t, s)
	if _, err := s.Writer(testID); !errors.Is(err, ErrInUse) {
		t.Fatalf("second writer error %v, want ErrInUse", err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Append(&User{At: at}); !errors.Is(err, errClosed) {
		t.Fatalf("append after close: %v", err)
	}
	w, err := s.Writer(testID)
	if err != nil {
		t.Fatalf("open after close: %v", err)
	}
	_ = w.Close()
}

// TestWriteFailure: a record that cannot be saved gives a WriteError that
// names the file, and the writer can be opened again after Close.
func TestWriteFailure(t *testing.T) {
	t.Run("cannot open", func(t *testing.T) {
		s := openStore(t)
		path, _ := s.Path(testID)
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		var we *WriteError
		if _, err := s.Create(*testMeta()); !errors.As(err, &we) || we.Path != path {
			t.Fatalf("got %v, want a WriteError for %s", err, path)
		}
		if _, err := s.Writer(testID); !errors.As(err, &we) || we.Path != path {
			t.Fatalf("got %v, want a WriteError for %s", err, path)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		openWriter(t, s)
	})
	t.Run("cannot write", func(t *testing.T) {
		w := openWriter(t, openStore(t))
		if err := w.f.Close(); err != nil {
			t.Fatal(err)
		}
		var we *WriteError
		if err := w.Append(&User{At: at}); !errors.As(err, &we) || we.Path != w.Path() {
			t.Fatalf("got %v, want a WriteError for %s", err, w.Path())
		}
	})
}

// TestFailedReplyKeepsPartial: when the whole reply cannot be saved, the
// partial text that waits is kept, since it is all there is on disk.
func TestFailedReplyKeepsPartial(t *testing.T) {
	w := openWriter(t, openStore(t))
	if err := w.AppendPartial(Partial{At: at, Text: "half"}); err != nil {
		t.Fatal(err)
	}
	if err := w.f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Append(&Assistant{At: at}); err == nil {
		t.Fatal("append to a closed file succeeded")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pending == nil || w.pending.Text != "half" || w.timer == nil {
		t.Fatalf("pending %#v, timer %v", w.pending, w.timer)
	}
}

// TestNewLineAfterFailedWrite: a write that fails may leave half a line,
// so the next record starts on a new line.
func TestNewLineAfterFailedWrite(t *testing.T) {
	w := openWriter(t, openStore(t))
	if err := w.f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Append(&User{At: at, Text: "lost"}); err == nil {
		t.Fatal("append to a closed file succeeded")
	}
	f, err := os.OpenFile(w.Path(), os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	w.f = f
	if err := w.Append(&User{At: at, Text: "next"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(w.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("\n\n{\"type\":\"user\"")) {
		t.Fatalf("next record does not start on a new line: %q", data)
	}
}

// killEnv makes TestAppendKilled act as the child that appends until it is
// killed.
const killEnv = "SNOW_CONVSTORE_KILL_DIR"

// TestAppendKilled: a process killed while it appends leaves every record
// that Append acknowledged whole in the file.
func TestAppendKilled(t *testing.T) {
	if dir := os.Getenv(killEnv); dir != "" {
		appendUntilKilled(t, dir)
		return
	}
	dir := filepath.Join(t.TempDir(), "conversations")
	acked := killAppender(t, dir)
	// The kill may tear the one record that was being written; only
	// acknowledged records must be whole.
	lines, torn := fileLines(t, filepath.Join(dir, testID+ext))
	if torn {
		lines = lines[:len(lines)-1]
	}
	var rs []Record
	for i, l := range lines[1:] {
		r, err := Decode(l)
		if err != nil {
			t.Fatalf("line %d %q: %v", i+2, l, err)
		}
		rs = append(rs, r)
	}
	if len(rs) < acked {
		t.Fatalf("file has %d records, %d were acknowledged", len(rs), acked)
	}
	for i := range acked {
		if u, ok := rs[i].(*User); !ok || u.Text != strconv.Itoa(i+1) {
			t.Fatalf("record %d is %#v", i+1, rs[i])
		}
	}
}

// appendUntilKilled is the child of TestAppendKilled. It prints the number
// of each record after Append returns.
func appendUntilKilled(t *testing.T, dir string) {
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	w, err := s.Create(*testMeta())
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < 1_000_000; i++ {
		if err := w.Append(&User{At: at, Text: strconv.Itoa(i)}); err != nil {
			t.Fatal(err)
		}
		fmt.Println(i)
	}
}

// killAppender runs the child, kills it once it has acknowledged some
// records, and returns the last number it printed.
func killAppender(t *testing.T, dir string) int {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(self, "-test.run=^TestAppendKilled$")
	cmd.Env = append(os.Environ(), killEnv+"="+dir)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	acked := 0
	sc := bufio.NewScanner(out)
	for acked < 30 && sc.Scan() {
		if n, err := strconv.Atoi(sc.Text()); err == nil {
			acked = n
		}
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	// Numbers printed before the kill landed are acknowledged too.
	for sc.Scan() {
		if n, err := strconv.Atoi(sc.Text()); err == nil {
			acked = n
		}
	}
	_ = cmd.Wait()
	if acked < 30 {
		t.Fatalf("the child acknowledged only %d records", acked)
	}
	return acked
}
