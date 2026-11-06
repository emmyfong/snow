package apitest

import (
	"errors"
	"time"

	"github.com/emmyfong/snow/pkg/api"
)

// WaitLayout returns the last or next layout that matches. It retains layouts
// received while another wait consumed a connection response or pane output.
func (c *Client) WaitLayout(match func(*api.Layout) bool) *api.Layout {
	c.t.Helper()
	deadline := time.Now().Add(Wait)
	for time.Now().Before(deadline) {
		if c.layout != nil && match(c.layout) {
			return c.layout
		}
		if _, err := c.Next(100 * time.Millisecond); err != nil && !errors.Is(err, ErrTimeout) {
			c.t.Fatalf("waiting for layout: %v", err)
		}
	}
	c.t.Fatal("layout never reached expected state")
	return nil
}
