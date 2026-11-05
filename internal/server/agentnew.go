package server

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/emmyfong/snow/internal/worktree"
	"github.com/emmyfong/snow/pkg/api"
)

var (
	// ErrNotAgent means agent-new named a profile that runs no agent.
	ErrNotAgent = errors.New("server: the profile runs no agent")
	// ErrFolderBusy means an agent already works in the folder, and no
	// worktree can be made there because it is not a git repository.
	ErrFolderBusy = errors.New("server: another agent works in this folder")
)

// gitWait bounds each git step of an agent launch; a hung WSL folder must
// not hold the launch forever.
const gitWait = 30 * time.Second

// maxAgentBranch is how far agentBranch counts before it gives up.
const maxAgentBranch = 999

// launched is a worktree made for an agent that has not started yet, and
// how the agent starts.
type launched struct {
	repo worktree.Repo
	wt   worktree.Worktree
	ok   bool // false for a folder that is not a git repository
	role *Role
	args []string // the role's Claude Code options
}

// agentNew starts the agent cmd asks for, from pane cmd.Target, in a new
// window. In a git repository it first makes the agent a worktree on a
// new branch; git and the agent's secrets are read on a goroutine, and the
// window opens when they answer. A launch that fails takes the worktree
// and its branch back, so it leaves no agent and no window behind.
func (st *state) agentNew(c *client, sess *session, cmd *api.Command) {
	_, from := sess.findPane(cmd.Target)
	if from == nil {
		st.enqueue(c, &api.Error{Code: api.CodeCommandFailed, Message: "no pane to start the agent from"})
		return
	}
	profile, folder := st.newPaneArgs(sess, cmd, from.id)
	launch := st.srv.launch
	if !launch.isAgent(profile) {
		st.enqueue(c, &api.Error{Code: api.CodeCommandFailed, Message: fmt.Sprintf("%v: %s", ErrNotAgent, profile)})
		return
	}
	var role *Role
	if name := cmd.Args[api.ArgRole]; name != "" {
		r, err := st.srv.role(name)
		if err != nil {
			st.enqueue(c, &api.Error{Code: api.CodeCommandFailed, Message: err.Error()})
			return
		}
		role = &r
	}
	distro, _ := st.srv.distroOf(profile)
	repo := worktree.Repo{Path: folder, Distro: distro}
	branch := cmd.Args[api.ArgBranch]
	srv := st.srv
	go func() {
		l, err := makeWorktree(repo, branch)
		l.role = role
		if err == nil && role != nil {
			l.args, err = srv.roleArgs(*role, distro)
		}
		if err == nil {
			err = launch.loadSecrets(launch.secrets(profile))
		}
		srv.post(func(st *state) {
			if err == nil {
				err = st.openAgent(c, sess, profile, folder, l)
			}
			if err != nil {
				discard(srv, l)
				if !c.dropped {
					st.enqueue(c, &api.Error{Code: api.CodeCommandFailed, Message: err.Error()})
				}
			}
		})
	}()
}

// makeWorktree makes repo a worktree on branch, or on agent/<n> when
// branch is empty. A folder that is not a git repository gets none, and
// no error.
func makeWorktree(repo worktree.Repo, branch string) (launched, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitWait)
	defer cancel()
	l := launched{repo: repo}
	var err error
	if branch != "" {
		l.wt, err = repo.Create(ctx, branch)
	} else {
		l.wt, err = agentBranch(ctx, repo)
	}
	switch {
	case errors.Is(err, worktree.ErrNotRepo):
		return l, nil
	case err != nil:
		return l, fmt.Errorf("make the agent's worktree: %w", err)
	}
	l.ok = true
	return l, nil
}

// agentBranch makes a worktree on agent/<n>, the lowest n whose branch and
// folder are free.
func agentBranch(ctx context.Context, repo worktree.Repo) (worktree.Worktree, error) {
	for n := 1; n <= maxAgentBranch; n++ {
		wt, err := repo.Create(ctx, "agent/"+strconv.Itoa(n))
		if errors.Is(err, worktree.ErrBranchExists) || errors.Is(err, worktree.ErrPathExists) {
			continue
		}
		return wt, err
	}
	return worktree.Worktree{}, fmt.Errorf("%w: agent/1 to agent/%d", worktree.ErrBranchExists, maxAgentBranch)
}

// openAgent opens the agent's window in sess. Without a worktree the agent
// works in folder itself, which no other agent may share.
func (st *state) openAgent(c *client, sess *session, profile, folder string, l launched) error {
	if c.dropped || st.sessions[sess.name] != sess || c.session != sess.name {
		return errors.New("the session ended before the agent started")
	}
	dir := folder
	if l.ok {
		dir = l.wt.Path
	} else if other := st.agentIn(folder); other != "" {
		return fmt.Errorf("%w: %s; it is not a git repository, so the agent cannot get its own worktree", ErrFolderBusy, other)
	}
	w, err := st.newWindowWith(sess, func(id, cols, rows int) (*pane, error) {
		return st.srv.startPaneArgs(id, cols, rows, profile, dir, "", l.args)
	})
	if err != nil {
		return err
	}
	p := w.panes[w.focused]
	if l.role != nil {
		info := l.role.info()
		p.role = &info
	}
	if l.ok {
		p.worktree, p.branch = l.wt.Path, l.wt.Branch
		w.name = l.wt.Branch
	}
	st.repaint(sess)
	st.changed(sess)
	return nil
}

// agentIn names the agent that works in folder, outside a worktree of its
// own, or returns "" when there is none.
func (st *state) agentIn(folder string) string {
	for _, sess := range st.sessions {
		for _, w := range sess.windows {
			for _, p := range w.panes {
				if p.worktree == "" && st.agentKind(p) != "" && p.folder() == folder {
					return fmt.Sprintf("pane %d in window %d of session %s", p.id, w.index, sess.name)
				}
			}
		}
	}
	return ""
}

// discard takes back the worktree of a launch that failed, on a goroutine.
func discard(srv *Server, l launched) {
	if !l.ok {
		return
	}
	srv.closing.Add(1)
	go func() {
		defer srv.closing.Done()
		ctx, cancel := context.WithTimeout(context.Background(), gitWait)
		defer cancel()
		if err := l.repo.Discard(ctx, l.wt); err != nil {
			srv.log.Warn("take back the agent's worktree", "worktree", l.wt.Path, "err", err)
		}
	}()
}
