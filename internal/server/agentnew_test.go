package server

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/gittest"
	"github.com/emmyfong/snow/internal/store"
	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// brokenClaude is an agent profile whose program cannot start.
const brokenClaude = "Claude Code broken"

// agentServer is a server whose "Claude Code" profiles run the test shell
// and count as agents; brokenClaude fails to start.
func agentServer(t *testing.T) *Server {
	t.Helper()
	opts := claudeOptions(nil, nil)
	spec := opts.Spec
	opts.Spec = func(profile, dir string) (term.Spec, string, error) {
		if profile == brokenClaude {
			return term.Spec{}, "", errors.New("no such program")
		}
		return spec(profile, dir)
	}
	opts.agentProfile = func(p string) bool { return strings.HasPrefix(p, "Claude Code") }
	s := New(opts)
	t.Cleanup(s.Close)
	return s
}

// attachIn attaches c to a new session in dir and returns its first pane.
func attachIn(t *testing.T, c *apitest.Client, dir string) int {
	t.Helper()
	c.Send(&api.Attach{Session: "work", Create: true, Dir: dir})
	return nextLayout(t, c, func(*api.Layout) bool { return true }).Windows[0].Panes[0].ID
}

// agentFolder returns the folder the board shows for pane id. The layout
// shows only a folder the shell reports, which the Windows test shell does
// not; the board falls back to the folder the pane started in.
func agentFolder(t *testing.T, board []api.AgentInfo, id int) string {
	t.Helper()
	for _, a := range board {
		if a.Pane == id {
			return a.Folder
		}
	}
	t.Fatalf("pane %d is not on the board %+v", id, board)
	return ""
}

func agentNew(from int, profile, branch string) *api.Command {
	args := map[string]string{api.ArgProfile: profile}
	if branch != "" {
		args[api.ArgBranch] = branch
	}
	return &api.Command{Op: api.OpAgentNew, Target: from, Args: args}
}

// agentWindow waits for a layout with n windows and returns the newest
// window's only pane, and the window's name.
func agentWindow(t *testing.T, c *apitest.Client, n int) (api.PaneInfo, string) {
	t.Helper()
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(l.Windows) == n })
	w := l.Windows[len(l.Windows)-1]
	return w.Panes[0], w.Name
}

