//go:build !windows

package clauderuntime

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/shell"
)

// TestWSLScriptRuns runs the script the WSL launch uses under a real sh,
// as the distribution would, with a fake claude that prints what it got:
// the private folder exists with owner-only permissions while Claude keeps
// the user's umask, inherited Claude and Anthropic variables are gone
// except the Git Bash path, Snow's forwarded variables arrive under their
// real names, and WSLENV passes the pane variables to snow.exe.
func TestWSLScriptRuns(t *testing.T) {
	home, bin := t.TempDir(), t.TempDir()
	fake := "#!/bin/sh\necho \"dir=$CLAUDE_CONFIG_DIR\"\necho \"key=$ANTHROPIC_API_KEY\"\necho \"base=$ANTHROPIC_BASE_URL\"\n" +
		"echo \"other=$CLAUDE_CODE_ENTRYPOINT\"\necho \"gitbash=$CLAUDE_CODE_GIT_BASH_PATH\"\necho \"wslenv=$WSLENV\"\n" +
		"echo \"umask=$(umask)\"\necho \"args=$*\"\n"
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(fake), 0o700); err != nil {
		t.Fatal(err)
	}
	path := bin + ":/usr/bin:/bin"
	// The script finds claude through the user's login shell. A login
	// profile may reset PATH, so the temporary home sets it back.
	for _, profile := range []string{".profile", ".bash_profile", ".zprofile"} {
		if err := os.WriteFile(filepath.Join(home, profile), []byte("export PATH=\""+path+"\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	inherited, err := exec.Command("sh", "-c", "umask").Output()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", wslScript, "snow-claude", "c1", "auth", "status", "--json")
	cmd.Env = []string{"HOME=" + home, "SHELL=/bin/sh", "PATH=" + path,
		"ANTHROPIC_API_KEY=inherited", "ANTHROPIC_BASE_URL=https://example.invalid", "CLAUDE_CODE_ENTRYPOINT=cli",
		"CLAUDE_CODE_GIT_BASH_PATH=/opt/bash", "WSLENV=USERS_OWN",
		wslVar + "ANTHROPIC_API_KEY=saved-key", wslVar + "CLAUDE_CODE_SDK_READS_SESSION_STATE=1"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("script failed: %v\n%s", err, out)
	}
	dir := filepath.Join(home, wslConfigBase, "c1", "claude")
	want := []string{"dir=" + dir, "key=saved-key", "base=", "other=", "gitbash=/opt/bash",
		"wslenv=SNOW_PANE:SNOW_SOCKET", "umask=" + strings.TrimSpace(string(inherited)), "args=auth status --json"}
	got := strings.Split(strings.TrimSpace(string(out)), "\n")
	if strings.Join(got[len(got)-len(want):], "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", out, strings.Join(want, "\n"))
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("folder mode = %v, want 0700", info.Mode().Perm())
	}
}

// TestWSLRemoveScriptRuns runs RemoveLaunch's script under a real sh: it
// deletes only its own connection's folder, and only for a WSL connection.
func TestWSLRemoveScriptRuns(t *testing.T) {
	home := t.TempDir()
	for _, id := range []string{"c1", "c2"} {
		if err := os.MkdirAll(filepath.Join(home, wslConfigBase, id, "claude"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	ubuntu := shell.Profile{Kind: shell.KindClaude, Path: "wsl.exe", Args: []string{"-d", "Ubuntu"}, Distro: "Ubuntu"}
	profile := Profile{ID: "c1", Distro: "Ubuntu"}
	launch, err := profile.RemoveLaunch(ubuntu)
	if err != nil {
		t.Fatal(err)
	}
	// wsl.exe -d Ubuntu -e sh -c <script> snow-claude c1: run what follows -e.
	args := launch.Args[slices.Index(launch.Args, "-e")+1:]
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin"}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("script failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(home, wslConfigBase, "c1")); !os.IsNotExist(err) {
		t.Fatalf("c1 still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, wslConfigBase, "c2", "claude")); err != nil {
		t.Fatalf("another connection's folder went: %v", err)
	}
	if _, err := (Profile{ID: "c1"}).RemoveLaunch(shell.Profile{Path: "/usr/bin/claude"}); err == nil {
		t.Fatal("a connection on this machine got a WSL remove")
	}
}
