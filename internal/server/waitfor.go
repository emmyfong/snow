package server

import (
	"context"
	"fmt"
	"time"

	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// wait_for serves an agent in a pane. The owner goroutine only finds the
// pane. The wait runs in its own goroutine, outside the owner (ADR 0007),
// and the connection's read loop stays free to see the caller go away. The
// screen is read only when the pane's version changes, so a quiet pane costs
// one number every waitPoll.

const (
	// waitPoll is how often a wait checks the pane's version.
	waitPoll = 100 * time.Millisecond
	// waitDefaultTimeout is how long a request waits when it gives no limit.
	waitDefaultTimeout = api.WaitDefaultMS * time.Millisecond
	// waitMaxTimeout caps any request, including a quiet period.
	waitMaxTimeout = api.WaitMaxMS * time.Millisecond
	// waitMaxMS is waitMaxTimeout in milliseconds, and the most a request
	// may ask for.
	waitMaxMS = api.WaitMaxMS
)

// waitFor starts a wait on one terminal pane of the caller's session.
func (s *Server) waitFor(c *client, r *api.WaitForRequest) {
	if err := r.Validate(); err != nil {
		s.refuse(c, api.CodeBadCommand, err)
		return
	}
	var (
		tp    *term.Pane
		ended bool
		e     *api.Error
	)
	if !s.call(func(st *state) {
		_, p, err := st.toolPane(r.Caller, r.Pane)
		if err != nil {
			e = err
			return
		}
		if p.id == r.Caller {
			e = &api.Error{Code: api.CodeOwnPane, Message: fmt.Sprintf("pane %d is your own pane; wait_for does not wait on the pane the agent runs in", p.id)}
			return
		}
		tp, ended = p.term, p.ended
	}) {
		return // the server stopped, so no reply can reach the client
	}
	if e != nil {
		s.reply(c, e)
		return
	}
	if ended {
		s.reply(c, exitedWait(tp, r.Pane))
		return
	}
	go s.awaitPane(c, tp, r)
}

// awaitPane runs one wait and answers it. A canceled wait, because the
// caller closed the connection, answers nothing.
func (s *Server) awaitPane(c *client, tp *term.Pane, r *api.WaitForRequest) {
	if s.opts.afterWait != nil {
		defer s.opts.afterWait()
	}
	deadline := time.Now().Add(waitLimit(r.TimeoutMS))
	var res *api.WaitFor
	if r.Text != "" {
		res = awaitText(c.tools, tp, r, deadline)
	} else {
		res = awaitQuiet(c.tools, tp, r, deadline)
	}
	if res == nil {
		return
	}
	s.reply(c, res)
}

// awaitText waits until the pane prints text not seen when the wait began,
// or for IncludeExisting, any text. It returns nil when ctx ends.
func awaitText(ctx context.Context, tp *term.Pane, r *api.WaitForRequest, deadline time.Time) *api.WaitFor {
	// New output can land on a line that already held the prompt, so a line
	// number does not say whether the text is new. Count the matching lines
	// by their text when the wait begins: text that appears more often than
	// it did then is new. Line numbers would fail when the history's dropped
	// count overcounts (term.History), which moves every line.
	seen := map[string]int{}
	if !r.IncludeExisting {
		for _, line := range tp.SearchLines(r.Text) {
			seen[line]++
		}
	}
	check := func() (string, bool) {
		lines := tp.SearchLines(r.Text)
		counts := make(map[string]int, len(lines))
		for i := len(lines) - 1; i >= 0; i-- { // newest first
			counts[lines[i]]++
			if counts[lines[i]] > seen[lines[i]] {
				return lines[i], true
			}
		}
		return "", false
	}
	if r.IncludeExisting {
		if line, ok := check(); ok {
			return foundWait(r.Pane, line)
		}
	}
	version := tp.Version()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tp.Done():
			// The program may have printed the text and exited between
			// two polls.
			if line, ok := check(); ok {
				return foundWait(r.Pane, line)
			}
			return exitedWait(tp, r.Pane)
		case <-time.After(waitPoll):
		}
		if tp.Version() == version {
			if time.Now().After(deadline) {
				return timeoutWait(r.Pane)
			}
			continue
		}
		version = tp.Version()
		if line, ok := check(); ok {
			return foundWait(r.Pane, line)
		}
		if time.Now().After(deadline) {
			return timeoutWait(r.Pane)
		}
	}
}

// awaitQuiet waits until the pane's version does not change for QuietMS.
func awaitQuiet(ctx context.Context, tp *term.Pane, r *api.WaitForRequest, deadline time.Time) *api.WaitFor {
	quiet := quietPeriod(r.QuietMS)
	version := tp.Version()
	last := time.Now()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tp.Done():
			return exitedWait(tp, r.Pane)
		case <-time.After(waitPoll):
		}
		if v := tp.Version(); v != version {
			version = v
			last = time.Now()
		}
		if time.Since(last) >= quiet {
			return &api.WaitFor{Pane: r.Pane, Result: api.WaitQuiet}
		}
		if time.Now().After(deadline) {
			return timeoutWait(r.Pane)
		}
	}
}

// waitLimit returns how long a request may wait, with the default and the
// cap applied. The comparison is in milliseconds, so a huge request cannot
// overflow the duration.
func waitLimit(ms int) time.Duration {
	switch {
	case ms <= 0:
		return waitDefaultTimeout
	case ms > waitMaxMS:
		return waitMaxTimeout
	}
	return time.Duration(ms) * time.Millisecond
}

// quietPeriod returns how long a pane must stay unchanged, with the cap
// applied.
func quietPeriod(ms int) time.Duration {
	if ms <= 0 {
		return 0
	}
	if ms > waitMaxMS {
		return waitMaxTimeout
	}
	return time.Duration(ms) * time.Millisecond
}

func foundWait(pane int, line string) *api.WaitFor {
	return &api.WaitFor{Pane: pane, Result: api.WaitFound, Line: line}
}

func timeoutWait(pane int) *api.WaitFor {
	return &api.WaitFor{Pane: pane, Result: api.WaitTimeout}
}

// exitedWait is the result for a pane whose program exited, with its status
// when Snow knows it.
func exitedWait(tp *term.Pane, pane int) *api.WaitFor {
	res := &api.WaitFor{Pane: pane, Result: api.WaitExited}
	if code, ok := tp.ExitStatus(); ok {
		res.Exit = &code
	}
	return res
}