func realPath(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// TestAgentWorktreeLaunch: two agents started in one repository get two
// worktrees on two new branches, each in its own window, and the main
// checkout stays as it was.
func TestAgentWorktreeLaunch(t *testing.T) {
	repo := t.TempDir() // before the server, so its shells have ended when the folder goes
	gittest.Repo(t, repo)
	s := agentServer(t)
	c := connect(t, s, 100, 30)
	first := attachIn(t, c, repo)

	c.Send(agentNew(first, "Claude Code", ""))
	a, nameA := agentWindow(t, c, 2)
	c.Send(agentNew(first, "Claude Code", "fix-login"))
	b, nameB := agentWindow(t, c, 3)

	if a.Branch != "agent/1" || nameA != "agent/1" || b.Branch != "fix-login" || nameB != "fix-login" {
		t.Fatalf("branches %q (%q), %q (%q); want agent/1 and fix-login as branch and window name", a.Branch, nameA, b.Branch, nameB)
	}
	board := nextAgents(t, c, func(l []api.AgentInfo) bool { return len(l) == 2 && l[1].Branch != "" })
	for _, p := range []api.PaneInfo{a, b} {
		want := filepath.Join(realPath(t, repo), ".worktrees", strings.ReplaceAll(p.Branch, "/", "-"))
		if realPath(t, p.Worktree) != want || realPath(t, agentFolder(t, board, p.ID)) != want {
			t.Fatalf("pane %+v, want worktree and folder %s", p, want)
		}
	}
	if got := gittest.Run(t, repo, "branch", "--show-current"); got != "main" {
		t.Fatalf("main checkout on %q", got)
	}
	if got := gittest.Run(t, repo, "status", "--porcelain"); got != "" {
		t.Fatalf("main checkout changed:\n%s", got)
	}
	if board[0].Branch != "agent/1" || board[1].Worktree != b.Worktree {
		t.Fatalf("board %+v, want each agent's branch and worktree", board)
	}
}

// TestAgentWorktreeLaunchFailure: an agent that cannot start leaves no
// window, no worktree, and no branch, and the error says why. A bad branch
// name is refused with the worktree's typed error.
func TestAgentWorktreeLaunchFailure(t *testing.T) {
	repo := t.TempDir() // before the server, so its shells have ended when the folder goes
	gittest.Repo(t, repo)
	s := agentServer(t)
	c := connect(t, s, 100, 30)
	first := attachIn(t, c, repo)

	c.Send(agentNew(first, brokenClaude, ""))
	if e := nextError(t, c); e.Code != api.CodeCommandFailed || !strings.Contains(e.Message, "no such program") {
		t.Fatalf("got %+v, want the start failure", e)
	}
	eventually(t, "the worktree and branch to go", func() bool {
		_, err := os.Stat(filepath.Join(repo, ".worktrees", "agent-1"))
		return os.IsNotExist(err) && gittest.Run(t, repo, "branch", "--list", "agent/1") == ""
	})
	c.Send(agentNew(first, "Claude Code", "-bad"))
	if e := nextError(t, c); e.Code != api.CodeBadBranch || !strings.Contains(e.Message, "not a valid branch name") {
		t.Fatalf("got %+v, want the bad branch refused", e)
	}
	var windows int
	s.call(func(st *state) { windows = len(st.sessions["work"].windows) })
	if windows != 1 {
		t.Fatalf("%d windows after failed launches, want 1", windows)
	}
}

// TestSecondAgentSameFolderRefused: in a folder that is not a git
// repository, the first agent works in the folder and a second is refused
// with an error that names the first.
func TestSecondAgentSameFolderRefused(t *testing.T) {
	dir := t.TempDir() // before the server, so its shells have ended when the folder goes
	gittest.Isolate(t)
	s := agentServer(t)
	c := connect(t, s, 100, 30)
	first := attachIn(t, c, dir)
	c.Send(agentNew(first, "Claude Code", ""))
	p, _ := agentWindow(t, c, 2)
	board := nextAgents(t, c, func(l []api.AgentInfo) bool { return len(l) == 1 })
	if p.Worktree != "" || agentFolder(t, board, p.ID) != dir {
		t.Fatalf("pane %+v, want the folder itself and no worktree", p)
	}
	c.Send(agentNew(first, "Claude Code", ""))
	e := nextError(t, c)
	if e.Code != api.CodeFolderBusy || !strings.Contains(e.Message, ErrFolderBusy.Error()) || !strings.Contains(e.Message, "window 1") {
		t.Fatalf("got %+v, want the busy folder refused, naming window 1", e)
	}
}

// TestAgentNewNeedsAgentProfile: a shell profile is refused.
func TestAgentNewNeedsAgentProfile(t *testing.T) {
	dir := t.TempDir() // before the server, so its shells have ended when the folder goes
	s := agentServer(t)
	c := connect(t, s, 100, 30)
	first := attachIn(t, c, dir)
	c.Send(agentNew(first, shellProfile(), ""))
	if e := nextError(t, c); !strings.Contains(e.Message, ErrNotAgent.Error()) {
		t.Fatalf("got %+v, want the profile refused", e)
	}
}

// TestAgentRemovalKeepsBranch: closing an agent's pane with the worktree
// option removes the worktree and keeps the branch. A worktree with
// changes stays, and the close says why. A plain close keeps the worktree.
func TestAgentRemovalKeepsBranch(t *testing.T) {
	repo := t.TempDir() // before the server, so its shells have ended when the folder goes
	gittest.Repo(t, repo)
	s := agentServer(t)
	c := connect(t, s, 100, 30)
	first := attachIn(t, c, repo)
	closeWith := func(id int, remove bool) {
		cmd := &api.Command{Op: api.OpClose, Target: id}
		if remove {
			cmd.Args = map[string]string{api.ArgWorktree: api.WorktreeRemove}
		}
		c.Send(cmd)
	}

	c.Send(agentNew(first, "Claude Code", ""))
	clean, _ := agentWindow(t, c, 2)
	closeWith(clean.ID, true)
	eventually(t, "the clean worktree to go", func() bool {
		_, err := os.Stat(clean.Worktree)
		return os.IsNotExist(err)
	})
	if gittest.Run(t, repo, "branch", "--list", "agent/1") == "" {
		t.Fatal("the branch went with the worktree")
	}

	c.Send(agentNew(first, "Claude Code", ""))
	dirty, _ := agentWindow(t, c, 2)
	if err := os.WriteFile(filepath.Join(dirty.Worktree, "work.txt"), []byte("work"), 0o644); err != nil {
		t.Fatal(err)
	}
	closeWith(dirty.ID, true)
	if e := nextError(t, c); e.Code != api.CodeNotice || !strings.Contains(e.Message, "uncommitted changes") {
		t.Fatalf("got %+v, want a notice that the worktree was kept", e)
	}
	if _, err := os.Stat(dirty.Worktree); err != nil {
		t.Fatalf("the worktree with changes is gone: %v", err)
	}

	c.Send(agentNew(first, "Claude Code", ""))
	kept, _ := agentWindow(t, c, 2)
	closeWith(kept.ID, false)
	nextLayout(t, c, func(l *api.Layout) bool { return len(l.Windows) == 1 })
	if _, err := os.Stat(kept.Worktree); err != nil {
		t.Fatalf("a plain close removed the worktree: %v", err)
	}
}

// TestAgentWorktreeSaved: a saved session keeps each agent's worktree and
// branch, and a restored pane has them again.
func TestAgentWorktreeSaved(t *testing.T) {
	repo := t.TempDir() // before the server, so its shells have ended when the folder goes
	gittest.Repo(t, repo)
	s := agentServer(t)
	c := connect(t, s, 100, 30)
	first := attachIn(t, c, repo)
	c.Send(agentNew(first, "Claude Code", ""))
	p, _ := agentWindow(t, c, 2)

	var restored *pane
	s.call(func(st *state) {
		snap := st.snapshot(st.sessions["work"])
		snap.Name = "copy"
		var saved store.Pane
		for _, w := range snap.Windows {
			for _, sp := range w.Panes {
				if sp.ID == p.ID {
					saved = sp
				}
			}
		}
		if saved.Worktree != p.Worktree || saved.Branch != "agent/1" {
			t.Errorf("saved pane %+v, want worktree %s on agent/1", saved, p.Worktree)
			return
		}
		sess, err := st.restore(&snap, nil, nil, 100, 30)
		if err != nil {
			t.Errorf("restore: %v", err)
			return
		}
		st.sessions[sess.name] = sess
		for _, w := range sess.windows {
			for _, rp := range w.panes {
				if rp.branch == "agent/1" {
					restored = rp
				}
			}
		}
	})
	if restored == nil || restored.worktree != p.Worktree {
		t.Fatalf("restored pane %+v, want the worktree back", restored)
	}
}

// TestRolePermissionCapability: only a role whose permission mode Claude
// Code enforces is read-only. Instructions that ask for read-only work do
// not make it so.
func TestRolePermissionCapability(t *testing.T) {
	tests := []struct {
		role     Role
		readOnly bool
	}{
		{Role{Name: "asks", Instructions: "Read only. Never edit files."}, false},
		{Role{Name: "edits", PermissionMode: "acceptEdits"}, false},
		{Role{Name: "plan", PermissionMode: "plan"}, true},
	}
	for _, tt := range tests {
		info := tt.role.info()
		if info.ReadOnly != tt.readOnly {
			t.Errorf("%s: read-only %v, want %v", tt.role.Name, info.ReadOnly, tt.readOnly)
		}
	}
	opts := claudeOptions(nil, nil)
	opts.Roles = []Role{tests[0].role, tests[2].role}
	s := New(opts)
	t.Cleanup(s.Close)
	var l *api.ProfileList
	s.call(func(st *state) { l = st.profileList() })
	if len(l.Roles) != 2 || l.Roles[0].ReadOnly || !l.Roles[0].HasInstructions || !l.Roles[1].ReadOnly {
		t.Fatalf("roles %+v", l.Roles)
	}
}

// TestAgentNewUnknownRole: a role the server does not have is refused
// before any worktree is made.
func TestAgentNewUnknownRole(t *testing.T) {
	repo := t.TempDir() // before the server, so its shells have ended when the folder goes
	gittest.Repo(t, repo)
	s := agentServer(t)
	c := connect(t, s, 100, 30)
	first := attachIn(t, c, repo)
	cmd := agentNew(first, "Claude Code", "")
	cmd.Args[api.ArgRole] = "nobody"
	c.Send(cmd)
	if e := nextError(t, c); !strings.Contains(e.Message, ErrUnknownRole.Error()) {
		t.Fatalf("got %+v, want the role refused", e)
	}
	if got := gittest.Run(t, repo, "branch", "--list", "agent/*"); got != "" {
		t.Fatalf("branches %q after a refused launch", got)
	}
}

// TestRoleFileNames: names that differ only in characters a file name
// cannot hold get different files, each safe to pass to cmd, and changed
// instructions get a new file.
func TestRoleFileNames(t *testing.T) {
	a, b := roleFile(Role{Name: "Code Reviewer"}), roleFile(Role{Name: "Code-Reviewer"})
	if a == b {
		t.Fatalf("both roles write %s", a)
	}
	if roleFile(Role{Name: "r", Instructions: "one"}) == roleFile(Role{Name: "r", Instructions: "two"}) {
		t.Fatal("new instructions reuse the old file")
	}
	for _, f := range []string{a, b, roleFile(Role{Name: `a/b\c:%!"`})} {
		if strings.ContainsAny(f, `/\:*?"<>|% !`) {
			t.Fatalf("file name %q holds an unsafe character", f)
		}
	}
}

// TestRoleFileAtOnce: launches that write one role's file at the same time
// all get the whole text.
func TestRoleFileAtOnce(t *testing.T) {
	s := &Server{opts: Options{RolesDir: t.TempDir()}}
	r := Role{Name: "reviewer", Instructions: strings.Repeat("Review the diff. ", 4000)}
	const n = 16
	paths, errs := make([]string, n), make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			args, err := s.roleArgs(r, "")
			errs[i] = err
			if err == nil {
				paths[i] = args[len(args)-1]
			}
		}()
	}
	wg.Wait()
	for i := range n {
		if errs[i] != nil {
			t.Fatalf("launch %d: %v", i, errs[i])
		}
		if b, err := os.ReadFile(paths[i]); err != nil || string(b) != r.Instructions {
			t.Fatalf("launch %d read %d bytes, %v; want the whole text", i, len(b), err)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(s.opts.RolesDir, "*.tmp")); len(left) != 0 {
		t.Fatalf("temp files left: %v", left)
	}
}

