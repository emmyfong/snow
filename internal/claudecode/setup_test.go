package claudecode

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func answer(s string) *bufio.Reader { return bufio.NewReader(strings.NewReader(s)) }

// TestClaudeSetupAsks: without a "y", nothing is written; with one, the
// hooks are written and the old file is kept as a copy.
func TestClaudeSetupAsks(t *testing.T) {
	for _, no := range []string{"", "\n", "n\n", "no\n", "maybe\n"} {
		path := filepath.Join(t.TempDir(), "settings.json")
		if err := os.WriteFile(path, []byte(otherHooks), 0o600); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err := Setup(FileTarget{Path: path, Cmd: snowCmd}, MCP{}, answer(no), &out); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(path); string(got) != otherHooks {
			t.Fatalf("answer %q changed the file", no)
		}
		if _, err := os.Stat(path + ".snow.bak"); err == nil {
			t.Fatalf("answer %q wrote a backup", no)
		}
		if !strings.Contains(out.String(), snowCmd) || !strings.Contains(out.String(), "[y/N]") {
			t.Fatalf("answer %q: the change was not shown:\n%s", no, out.String())
		}
	}

	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(otherHooks), 0o640); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Setup(FileTarget{Path: path, Cmd: snowCmd}, MCP{}, answer("y\n"), &out); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if st, err := HookStatus(got); err != nil || !st.Set() {
		t.Fatalf("after yes: %+v, %v", st, err)
	}
	if b, _ := os.ReadFile(path + ".snow.bak"); string(b) != otherHooks {
		t.Fatalf("backup %q, want the old file", b)
	}
	if fi, _ := os.Stat(path); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o640 {
		t.Fatalf("mode %v, want the file's own 0640", fi.Mode().Perm())
	}
	out.Reset()
	if err := Setup(FileTarget{Path: path, Cmd: snowCmd}, MCP{}, answer(""), &out); err != nil || !strings.Contains(out.String(), "already set") {
		t.Fatalf("second setup: %v, %q", err, out.String())
	}
}

// TestClaudeSetupNoFile: setup creates the folder and the file.
func TestClaudeSetupNoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude", "settings.json")
	var out bytes.Buffer
	if err := Setup(FileTarget{Path: path, Cmd: snowCmd}, MCP{}, answer("yes\n"), &out); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || !strings.Contains(string(got), snowCmd) {
		t.Fatalf("file %q, %v", got, err)
	}
	if strings.Contains(out.String(), "copy") {
		t.Fatalf("promised a copy of a file that did not exist:\n%s", out.String())
	}
}

// TestClaudeSetupRefusesUnreadable: a broken file is left as it is.
func TestClaudeSetupRefusesUnreadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	broken := `{"model": "opus",`
	if err := os.WriteFile(path, []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := Setup(FileTarget{Path: path, Cmd: snowCmd}, MCP{}, answer("y\n"), &out)
	if !errors.Is(err, ErrUnreadable) {
		t.Fatalf("Setup() = %v, want ErrUnreadable", err)
	}
	if got, _ := os.ReadFile(path); string(got) != broken {
		t.Fatal("the broken file changed")
	}
}

func TestClaudeRemoveAsks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	with, _, _ := AddHooks([]byte(otherHooks), snowCmd)
	if err := os.WriteFile(path, with, 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Remove(FileTarget{Path: path, Cmd: snowCmd}, MCP{}, answer("y\n"), &out); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != otherHooks {
		t.Fatalf("after remove:\n%s", got)
	}
}

func TestReport(t *testing.T) {
	dir := t.TempDir()
	full, _, _ := AddHooks(nil, snowCmd)
	tests := []struct{ body, want string }{
		{"", "not set"},
		{string(full), "are set"},
		{strings.Replace(string(full), `"Stop"`, `"X"`, 1), "missing Stop"},
		{"{", "not JSON"},
	}
	for _, tt := range tests {
		path := filepath.Join(dir, "settings.json")
		if err := os.WriteFile(path, []byte(tt.body), 0o600); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err := Report(FileTarget{Path: path}, MCP{}, &out); err != nil || !strings.Contains(out.String(), tt.want) {
			t.Errorf("Report(%.20q) = %q, %v; want %q", tt.body, out.String(), err, tt.want)
		}
	}
}

