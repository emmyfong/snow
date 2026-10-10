package store

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// crashEnv makes TestAtomicWrite act as the crashing child process: it
// starts a write to the path in the variable and exits halfway through.
const crashEnv = "SNOW_STORE_CRASH_PATH"

const crashExit = 3

var (
	oldData = []byte(`{"version":1,"name":"old"}`)
	newData = []byte(`{"version":1,"name":"new","windows":[]}`)
)

func TestAtomicWrite(t *testing.T) {
	if path := os.Getenv(crashEnv); path != "" {
		_ = writeAtomic(path, func(w io.Writer) error {
			if _, err := w.Write(newData[:len(newData)/2]); err != nil {
				return err
			}
			os.Exit(crashExit)
			return nil
		})
		t.Fatal("the write returned instead of crashing")
	}

	t.Run("error mid-write", func(t *testing.T) {
		path := oldFile(t)
		errCrash := errors.New("disk full")
		err := writeAtomic(path, func(w io.Writer) error {
			if _, err := w.Write(newData[:len(newData)/2]); err != nil {
				return err
			}
			return errCrash
		})
		if !errors.Is(err, errCrash) {
			t.Fatalf("writeAtomic error %v, want %v", err, errCrash)
		}
		assertContent(t, path, oldData)
		if _, err := os.Stat(tempPath(path)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("temp file left after a failed write: %v", err)
		}
	})

	t.Run("process exits mid-write", func(t *testing.T) {
		path := oldFile(t)
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(self, "-test.run=^TestAtomicWrite$")
		cmd.Env = append(os.Environ(), crashEnv+"="+path)
		out, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != crashExit {
			t.Fatalf("child did not crash mid-write: %v\n%s", err, out)
		}
		assertContent(t, path, oldData)
		assertContent(t, tempPath(path), newData[:len(newData)/2])

		if err := writeAtomic(path, writeBytes(newData)); err != nil {
			t.Fatalf("write after a crash: %v", err)
		}
		assertContent(t, path, newData)
		if _, err := os.Stat(tempPath(path)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("temp file left after a good write: %v", err)
		}
	})
}

func TestAtomicWriteReplacesExisting(t *testing.T) {
	path := oldFile(t)
	for range 3 {
		if err := writeAtomic(path, writeBytes(newData)); err != nil {
			t.Fatal(err)
		}
	}
	assertContent(t, path, newData)
}

func oldFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "api.json")
	if err := writeAtomic(path, writeBytes(oldData)); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertContent(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s holds %q, want %q", filepath.Base(path), got, want)
	}
}