// TestRoleBypassRefused: the server refuses a role that would turn off
// every permission check, though config already does.
func TestRoleBypassRefused(t *testing.T) {
	s := &Server{opts: Options{RolesDir: t.TempDir()}}
	if _, err := s.roleArgs(Role{Name: "yolo", PermissionMode: "bypassPermissions"}, ""); !errors.Is(err, ErrRoleMode) {
		t.Fatalf("err %v, want ErrRoleMode", err)
	}
}

// TestAgentLaunchesAtOnce: launches sent together with no branch each get
// their own agent/<n>, though their git commands overlap.
func TestAgentLaunchesAtOnce(t *testing.T) {
	repo := t.TempDir() // before the server, so its shells have ended when the folder goes
	gittest.Repo(t, repo)
	s := agentServer(t)
	c := connect(t, s, 100, 30)
	first := attachIn(t, c, repo)
	const n = 4
	for range n {
		c.Send(agentNew(first, "Claude Code", ""))
	}
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(l.Windows) == n+1 })
	got := map[string]bool{}
	for _, w := range l.Windows[1:] {
		got[w.Panes[0].Branch] = true
	}
	for i := 1; i <= n; i++ {
		if b := "agent/" + strconv.Itoa(i); !got[b] {
			t.Fatalf("branches %v, want agent/1 to agent/%d", got, n)
		}
	}
}