// TestWSLTarget runs the scripts WSLTarget sends to wsl.exe with this
// machine's sh, in a home folder of the test's own.
func TestWSLTarget(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	tgt := WSLTarget{Distro: "Ubuntu", Cmd: `'/mnt/c/snow/snow.exe' hook`, run: func(args []string, stdin []byte) ([]byte, error) {
		if len(args) < 6 || args[0] != "-d" || args[1] != "Ubuntu" || args[2] != "-e" || args[3] != "sh" || args[4] != "-c" {
			t.Fatalf("wsl.exe args %q", args)
		}
		cmd := exec.Command(sh, append([]string{"-c", args[5]}, args[6:]...)...)
		cmd.Env = append(os.Environ(), "HOME="+home, "CLAUDE_CONFIG_DIR=")
		cmd.Stdin = bytes.NewReader(stdin)
		return cmd.Output()
	}}
	if got, err := tgt.Read(); err != nil || got != nil {
		t.Fatalf("Read() of no file = %q, %v; want nil", got, err)
	}
	if err := Setup(tgt, MCP{}, answer("y\n"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	got, err := tgt.Read()
	if err != nil || !strings.Contains(string(got), `'/mnt/c/snow/snow.exe' hook`) {
		t.Fatalf("Read() after setup = %q, %v", got, err)
	}
	if err := Remove(tgt, MCP{}, answer("y\n"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(home, ".claude", "settings.json.snow.bak")); !bytes.Equal(b, got) {
		t.Fatalf("backup %q, want the file before remove", b)
	}
	if b, _ := os.ReadFile(filepath.Join(home, ".claude", "settings.json")); string(b) != "{}\n" {
		t.Fatalf("after remove %q", b)
	}
}

// TestClaudeSetupShowsDiff: the user sees the exact change before the
// question.
func TestClaudeSetupShowsDiff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(otherHooks), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Setup(FileTarget{Path: path, Cmd: snowCmd}, MCP{}, answer("n\n"), &out); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"--- " + path, "+++ " + path, "@@ ", `+            "command": "'/usr/local/bin/snow' hook",`, `+    "Stop": [`} {
		if !strings.Contains(s, want) {
			t.Fatalf("output lacks %q:\n%s", want, s)
		}
	}
	if strings.Index(s, "@@ ") > strings.Index(s, "[y/N]") {
		t.Fatalf("the diff comes after the question:\n%s", s)
	}
}

// TestClaudeSetupFollowsSymlink: a settings file that is a link, as in a
// dotfiles folder, stays a link; the file it points to gets the change.
func TestClaudeSetupFollowsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need a privilege on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles", "claude-settings.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(otherHooks), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "settings.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := Setup(FileTarget{Path: link, Cmd: snowCmd}, MCP{}, answer("y\n"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the link was replaced: %v, %v", fi, err)
	}
	if got, _ := os.ReadFile(target); !strings.Contains(string(got), snowCmd) {
		t.Fatalf("the linked file has no hook:\n%s", got)
	}
	if b, err := os.ReadFile(target + ".snow.bak"); err != nil || string(b) != otherHooks {
		t.Fatalf("backup next to the target file: %q, %v", b, err)
	}
}

// TestClaudeSetupKeepsFirstBackup: the copy keeps the file from before
// Snow ever changed it.
func TestClaudeSetupKeepsFirstBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(otherHooks), 0o600); err != nil {
		t.Fatal(err)
	}
	tgt := FileTarget{Path: path, Cmd: snowCmd}
	for _, f := range []func(Target, MCP, *bufio.Reader, io.Writer) error{Setup, Remove, Setup} {
		if err := f(tgt, MCP{}, answer("y\n"), &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
	}
	if b, _ := os.ReadFile(path + ".snow.bak"); string(b) != otherHooks {
		t.Fatalf("backup %q, want the file before Snow", b)
	}
}

// changingReader answers yes, but first changes the file, as the user's
// editor or Claude Code may while the question waits.
type changingReader struct {
	path string
	done bool
}

func (r *changingReader) Read(b []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}
	r.done = true
	if err := os.WriteFile(r.path, []byte(`{"model": "sonnet"}`), 0o600); err != nil {
		return 0, err
	}
	return copy(b, "y\n"), nil
}

// TestClaudeSetupRefusesChangedFile: a file that changed while the
// question waited is not overwritten.
func TestClaudeSetupRefusesChangedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(otherHooks), 0o600); err != nil {
		t.Fatal(err)
	}
	err := Setup(FileTarget{Path: path, Cmd: snowCmd}, MCP{}, bufio.NewReader(&changingReader{path: path}), &bytes.Buffer{})
	if !errors.Is(err, ErrChanged) {
		t.Fatalf("Setup() = %v, want ErrChanged", err)
	}
	if got, _ := os.ReadFile(path); string(got) != `{"model": "sonnet"}` {
		t.Fatalf("the newer file was overwritten:\n%s", got)
	}
}

// TestWSLTargetSafeWrite: inside the distribution the file keeps its
// private mode, and input cut short leaves the file as it was.
func TestWSLTargetSafeWrite(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil || runtime.GOOS == "windows" {
		t.Skip("needs a POSIX sh")
	}
	home := t.TempDir()
	cut := false
	tgt := WSLTarget{Distro: "Ubuntu", Cmd: snowCmd, run: func(args []string, stdin []byte) ([]byte, error) {
		if cut {
			stdin = stdin[:len(stdin)/2]
		}
		cmd := exec.Command(sh, append([]string{"-c", args[5]}, args[6:]...)...)
		cmd.Env = append(os.Environ(), "HOME="+home, "CLAUDE_CONFIG_DIR=")
		cmd.Stdin = bytes.NewReader(stdin)
		return cmd.Output()
	}}
	path := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(otherHooks), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Setup(tgt, MCP{}, answer("y\n"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v after setup, want 0600", fi.Mode().Perm())
	}
	before, _ := os.ReadFile(path)
	cut = true
	if err := Remove(tgt, MCP{}, answer("y\n"), &bytes.Buffer{}); err == nil {
		t.Fatal("Remove() = nil with its input cut short")
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(after, before) {
		t.Fatalf("a cut write changed the file:\n%s", after)
	}
}
