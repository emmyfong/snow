package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/emmyfong/snow/internal/worktree"
	"github.com/emmyfong/snow/pkg/api"
)

// closeAgent closes the agent's pane cmd names, as run does, then removes
// the agent's worktree once its program has ended. The branch stays. A
// worktree with work in it stays too, and c hears why.
func (st *state) closeAgent(c *client, sess *session, cmd *api.Command) {
	_, p := sess.findPane(cmd.Target)
	st.run(c, sess, cmd)
	if p == nil || p.worktree == "" || p.term == nil || !p.closed.Load() {
		return
	}
	distro, _ := st.srv.distroOf(p.profile)
	repo := worktree.Repo{Path: p.worktree, Distro: distro}
	path, done := p.worktree, p.term.Done()
	srv := st.srv
	srv.closing.Add(1)
	go func() {
		defer srv.closing.Done()
		// git cannot remove a folder that the agent's program still holds.
		<-done
		ctx, cancel := context.WithTimeout(context.Background(), gitWait)
		defer cancel()
		err := repo.Remove(ctx, path)
		if err == nil {
			return
		}
		srv.post(func(st *state) {
			if !c.dropped {
				st.enqueue(c, &api.Error{Code: api.CodeNotice, Message: keptWorktree(path, err)})
			}
		})
	}()
}

// keptWorktree says why the worktree at path was not removed.
func keptWorktree(path string, err error) string {
	switch {
	case errors.Is(err, worktree.ErrDirty):
		return fmt.Sprintf("Kept the worktree %s: it has uncommitted changes.", path)
	case errors.Is(err, worktree.ErrUnreachableCommits):
		return fmt.Sprintf("Kept the worktree %s: it has commits no branch holds.", path)
	}
	return fmt.Sprintf("Kept the worktree %s: %v", path, err)
}