// TestAgentLaunchStoppedServer: a server that stops while a launch makes
// its worktree takes the worktree and branch back before Close returns.
// The launch starts on the model goroutine, which then stops at once, so
// the launch finishes its git work with no model goroutine to open the
// agent.
func TestAgentLaunchStoppedServer(t *testing.T) {
	repo := t.TempDir() // before the server, so its shells have ended when the folder goes
	gittest.Repo(t, repo)
	s := agentServer(t)
	c := connect(t, s, 100, 30)
	first := attachIn(t, c, repo)
	closed := make(chan struct{})
	s.call(func(st *state) {
		var cl *client
		for k := range st.clients {
			cl = k
		}
		st.agentNew(cl, st.sessions["work"], agentNew(first, "Claude Code", ""))
		go func() {
			s.Close()
			close(closed)
		}()
		<-s.quit
	})
	<-closed
	if got := gittest.Run(t, repo, "branch", "--list", "agent/*"); got != "" {
		t.Fatalf("branch %q left by a launch the server stopped", got)
	}
	if got := gittest.Run(t, repo, "worktree", "list", "--porcelain"); strings.Contains(got, ".worktrees") {
		t.Fatalf("worktree left by a launch the server stopped:\n%s", got)
	}
}

// TestAgentFromSubfolder: an agent started from a folder below the top of
// a repository starts in the same folder of its worktree.
func TestAgentFromSubfolder(t *testing.T) {
	repo := t.TempDir() // before the server, so its shells have ended when the folder goes
	gittest.Repo(t, repo)
	sub := filepath.Join(repo, "cmd")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gittest.Run(t, repo, "add", "cmd")
	gittest.Run(t, repo, "commit", "-q", "-m", "cmd")
	s := agentServer(t)
	c := connect(t, s, 100, 30)
	first := attachIn(t, c, sub)
	c.Send(agentNew(first, "Claude Code", ""))
	p, _ := agentWindow(t, c, 2)
	board := nextAgents(t, c, func(l []api.AgentInfo) bool { return len(l) == 1 && l[0].Branch != "" })
	if want := filepath.Join(realPath(t, p.Worktree), "cmd"); realPath(t, agentFolder(t, board, p.ID)) != want {
		t.Fatalf("agent in %s, want %s", agentFolder(t, board, p.ID), want)
	}
}

// TestAgentBranchExists: a branch the user names that is already there is
// refused with its own code, and the branch is left as it was.
func TestAgentBranchExists(t *testing.T) {
	repo := t.TempDir() // before the server, so its shells have ended when the folder goes
	gittest.Repo(t, repo)
	gittest.Run(t, repo, "branch", "fix-login")
	s := agentServer(t)
	c := connect(t, s, 100, 30)
	first := attachIn(t, c, repo)
	c.Send(agentNew(first, "Claude Code", "fix-login"))
	if e := nextError(t, c); e.Code != api.CodeBranchExists {
		t.Fatalf("got %+v, want branch-exists", e)
	}
	if got := gittest.Run(t, repo, "branch", "--list", "fix-login"); got == "" {
		t.Fatal("the existing branch is gone")
	}
}
