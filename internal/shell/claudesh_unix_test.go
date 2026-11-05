//go:build !windows

package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestClaudeScriptPassesArguments: the WSL launcher hands its arguments to
// claude unchanged, a blank inside one included, through each kind of
// login shell. A stub getent names the login shell. A login sh reads
// /etc/profile, which may reset PATH, so the fake claude goes on PATH
// again from the test HOME's .profile. The fish stub runs its command as
// fish would: $argv is the arguments after the command.
func TestClaudeScriptPassesArguments(t *testing.T) {
	h, err := WriteHooks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bin, home := t.TempDir(), t.TempDir()
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(bin, "claude"), "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done > \"$SNOW_TEST_OUT\"\n")
	write(filepath.Join(bin, "fish"), "#!/bin/sh\n[ \"$1\" = -lic ] && [ \"$2\" = 'exec claude $argv' ] || exit 9\nshift 2\nexec claude \"$@\"\n")
	write(filepath.Join(home, ".profile"), "PATH="+bin+":$PATH\n")
	want := []string{"--permission-mode", "plan", "--append-system-prompt-file", "/tmp/a role.md"}
	for _, login := range []string{"/bin/sh", filepath.Join(bin, "fish")} {
		t.Run(filepath.Base(login), func(t *testing.T) {
			write(filepath.Join(bin, "getent"), "#!/bin/sh\necho \"u:x:1000:1000::"+home+":"+login+"\"\n")
			out := filepath.Join(t.TempDir(), "args")
			cmd := exec.Command("/bin/sh", append([]string{filepath.Join(h.Dir, "claude.sh")}, want...)...)
			cmd.Env = append(os.Environ(), "HOME="+home, "PATH="+bin+":"+os.Getenv("PATH"), "SNOW_TEST_OUT="+out, "ENV=")
			if msg, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("claude.sh: %v\n%s", err, msg)
			}
			b, err := os.ReadFile(out)
			if err != nil {
				t.Fatalf("claude did not run: %v", err)
			}
			got := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
			if strings.Join(got, "|") != strings.Join(want, "|") {
				t.Fatalf("claude got %q, want %q", got, want)
			}
		})
	}
}
