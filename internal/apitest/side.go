package apitest

import (
	"errors"
	"time"

	"github.com/emmyfong/snow/pkg/api"
)

// Side returns the newest state of side pane pane, or nil before the first.
func (c *Client) Side(pane int) *api.SidePaneState { return c.sides[pane] }

// WaitSide reads messages until ok reports true for side pane pane's
// newest state, and returns it.
func (c *Client) WaitSide(pane int, ok func(*api.SidePaneState) bool) *api.SidePaneState {
	c.t.Helper()
	deadline := time.Now().Add(Wait)
	for time.Now().Before(deadline) {
		if s := c.sides[pane]; s != nil && ok(s) {
			return s
		}
		if _, err := c.Next(100 * time.Millisecond); err != nil && !errors.Is(err, ErrTimeout) {
			c.t.Fatalf("waiting for side pane %d: %v", pane, err)
		}
	}
	c.t.Fatalf("side pane %d never got there: %+v", pane, c.sides[pane])
	return nil
}

// Kinds returns the last pane kind list, or nil before the first.
func (c *Client) Kinds() []api.PaneKindInfo {
	if c.kinds == nil {
		return nil
	}
	return c.kinds.Kinds
}
