package chat

import (
	"github.com/emmyfong/snow/internal/client/textdiff"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// toolDiff is the change of one tool call, worked out once: the line diff
// costs time in the size of the file, and a frame must not redo it.
type toolDiff struct {
	lines          []ui.DiffLine
	added, removed int
}

// diffCache keeps the toolDiff of each tool call until a draw no longer
// shows the call. Api.ChatSnapshot.Apply swaps in a new ChatTool when a
// call changes, so the pointer says whether the diff is still current.
type diffCache struct {
	byTool map[*api.ChatTool]*toolDiff
	seen   map[*api.ChatTool]bool
	// box is the diff of the approval box, which is not a transcript call.
	boxKey  *api.ChatApproval
	boxDiff *toolDiff
}

func newToolDiff(t *api.ChatTool) *toolDiff {
	lines := diffOf(t)
	if lines == nil {
		return nil
	}
	a, r := textdiff.Counts(lines)
	return &toolDiff{lines: uiDiff(lines), added: a, removed: r}
}

// of returns the diff of call t, or nil when it changes nothing.
func (c *diffCache) of(t *api.ChatTool) *toolDiff {
	if c.byTool == nil {
		c.byTool = map[*api.ChatTool]*toolDiff{}
	}
	if c.seen != nil {
		c.seen[t] = true
	}
	d, ok := c.byTool[t]
	if !ok {
		d = newToolDiff(t)
		c.byTool[t] = d
	}
	return d
}

// ofApproval returns the diff of the request p, which t describes.
func (c *diffCache) ofApproval(p *api.ChatApproval, t *api.ChatTool) *toolDiff {
	if c.boxKey != p {
		c.boxKey, c.boxDiff = p, newToolDiff(t)
	}
	return c.boxDiff
}

// startDraw and endDraw drop the diffs of calls a draw did not show, so
// the cache never holds more than the transcript.
func (c *diffCache) startDraw() { c.seen = map[*api.ChatTool]bool{} }

func (c *diffCache) endDraw() {
	for t := range c.byTool {
		if !c.seen[t] {
			delete(c.byTool, t)
		}
	}
	c.seen = nil
}
