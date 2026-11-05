package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/emmyfong/snow/internal/git"
	"github.com/emmyfong/snow/internal/worktree"
	"github.com/emmyfong/snow/pkg/api"
)

// Git pane timing. The folder changes under the pane as agents work, so
// the worker reads it again on a timer. A commit runs the repository's
// hooks, which can take a while.
var (
	gitPoll       = 3 * time.Second
	gitReadWait   = 30 * time.Second
	gitCommitWait = 5 * time.Minute
)

// gitStateBudget is the most a git pane's state may take as JSON, so it
// fits in one frame with room for the envelope.
const gitStateBudget = api.MaxFrameSize * 3 / 4

// gitWorker runs a git pane's git commands on its own goroutine. cur is
// the state it last built; only the worker touches it. Each state it hands
// to apply is built anew, never changed afterwards.
type gitWorker struct {
	repo   git.Repo
	cur    api.GitState
	diff   *git.Change // the change whose diff is open, or nil
	staged bool
	apply  func(api.GitState)
}

// run reads the folder, then does each job, and reads the folder again on
// each tick, until ctx ends.
func (w *gitWorker) run(ctx context.Context, jobs <-chan gitJob) {
	w.refresh(ctx, true)
	tick := time.NewTicker(gitPoll)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case j := <-jobs:
			w.do(ctx, j)
		case <-tick.C:
			w.refresh(ctx, false)
		}
	}
}

// do runs one job, then reads the folder again and hands the state over,
// with the job's outcome in Notice or Error.
func (w *gitWorker) do(ctx context.Context, j gitJob) {
	notice, err := w.action(ctx, j)
	if ctx.Err() != nil {
		return
	}
	next := w.read(ctx)
	next.Notice = notice
	if err != nil {
		next.Error = err.Error()
	}
	w.publish(next, true)
}

// action runs j's git command and returns what to tell the user.
func (w *gitWorker) action(ctx context.Context, j gitJob) (string, error) {
	path := j.args[api.ArgGitPath]
	switch j.action {
	case api.GitActStage:
		return "", w.withWait(ctx, gitReadWait, func(c context.Context) error { return w.repo.Stage(c, path) })
	case api.GitActUnstage:
		paths := []string{path}
		if ch, ok := w.change(path); ok && ch.Orig != "" {
			paths = append(paths, ch.Orig)
		}
		return "", w.withWait(ctx, gitReadWait, func(c context.Context) error { return w.repo.Unstage(c, paths...) })
	case api.GitActStageAll:
		return "", w.withWait(ctx, gitReadWait, func(c context.Context) error {
			paths := make([]string, len(w.cur.Changes))
			for i, ch := range w.cur.Changes {
				paths[i] = ch.Path
			}
			return w.repo.Stage(c, paths...)
		})
	case api.GitActCommit:
		var hash string
		err := w.withWait(ctx, gitCommitWait, func(c context.Context) error {
			var err error
			hash, err = w.repo.Commit(c, j.args[api.ArgGitMessage])
			return err
		})
		if err != nil {
			return "", err
		}
		w.diff = nil
		return "Committed " + hash, nil
	case api.GitActDiff:
		ch, ok := w.change(path)
		if !ok {
			return "", fmt.Errorf("%s has no change", path)
		}
		w.diff, w.staged = &ch, j.args[api.ArgGitStaged] == "true"
		return "", nil
	case api.GitActCloseDiff:
		w.diff = nil
	}
	return "", nil
}

// withWait runs f with ctx bounded by wait.
func (w *gitWorker) withWait(ctx context.Context, wait time.Duration, f func(context.Context) error) error {
	c, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	return f(c)
}

// change returns the change to path in the last state, as internal/git
// has it.
func (w *gitWorker) change(path string) (git.Change, bool) {
	for _, c := range w.cur.Changes {
		if c.Path == path {
			return git.Change{Path: c.Path, Orig: c.Orig, Index: c.Index, Work: c.Work, Conflict: c.Conflict}, true
		}
	}
	return git.Change{}, false
}

