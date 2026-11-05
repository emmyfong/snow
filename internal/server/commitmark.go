package server

import (
	"strconv"

	"github.com/emmyfong/snow/pkg/api"
)

// maxCommitMarks bounds the commits the server remembers an agent made.
// The oldest mark goes first.
const maxCommitMarks = 1000

// commitMarks are the commits Snow saw agents make, by hash, with the name
// of the agent shown in the git pane's history. Owned by the model
// goroutine; kept in memory only.
type commitMarks struct {
	by    map[string]string
	order []string
}

// add marks hash as made by agent, dropping the oldest mark when full.
func (c *commitMarks) add(hash, agent string) {
	if c.by == nil {
		c.by = map[string]string{}
	}
	if _, ok := c.by[hash]; !ok {
		c.order = append(c.order, hash)
		if len(c.order) > maxCommitMarks {
			delete(c.by, c.order[0])
			c.order = c.order[1:]
		}
	}
	c.by[hash] = agent
}

// agent returns the agent that made hash, or "" when Snow did not see one.
func (c *commitMarks) agent(hash string) string { return c.by[hash] }

// observeCommit records what snow hook read of git around one of p's tool
// calls that runs git commit. HEAD before the call waits on the pane; the
// read after it marks HEAD as the agent's only when HEAD's parent is that
// HEAD: the call made exactly one new commit on top. An amend, a reset,
// or a commit someone else made between the two reads marks nothing.
func (st *state) observeCommit(w *window, p *pane, c *api.AgentCommit) {
	if c.Before != "" {
		p.commitBefore = c.Before
		return
	}
	before := p.commitBefore
	p.commitBefore = ""
	if before == "" || c.After == "" || c.Parent != before {
		return
	}
	name := "window " + strconv.Itoa(w.index)
	if w.name != "" {
		name += " " + w.name
	}
	st.marks.add(c.After, name)
	for _, sess := range st.sessions {
		for _, w := range sess.windows {
			for id, sp := range w.panes {
				if sp.sideKind == api.PaneGit {
					st.sideChanged(id)
				}
			}
		}
	}
}
