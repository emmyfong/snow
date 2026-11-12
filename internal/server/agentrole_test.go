package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/store"
	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// helperArgs names the file the helper agent writes its arguments to.
const helperArgs = "SNOW_HELPER_ARGS"

// TestHelperAgentArgs is not a test: a role test's "Claude Code" pane runs
// the test binary with -test.run set to it. It writes the arguments after
// "--", one per line, to the file helperArgs names, then stays running as
// an agent would until the server ends it.
func TestHelperAgentArgs(t *testing.T) {
	out := os.Getenv(helperArgs)
	if out == "" {
		t.Skip("runs only as a role test's agent")
	}
	var args []string
	for i, a := range os.Args {
		if a == "--" {
			args = os.Args[i+1:]
			break
		}
	}
	if err := os.WriteFile(out, []byte(strings.Join(args, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Minute)
}

// argsServer is agentServer whose "Claude Code" pane is TestHelperAgentArgs,
// which writes the arguments it got, one per line, to the returned file.
// It runs on every system, so the arguments a Windows launch builds are
// checked too.
func argsServer(t *testing.T, roles []Role) (*Server, string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "args")
	opts := claudeOptions(nil, nil)
	shell := opts.Spec
	opts.agentProfile = func(p string) bool { return strings.HasPrefix(p, "Claude Code") }
	opts.Spec = func(profile, dir string) (term.Spec, string, error) {
		// Only agent panes write the file. A shell pane that wrote it too
		// would race the agent, and the file would hold the shell's empty
		// arguments (#356).
		if !opts.agentProfile(profile) {
			return shell(profile, dir)
		}
		return term.Spec{Path: exe, Args: []string{"-test.run=^TestHelperAgentArgs$", "--"}, Env: []string{helperArgs + "=" + out}, Dir: dir}, profile, nil
	}
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

// TestRoleRestored: a saved agent started in a role starts again in it, so
// a read-only agent does not come back unrestricted after a restart.
func TestRoleRestored(t *testing.T) {
	dir := t.TempDir() // before the server, so its shells have ended when the folder goes
	s, out := argsServer(t, []Role{{Name: "reviewer", Title: "Reviewer", Instructions: "Review.", PermissionMode: "plan"}})
	c := connect(t, s, 100, 30)
	first := attachIn(t, c, dir)
	cmd := agentNew(first, "Claude Code", "")
	cmd.Args[api.ArgRole] = "reviewer"
	c.Send(cmd)
	agentWindow(t, c, 2)
	eventually(t, "the first launch's arguments", func() bool {
		b, err := os.ReadFile(out)
		return err == nil && strings.Count(string(b), "\n") == 4
	})
	if err := os.Remove(out); err != nil {
		t.Fatal(err)
	}

	var snap store.Snapshot
	s.call(func(st *state) {
		snap = st.snapshot(st.sessions["work"])
		snap.Name = "copy"
	})
	roles := s.restoreRoles(&snap)
	var role *api.RoleInfo
	s.call(func(st *state) {
		sess, err := st.restore(&snap, nil, roles, 100, 30)
		if err != nil {
			t.Errorf("restore: %v", err)
			return
		}
		st.sessions[sess.name] = sess
		for _, w := range sess.windows {
			for _, p := range w.panes {
				if p.role != nil {
					role = p.role
				}
			}
		}
	})
	if role == nil || role.Name != "reviewer" || role.Title != "Reviewer" || !role.ReadOnly {
		t.Fatalf("restored role %+v, want the reviewer, read-only", role)
	}
	eventually(t, "the restored agent's arguments", func() bool {
		b, err := os.ReadFile(out)
		return err == nil && strings.HasPrefix(string(b), "--permission-mode\nplan\n--append-system-prompt-file\n")
	})
}
