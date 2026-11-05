package server

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/git"
	"github.com/emmyfong/snow/internal/gittest"
	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// openGitPane attaches c to a new session in dir and splits its first pane
// with a git pane. It returns the git pane's ID.
func openGitPane(t *testing.T, c *apitest.Client, dir string) int {
	t.Helper()
	c.Send(&api.Attach{Session: "git", Create: true, Dir: dir})
	first := shownPanes(nextLayout(t, c, func(*api.Layout) bool { return true }))[0].ID
	c.Send(&api.Command{Op: api.OpSplit, Target: first, Args: map[string]string{api.ArgDir: api.DirRight, api.ArgKind: api.PaneGit}})
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	for _, p := range shownPanes(l) {
		if p.Kind == api.PaneGit {
			return p.ID
		}
	}
	t.Fatal("no git pane")
	return 0
}

// waitGit reads until the git pane's state satisfies ok, and returns it.
func waitGit(t *testing.T, c *apitest.Client, pane int, ok func(*api.GitState) bool) api.GitState {
	t.Helper()
	var st api.GitState
	c.WaitSide(pane, func(s *api.SidePaneState) bool {
		var got api.GitState
		if err := api.DecodeSideState(s, &got); err != nil {
			t.Fatal(err)
		}
		st = got
		return ok(&got)
	})
	return st
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestGitPaneStageCommit: the pane lists the folder's changes, shows a
// diff, stages one file, and commits it with the user's identity; the
// other change stays.
func TestGitPaneStageCommit(t *testing.T) {
	repo := t.TempDir() // before the server, so its shell has ended when the folder goes
	gittest.Repo(t, repo)
	writeFile(t, repo, "a.txt", "changed\n")
	writeFile(t, repo, "b.txt", "new\n")
	s := New(testOptions())
	t.Cleanup(s.Close)
	c := connect(t, s, 120, 30)
	pane := openGitPane(t, c, repo)

	st := waitGit(t, c, pane, func(st *api.GitState) bool { return len(st.Changes) == 2 })
	if st.Branch != "main" || st.Folder != repo {
		t.Fatalf("state %+v, want main in %s", st, repo)
	}

	c.Send(&api.SidePaneAction{Pane: pane, Action: api.GitActDiff, Args: map[string]string{api.ArgGitPath: "a.txt"}})
	st = waitGit(t, c, pane, func(st *api.GitState) bool { return st.Diff != nil && !st.Loading })
	if !hasDiffLine(st.Diff, api.GitLineAdd, "changed") || !hasDiffLine(st.Diff, api.GitLineRemove, "a") {
		t.Fatalf("diff %+v", st.Diff)
	}
	c.Send(&api.SidePaneAction{Pane: pane, Action: api.GitActCloseDiff})
	waitGit(t, c, pane, func(st *api.GitState) bool { return st.Diff == nil && !st.Loading })

	c.Send(&api.SidePaneAction{Pane: pane, Action: api.GitActStage, Args: map[string]string{api.ArgGitPath: "b.txt"}})
	waitGit(t, c, pane, func(st *api.GitState) bool {
		return !st.Loading && len(st.Changes) == 2 && st.Changes[1].Path == "b.txt" && st.Changes[1].Staged()
	})

	c.Send(&api.SidePaneAction{Pane: pane, Action: api.GitActCommit, Args: map[string]string{api.ArgGitMessage: "add b"}})
	st = waitGit(t, c, pane, func(st *api.GitState) bool { return strings.HasPrefix(st.Notice, "Committed ") })
	if len(st.Changes) != 1 || st.Changes[0].Path != "a.txt" || st.Error != "" {
		t.Fatalf("after commit %+v, want only a.txt changed", st)
	}
	if who := gittest.Run(t, repo, "log", "-1", "--format=%an <%ae>|%B"); who != "Test <test@example.com>|add b" {
		t.Errorf("commit %q, want the user's identity and the message alone", who)
	}
}

// TestGitPaneCommitFails: a failed commit shows git's own words in Error,
// and the pane goes on working.
func TestGitPaneCommitFails(t *testing.T) {
	repo := t.TempDir()
	gittest.Repo(t, repo)
	s := New(testOptions())
	t.Cleanup(s.Close)
	c := connect(t, s, 120, 30)
	pane := openGitPane(t, c, repo)
	waitGit(t, c, pane, func(st *api.GitState) bool { return st.Branch == "main" })
	c.Send(&api.SidePaneAction{Pane: pane, Action: api.GitActCommit, Args: map[string]string{api.ArgGitMessage: "nothing"}})
	st := waitGit(t, c, pane, func(st *api.GitState) bool { return st.Error != "" })
	if !strings.Contains(st.Error, "git commit") {
		t.Errorf("error %q, want git's commit failure", st.Error)
	}
}

// TestGitPaneNotRepo: a folder in no repository says so.
func TestGitPaneNotRepo(t *testing.T) {
	dir := t.TempDir()
	gittest.Isolate(t)
	s := New(testOptions())
	t.Cleanup(s.Close)
	c := connect(t, s, 120, 30)
	pane := openGitPane(t, c, dir)
	if st := waitGit(t, c, pane, func(st *api.GitState) bool { return st.NotRepo }); len(st.Changes) != 0 || st.Error != "" {
		t.Errorf("state %+v", st)
	}
}

// TestGitPaneActionRefused: an action that cannot run is refused before
// git starts.
func TestGitPaneActionRefused(t *testing.T) {
	repo := t.TempDir()
	gittest.Repo(t, repo)
	s := New(testOptions())
	t.Cleanup(s.Close)
	c := connect(t, s, 120, 30)
	pane := openGitPane(t, c, repo)
	for _, a := range []*api.SidePaneAction{
		{Pane: pane, Action: api.GitActCommit, Args: map[string]string{api.ArgGitMessage: " "}},
		{Pane: pane, Action: api.GitActStage},
		{Pane: pane, Action: "discard", Args: map[string]string{api.ArgGitPath: "a.txt"}},
	} {
		c.Send(a)
		if _, e := apitest.Reply[*api.SidePaneState](c); e == nil || e.Code != api.CodeCommandFailed {
			t.Errorf("%s answered %v, want a command failure", a.Action, e)
		}
	}
}

func hasDiffLine(d *api.GitDiff, kind, text string) bool {
	for _, l := range d.Lines {
		if l.Kind == kind && l.Text == text {
			return true
		}
	}
	return false
}

// TestCheckGitAction: each action's arguments are checked.
func TestCheckGitAction(t *testing.T) {
	long := strings.Repeat("x", api.MaxCommitMessage+1)
	for _, tt := range []struct {
		action string
		args   map[string]string
		ok     bool
	}{
		{api.GitActRefresh, nil, true},
		{api.GitActStageAll, nil, true},
		{api.GitActStage, map[string]string{api.ArgGitPath: "a"}, true},
		{api.GitActStage, nil, false},
		{api.GitActDiff, map[string]string{api.ArgGitPath: "a", api.ArgGitStaged: "true"}, true},
		{api.GitActCommit, map[string]string{api.ArgGitMessage: "m"}, true},
		{api.GitActCommit, map[string]string{api.ArgGitMessage: long}, false},
		{"discard", nil, false},
	} {
		if err := checkGitAction(tt.action, tt.args); (err == nil) != tt.ok {
			t.Errorf("%s %v: %v, want ok %v", tt.action, tt.args, err, tt.ok)
		}
	}
	if err := checkGitAction(api.GitActCommit, map[string]string{api.ArgGitMessage: "\n"}); !errors.Is(err, git.ErrEmptyMessage) {
		t.Errorf("blank message: %v", err)
	}
}

// TestFitGitState: a state too big for a frame loses diff rows first, then
// changes, and says so.
func TestFitGitState(t *testing.T) {
	big := strings.Repeat("\x01", 2000) // six bytes each as JSON
	st := api.GitState{Diff: &api.GitDiff{Path: "a"}}
	for range 200 {
		st.Diff.Lines = append(st.Diff.Lines, api.GitDiffLine{Kind: api.GitLineAdd, Text: big})
		st.Changes = append(st.Changes, api.GitChange{Path: big, Work: "M"})
	}
	fitGitState(&st)
	if size(&st) > gitStateBudget {
		t.Fatalf("still %d bytes", size(&st))
	}
	if !st.Diff.Cut || len(st.Diff.Lines) != 0 || st.Omitted == 0 || len(st.Changes)+st.Omitted != 200 {
		t.Errorf("cut %v, %d lines, %d changes, %d omitted", st.Diff.Cut, len(st.Diff.Lines), len(st.Changes), st.Omitted)
	}
}

// TestSideKindsOverride: a test's kind takes the place of Snow's own kind
// of its name, and others are added.
func TestSideKindsOverride(t *testing.T) {
	open := func(sideContext) (sidePane, error) { return nil, nil }
	got := sideKinds([]sideKind{{Name: api.PaneGit, Title: "Fake", Open: open}, {Name: "note", Open: open}})
	if len(got) != 2 || got[0].Title != "Fake" || got[1].Name != "note" {
		t.Errorf("kinds %+v", got)
	}
}

// TestGitPaneWorktreeList: the pane lists the repository's worktrees from
// real git, the main checkout first, with each branch.
func TestGitPaneWorktreeList(t *testing.T) {
	repo := t.TempDir()
	gittest.Repo(t, repo)
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo, "worktree", "add", "-q", "-b", "agent/1", linked)
	s := New(testOptions())
	t.Cleanup(s.Close)
	c := connect(t, s, 120, 30)
	pane := openGitPane(t, c, repo)
	st := waitGit(t, c, pane, func(st *api.GitState) bool { return len(st.Worktrees) == 2 })
	if !st.Worktrees[0].Main || st.Worktrees[0].Branch != "main" || st.Worktrees[1].Branch != "agent/1" {
		t.Fatalf("worktrees %+v", st.Worktrees)
	}
	if !samePathForTest(st.Worktrees[1].Path, linked) {
		t.Errorf("linked worktree at %s, want %s", st.Worktrees[1].Path, linked)
	}
}

