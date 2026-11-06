//go:build !windows

package clauderuntime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestWSLScriptRuns runs the script the WSL launch uses under a real sh,
// as the distribution would, with a fake claude that prints what it got:
// the private folder exists with owner-only permissions, inherited Claude
// and Anthropic variables are gone, and Snow's forwarded variables arrive
// under their real names.
func TestWSLScriptRuns(t *testing.T) {
	home, bin := t.TempDir(), t.TempDir()
	fake := "#!/bin/sh\necho \"dir=$CLAUDE_CONFIG_DIR\"\necho \"key=$ANTHROPIC_API_KEY\"\necho \"base=$ANTHROPIC_BASE_URL\"\necho \"other=$CLAUDE_CODE_ENTRYPOINT\"\necho \"args=$*\"\n"
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(fake), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", wslScript, "snow-claude", "c1", "auth", "status", "--json")
	cmd.Env = []string{"HOME=" + home, "SHELL=/bin/sh", "PATH=" + bin + ":/usr/bin:/bin",
		"ANTHROPIC_API_KEY=inherited", "ANTHROPIC_BASE_URL=https://example.invalid", "CLAUDE_CODE_ENTRYPOINT=cli",
		wslVar + "ANTHROPIC_API_KEY=saved-key", wslVar + "CLAUDE_CODE_SDK_READS_SESSION_STATE=1"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("script failed: %v\n%s", err, out)
	}
	dir := filepath.Join(home, wslConfigBase, "c1", "claude")
	want := []string{"dir=" + dir, "key=saved-key", "base=", "other=", "args=auth status --json"}
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
