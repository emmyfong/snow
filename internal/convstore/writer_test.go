package convstore

import (
	"bufio"
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
	for _, r := range everyRecord {
		if err := w.Append(r); err != nil {
			t.Fatal(err)
		}
	}
	got := records(t, w.Path())
	if !reflect.DeepEqual(got, everyRecord) {
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
	w := openWriter(t, s)
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

// TestPartialRateLimit: many partials in one second give one write, with
// the latest text, when the second ends.
func TestPartialRateLimit(t *testing.T) {
	w := openWriter(t, openStore(t))
	start := time.Now()
	for i := range 50 {
		if err := w.AppendPartial(Partial{At: at, Text: fmt.Sprint("text ", i)}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	if got := partials(t, w.Path()); len(got) != 0 && time.Since(start) < partialEvery {
		t.Fatalf("partial written before the second ended: %q", got)
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(partials(t, w.Path())) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	if got := partials(t, w.Path()); !reflect.DeepEqual(got, []string{"text 49"}) {
		t.Fatalf("partials %q, want one with the latest text", got)
	}
}

// TestPartialThenRecord: a whole reply replaces partial text that waits; any
// other record, and Close, write it first, so a cut-off reply keeps it.
func TestPartialThenRecord(t *testing.T) {
	tests := []struct {
		name string
		then func(*Writer) error
		want []Type
	}{
		{"assistant", func(w *Writer) error { return w.Append(&Assistant{At: at}) }, []Type{TypeAssistant}},
		{"interruption", func(w *Writer) error { return w.Append(&Interruption{At: at}) }, []Type{TypePartial, TypeInterruption}},
		{"close", (*Writer).Close, []Type{TypePartial}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := openWriter(t, openStore(t))
			if err := w.AppendPartial(Partial{At: at, Text: "half"}); err != nil {
				t.Fatal(err)
			}
			if err := tt.then(w); err != nil {
				t.Fatal(err)
			}
			time.Sleep(partialEvery + 200*time.Millisecond)
			var got []Type
			for _, r := range records(t, w.Path()) {
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
	openWriter(t, s)
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
	rs := records(t, filepath.Join(dir, testID+ext))
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
	w, err := s.Writer(testID)
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
