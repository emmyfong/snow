//go:build !windows

package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// argsServer is agentServer whose "Claude Code" pane writes the arguments
// it got, one per line, to the returned file.
func argsServer(t *testing.T, roles []Role) (*Server, string) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "args")
	opts := claudeOptions(nil, nil)
	opts.Spec = func(profile, dir string) (term.Spec, string, error) {
		script := `for a; do printf '%s\n' "$a"; done > "` + out + `"; exec sleep 60`
		return term.Spec{Path: "/bin/sh", Args: []string{"-c", script, "sh"}, Dir: dir}, profile, nil
	}
	opts.agentProfile = func(p string) bool { return strings.HasPrefix(p, "Claude Code") }
	opts.Roles, opts.RolesDir = roles, t.TempDir()
	s := New(opts)
	t.Cleanup(s.Close)
	return s, out
}

// TestRoleProfileInstructions: an agent started in a role gets that role's
// permission mode and only that role's instructions, from a file.
func TestRoleProfileInstructions(t *testing.T) {
	dir := t.TempDir() // before the server, so its shells have ended when the folder goes
	s, out := argsServer(t, []Role{
		{Name: "reviewer", Instructions: "Review the diff. Report findings.", PermissionMode: "plan"},
		{Name: "coder", Instructions: "Fix the bug."},
	})
	c := connect(t, s, 100, 30)
	first := attachIn(t, c, dir)
	cmd := agentNew(first, "Claude Code", "")
	cmd.Args[api.ArgRole] = "reviewer"
	c.Send(cmd)
	agentWindow(t, c, 2)
	var args []string
	eventually(t, "the agent's arguments", func() bool {
		b, err := os.ReadFile(out)
		args = strings.Split(strings.TrimSpace(string(b)), "\n")
		return err == nil && len(args) == 4
	})
	if args[0] != "--permission-mode" || args[1] != "plan" || args[2] != "--append-system-prompt-file" {
		t.Fatalf("args %q, want the plan mode and an instructions file", args)
	}
	b, err := os.ReadFile(args[3])
	if err != nil || string(b) != "Review the diff. Report findings." {
		t.Fatalf("instructions file holds %q, %v; want the reviewer's only", b, err)
	}
	board := nextAgents(t, c, func(l []api.AgentInfo) bool { return len(l) == 1 && l[0].Role != "" })
	if board[0].Role != "reviewer" || board[0].PermissionMode != "plan" || !board[0].ReadOnly {
		t.Fatalf("board %+v, want the reviewer role, read-only", board[0])
	}
}