// refresh reads the folder and hands the state over if it changed. The
// last action's Notice and Error stay until the next action.
func (w *gitWorker) refresh(ctx context.Context, first bool) {
	next := w.read(ctx)
	if ctx.Err() != nil {
		return
	}
	next.Notice, next.Error = w.cur.Notice, w.cur.Error
	w.publish(next, first)
}

// read builds a state from the folder's status and the open diff.
func (w *gitWorker) read(ctx context.Context) api.GitState {
	next := api.GitState{Folder: w.cur.Folder}
	c, cancel := context.WithTimeout(ctx, gitReadWait)
	defer cancel()
	st, err := w.repo.Status(c)
	switch {
	case errors.Is(err, git.ErrNotRepo):
		next.NotRepo = true
		return next
	case err != nil:
		next.Error = err.Error()
		return next
	}
	next.Branch, next.Detached, next.Initial = st.Branch, st.Detached, st.Initial
	next.Upstream, next.Ahead, next.Behind = st.Upstream, st.Ahead, st.Behind
	next.Omitted = st.Omitted
	for _, ch := range st.Changes {
		next.Changes = append(next.Changes, api.GitChange{Path: ch.Path, Orig: ch.Orig, Index: ch.Index, Work: ch.Work, Conflict: ch.Conflict})
	}
	if w.diff != nil {
		next.Diff = w.readDiff(c, st.Changes)
	}
	// A git that cannot list worktrees leaves the section empty; the
	// changes still show.
	if wts, err := worktree.Repo(w.repo).List(c); err == nil {
		for _, wt := range wts {
			if wt.Bare {
				continue
			}
			next.Worktrees = append(next.Worktrees, api.GitWorktree{Path: wt.Path, Branch: wt.Branch, Main: wt.Main, Detached: wt.Detached, Locked: wt.Locked})
		}
	}
	return next
}

// readDiff reads the open diff again. A file whose change is gone, such as
// one just committed, shows an empty diff.
func (w *gitWorker) readDiff(ctx context.Context, changes []git.Change) *api.GitDiff {
	d := &api.GitDiff{Path: w.diff.Path, Staged: w.staged}
	for _, ch := range changes {
		if ch.Path == w.diff.Path {
			*w.diff = ch
			fd, err := w.repo.Diff(ctx, ch, w.staged)
			if err != nil {
				return d
			}
			d.Binary, d.Cut = fd.Binary, fd.Cut
			for _, l := range fd.Lines {
				d.Lines = append(d.Lines, api.GitDiffLine{Kind: gitLineKinds[l.Kind], Text: l.Text})
			}
			return d
		}
	}
	return d
}

var gitLineKinds = map[git.LineKind]string{
	git.LineKeep: api.GitLineKeep, git.LineAdd: api.GitLineAdd,
	git.LineRemove: api.GitLineRemove, git.LineHunk: api.GitLineHunk,
}

// publish hands next over when it differs from the last state, or always
// when force is set: an action's answer clears the pane's Loading mark.
func (w *gitWorker) publish(next api.GitState, force bool) {
	fitGitState(&next)
	if !force && reflect.DeepEqual(next, w.cur) {
		return
	}
	w.cur = next
	w.apply(next)
}

// fitGitState cuts st until its JSON fits gitStateBudget: the diff first,
// then the list of changes. Escaped control characters can make a short
// line long, so the size is measured, not guessed.
func fitGitState(st *api.GitState) {
	for size(st) > gitStateBudget {
		switch {
		case st.Diff != nil && len(st.Diff.Lines) > 0:
			st.Diff.Lines = st.Diff.Lines[:len(st.Diff.Lines)/2]
			st.Diff.Cut = true
		case len(st.Worktrees) > 0:
			st.Worktrees = nil
		case len(st.Changes) > 0:
			half := len(st.Changes) / 2
			st.Omitted += len(st.Changes) - half
			st.Changes = st.Changes[:half]
		default:
			return
		}
	}
}

func size(st *api.GitState) int {
	b, err := json.Marshal(st)
	if err != nil {
		return 0
	}
	return len(b)
}
