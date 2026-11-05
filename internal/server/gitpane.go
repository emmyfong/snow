package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/emmyfong/snow/internal/git"
	"github.com/emmyfong/snow/pkg/api"
)

// ErrGitBusy is returned for a git pane action while earlier ones still
// wait for git.
var ErrGitBusy = errors.New("git is still busy with earlier actions")

// gitJobs bounds the actions a git pane holds while git runs.
const gitJobs = 8

// gitKind is the git pane: the changes in its folder, their diffs, and
// stage and commit actions. Git runs on the pane's own goroutine.
func gitKind() sideKind {
	return sideKind{Name: api.PaneGit, Title: "Git", Open: openGit}
}

// gitPane is the model goroutine's half of a git pane. Its worker runs git
// and hands each new state over through Apply, so state needs no lock.
type gitPane struct {
	state api.GitState // owned by the model goroutine
	// pending counts the jobs taken and not yet answered, so a state the
	// worker read before a job never shows the pane as done with it.
	pending int
	agent   func(hash string) string
	jobs    chan gitJob
	cancel  context.CancelFunc
	done    chan struct{}
}

// gitJob is one action for the worker.
type gitJob struct {
	action string
	args   map[string]string
}

func openGit(ctx sideContext) (sidePane, error) {
	runCtx, cancel := context.WithCancel(context.Background())
	p := &gitPane{
		state:  api.GitState{Folder: ctx.Folder, Loading: true},
		jobs:   make(chan gitJob, gitJobs),
		cancel: cancel,
		done:   make(chan struct{}),
		agent:  ctx.Agent,
	}
	w := &gitWorker{
		repo:  git.Repo{Path: ctx.Folder, Distro: ctx.Distro},
		cur:   api.GitState{Folder: ctx.Folder},
		apply: func(st api.GitState, done bool) { ctx.Apply(func() { p.take(st, done) }) },
	}
	go func() {
		defer close(p.done)
		w.run(runCtx, p.jobs)
	}()
	return p, nil
}

// take makes st the pane's state, on the model goroutine. done is set
// when st answers a job. The pane shows loading while any job waits.
func (p *gitPane) take(st api.GitState, done bool) {
	if done && p.pending > 0 {
		p.pending--
	}
	st.Loading = p.pending > 0
	p.state = st
}

// State implements sidePane.
//
// The history's agent marks are read now, so a mark that arrives after
// git's history still shows.
func (p *gitPane) State() api.SideState {
	st := p.state
	if p.agent != nil && len(st.History) > 0 {
		st.History = append([]api.GitCommit(nil), st.History...)
		for i := range st.History {
			st.History[i].Agent = p.agent(st.History[i].Hash)
		}
	}
	return &st
}

// Act implements sidePane. It checks the action and hands it to the
// worker; the outcome comes back as a new state.
func (p *gitPane) Act(action string, args map[string]string) error {
	if err := checkGitAction(action, args); err != nil {
		return err
	}
	select {
	case p.jobs <- gitJob{action: action, args: args}:
	default:
		return ErrGitBusy
	}
	p.pending++
	p.state.Loading = true
	return nil
}

// Close implements sidePane. It stops git and waits for the worker.
func (p *gitPane) Close() error {
	p.cancel()
	<-p.done
	return nil
}

// checkGitAction checks action and its arguments before git runs.
func checkGitAction(action string, args map[string]string) error {
	switch action {
	case api.GitActRefresh, api.GitActStageAll, api.GitActCloseDiff:
		return nil
	case api.GitActStage, api.GitActUnstage, api.GitActDiff:
		if args[api.ArgGitPath] == "" {
			return fmt.Errorf("git %s: no file named", action)
		}
		return nil
	case api.GitActCommit:
		msg := args[api.ArgGitMessage]
		switch {
		case strings.TrimSpace(msg) == "":
			return git.ErrEmptyMessage
		case len(msg) > api.MaxCommitMessage:
			return fmt.Errorf("the commit message is longer than %d bytes", api.MaxCommitMessage)
		}
		return nil
	}
	return fmt.Errorf("the git pane has no action %q", action)
}