// samePathForTest compares a path git listed with one the test made, which
// may differ by symlinks, such as macOS's /var and /private/var.
func samePathForTest(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && strings.EqualFold(ra, rb)
}

// TestNewPaneFromSidePane: a pane opened from a git pane runs with the
// profile of the pane the git pane opened from, so a terminal for a WSL
// worktree starts in that distribution. The git pane is still no agent.
func TestNewPaneFromSidePane(t *testing.T) {
	dir := t.TempDir()
	gittest.Isolate(t)
	var asked []string
	opts := testOptions()
	shell := opts.Spec
	opts.Spec = func(profile, d string) (term.Spec, string, error) {
		asked = append(asked, profile)
		spec, name, err := shell(profile, d)
		if profile != "" {
			name = profile
		}
		return spec, name, err
	}
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 120, 30)
	c.Send(&api.Attach{Session: "git", Create: true, Dir: dir})
	first := shownPanes(nextLayout(t, c, func(*api.Layout) bool { return true }))[0].ID
	c.Send(&api.Command{Op: api.OpSplit, Target: first, Args: map[string]string{api.ArgDir: api.DirRight, api.ArgProfile: "work"}})
	work := 0
	for _, p := range shownPanes(nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })) {
		if p.ID != first {
			work = p.ID
		}
	}
	c.Send(&api.Command{Op: api.OpSplit, Target: work, Args: map[string]string{api.ArgDir: api.DirDown, api.ArgKind: api.PaneGit}})
	nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 3 })
	c.Send(&api.Command{Op: api.OpNewWindow, Args: map[string]string{api.ArgKind: api.PaneTerminal, api.ArgFolder: dir}})
	nextLayout(t, c, func(l *api.Layout) bool { return len(l.Windows) == 2 })
	var last string
	s.call(func(*state) {})
	if len(asked) > 0 {
		last = asked[len(asked)-1]
	}
	if last != "work" {
		t.Errorf("the new window's terminal asked for profile %q, want work (asked %q)", last, asked)
	}
	for _, a := range c.WaitAgents(func([]api.AgentInfo) bool { return true }) {
		if a.Kind == api.PaneGit {
			t.Errorf("the git pane is listed as an agent: %+v", a)
		}
	}
}
