package credential

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const secret = "sk-ant-test-0123456789"

// fakeOS is an OSStore in memory. broken acts like a system with no store;
// locked like a store that refuses; failSet makes saving fail with an
// error that holds the key, as a careless store might; block makes every
// call wait until the channel closes.
type fakeOS struct {
	mu      sync.Mutex
	keys    map[string]string
	broken  bool
	locked  bool
	failSet bool
	block   chan struct{}
}

func newFakeOS() *fakeOS { return &fakeOS{keys: map[string]string{}} }

var (
	errNoDBus = fmt.Errorf("%w: dbus: no session bus", ErrUnavailable)
	errLocked = errors.New("org.freedesktop.Secret.Error.IsLocked")
)

// state waits while blocked, then returns the error every call gives.
func (f *fakeOS) state() error {
	f.mu.Lock()
	block := f.block
	f.mu.Unlock()
	if block != nil {
		<-block
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case f.broken:
		return errNoDBus
	case f.locked:
		return errLocked
	}
	return nil
}

func (f *fakeOS) Get(p string) (string, error) {
	if err := f.state(); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.keys[p]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (f *fakeOS) Set(p, k string) error {
	if err := f.state(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failSet {
		return fmt.Errorf("add %s: access denied", k)
	}
	f.keys[p] = k
	return nil
}

func (f *fakeOS) Delete(p string) error {
	if err := f.state(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.keys[p]; !ok {
		return ErrNotFound
	}
	delete(f.keys, p)
	return nil
}

// key returns what the fake holds for p.
func (f *fakeOS) key(p string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.keys[p]
	return v, ok
}

func env(vars map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := vars[k]; return v, ok }
}

func mustKey(t *testing.T, s string) Key {
	t.Helper()
	k, err := NewKey(s)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func writeKeys(t *testing.T, dir string, keys map[string]string) {
	t.Helper()
	if err := writeFile(dir, filepath.Join(dir, fileName), keys); err != nil {
		t.Fatal(err)
	}
}

// TestLookupOrder: the environment wins over the OS store, and the OS store
// wins over the file.
func TestLookupOrder(t *testing.T) {
	tests := []struct {
		name      string
		env, os   string
		file      string
		want      string
		wantSrc   Source
		osMissing bool
	}{
		{name: "env wins", env: "k-env", os: "k-os", file: "k-file", want: "k-env", wantSrc: SourceEnv},
		{name: "OS over file", os: "k-os", file: "k-file", want: "k-os", wantSrc: SourceOS},
		{name: "file last", file: "k-file", want: "k-file", wantSrc: SourceFile},
		{name: "file when no OS store", file: "k-file", want: "k-file", wantSrc: SourceFile, osMissing: true},
		{name: "empty env ignored", env: "", os: "k-os", want: "k-os", wantSrc: SourceOS},
		{name: "spaces around an env key", env: " k-env\r\n", os: "k-os", want: "k-env", wantSrc: SourceEnv},
		{name: "blank env ignored", env: " \t ", os: "k-os", want: "k-os", wantSrc: SourceOS},
		{name: "line ending on an OS key", os: "k-os\n", want: "k-os", wantSrc: SourceOS},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			vars := map[string]string{}
			if tt.env != "" {
				vars["ANTHROPIC_API_KEY"] = tt.env
			}
			o := newFakeOS()
			if tt.os != "" {
				o.keys["anthropic"] = tt.os
			}
			o.broken = tt.osMissing
			if tt.file != "" {
				writeKeys(t, dir, map[string]string{"anthropic": tt.file})
			}
			k, src, err := newStore(env(vars), o, dir, nil).Get("anthropic")
			if err != nil || k.Reveal() != tt.want || src != tt.wantSrc {
				t.Fatalf("Get = %q, %v, %v; want %q from %v", k.Reveal(), src, err, tt.want, tt.wantSrc)
			}
		})
	}
	_, _, err := newStore(env(nil), newFakeOS(), t.TempDir(), nil).Get("anthropic")
	if !errors.Is(err, ErrNoKey) {
		t.Fatalf("Get with no key = %v, want ErrNoKey", err)
	}
}

// TestSetGoesToOSStore: with a working store, Set saves there and writes no
// file.
func TestSetGoesToOSStore(t *testing.T) {
	dir := t.TempDir()
	o := newFakeOS()
	s := newStore(env(nil), o, dir, nil)
	if src, err := s.Set("anthropic", mustKey(t, secret)); err != nil || src != SourceOS {
		t.Fatalf("Set = %v, %v", src, err)
	}
	if v, _ := o.key("anthropic"); v != secret {
		t.Fatal("the OS store does not hold the key")
	}
	if _, err := os.Stat(filepath.Join(dir, fileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a file was written: %v", err)
	}
	if got, err := s.APIKey("anthropic"); err != nil || got != secret {
		t.Fatalf("APIKey = %v", err)
	}
}

// TestFileFallbackNotice: the notice comes once, and only when the system
// has no OS store.
func TestFileFallbackNotice(t *testing.T) {
	for _, broken := range []bool{false, true} {
		o := newFakeOS()
		o.broken = broken
		var notices []string
		s := newStore(env(nil), o, filepath.Join(t.TempDir(), ".snow"), func(m string) { notices = append(notices, m) })
		for _, p := range []string{"anthropic", "openai"} {
			if _, err := s.Set(p, mustKey(t, secret)); err != nil {
				t.Fatal(err)
			}
		}
		if want := map[bool]int{false: 0, true: 1}[broken]; len(notices) != want {
			t.Fatalf("no OS store %v: %d notices, want %d: %q", broken, len(notices), want, notices)
		}
		if broken && (!strings.Contains(notices[0], "plain text") || !strings.Contains(notices[0], "no session bus")) {
			t.Fatalf("the notice does not say where or why: %q", notices[0])
		}
		if broken {
			if k, src, err := s.Get("openai"); err != nil || src != SourceFile || k.Reveal() != secret {
				t.Fatalf("Get from file = %v, %v", src, err)
			}
		}
	}
}

// TestDeleteRemovesKey: Delete removes the key from the place that held it.
func TestDeleteRemovesKey(t *testing.T) {
	t.Run("OS store", func(t *testing.T) {
		o := newFakeOS()
		o.keys["anthropic"] = secret
		s := newStore(env(nil), o, t.TempDir(), nil)
		if err := s.Delete("anthropic"); err != nil {
			t.Fatal(err)
		}
		if _, ok := o.key("anthropic"); ok {
			t.Fatal("key still in the OS store")
		}
	})
	t.Run("file", func(t *testing.T) {
		dir := t.TempDir()
		writeKeys(t, dir, map[string]string{"anthropic": secret, "openai": "k2"})
		o := newFakeOS()
		o.broken = true
		s := newStore(env(nil), o, dir, nil)
		if err := s.Delete("anthropic"); err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.Get("anthropic"); !errors.Is(err, ErrNoKey) {
			t.Fatalf("Get after Delete = %v", err)
		}
		if k, _, err := s.Get("openai"); err != nil || k.Reveal() != "k2" {
			t.Fatalf("the other key is gone: %v", err)
		}
	})
	t.Run("environment", func(t *testing.T) {
		s := newStore(env(map[string]string{"ANTHROPIC_API_KEY": secret}), newFakeOS(), t.TempDir(), nil)
		if err := s.Delete("anthropic"); !errors.Is(err, ErrInEnvironment) {
			t.Fatalf("Delete = %v, want ErrInEnvironment", err)
		}
	})
	t.Run("nothing saved", func(t *testing.T) {
		if err := newStore(env(nil), newFakeOS(), t.TempDir(), nil).Delete("anthropic"); !errors.Is(err, ErrNoKey) {
			t.Fatalf("Delete = %v, want ErrNoKey", err)
		}
	})
}

func TestEnvVar(t *testing.T) {
	for in, want := range map[string]string{"anthropic": "ANTHROPIC_API_KEY", "open-ai": "OPEN_AI_API_KEY"} {
		if got := EnvVar(in); got != want {
			t.Errorf("EnvVar(%q) = %q, want %q", in, got, want)
		}
	}
}

// crashEnv makes TestCrashKeepsOldFile act as the child that dies halfway
// through a write.
const crashEnv = "SNOW_CREDENTIAL_CRASH_DIR"

// TestCrashKeepsOldFile: a process that dies in the middle of saving keys
// leaves the old file whole.
func TestCrashKeepsOldFile(t *testing.T) {
	if dir := os.Getenv(crashEnv); dir != "" {
		afterLine = func() { os.Exit(3) }
		_ = writeFile(dir, filepath.Join(dir, fileName), map[string]string{"anthropic": "new-1", "openai": "new-2"})
		t.Fatal("the write returned instead of crashing")
	}
	dir := t.TempDir()
	writeKeys(t, dir, map[string]string{"anthropic": "old-1", "openai": "old-2"})
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(self, "-test.run=^TestCrashKeepsOldFile$")
	cmd.Env = append(os.Environ(), crashEnv+"="+dir)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("child did not crash mid-write: %v\n%s", err, out)
	}
	keys, err := readFile(filepath.Join(dir, fileName))
	if err != nil || keys["anthropic"] != "old-1" || keys["openai"] != "old-2" {
		t.Fatalf("after the crash the file holds %v, %v", len(keys), err)
	}
}
