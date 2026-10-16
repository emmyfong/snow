package api

import (
	"encoding/json"
	"errors"
	"fmt"
)

// MaxScrollbackLines bounds how many lines one ScrollbackRequest may ask
// for. A server may return fewer, for example to keep a page of heavily
// styled lines inside one frame; the client asks again for the rest.
const MaxScrollbackLines = 256

// ErrInvalidHistory is returned by the Validate methods of the history
// requests (ScrollbackRequest and SearchRequest).
var ErrInvalidHistory = errors.New("api: invalid history request")

// ScrollbackRequest asks for Count lines of a pane's history, starting at
// line From. The history is the lines that scrolled off the pane, oldest
// first, followed by the rows now on its screen; line 0 is the oldest. The
// server answers with a Scrollback, or an Error when the pane is gone.
type ScrollbackRequest struct {
	Pane  int `json:"pane"`
	From  int `json:"from"`
	Count int `json:"count"`
}

// Scrollback answers a ScrollbackRequest. Lines starts at line From and is
// styled with ANSI escape sequences, like LineUpdate. It is shorter than
// requested, or empty, when the request reaches past the history. Total is
// the history's length when the server answered; it grows as the pane prints
// and stops growing when the server's scrollback limit is reached, after
// which the oldest lines drop and every line number shifts down.
//
// Dropped counts the lines that have dropped off the top since the pane
// started. When it grows by d between two replies, every line number has
// shifted down by d, so a client moves what it holds before it stores the
// new lines. It may overcount, never undercount; when it grows by more than
// the client held, nothing held is still valid.
type Scrollback struct {
	Pane    int      `json:"pane"`
	From    int      `json:"from"`
	Total   int      `json:"total"`
	Dropped int      `json:"dropped,omitempty"`
	Lines   []string `json:"lines"`
}

// Type implements Message.
func (*ScrollbackRequest) Type() MsgType { return TypeScrollbackRequest }
func (*ScrollbackRequest) message()      {}

// Type implements Message.
func (*Scrollback) Type() MsgType { return TypeScrollback }
func (*Scrollback) message()      {}

// MarshalJSON writes Lines as [] when it is empty.
func (s Scrollback) MarshalJSON() ([]byte, error) {
	type wire Scrollback
	if s.Lines == nil {
		s.Lines = []string{}
	}
	return json.Marshal(wire(s))
}

// Validate checks that r names a pane and a range of 1 to MaxScrollbackLines
// lines from a line of 0 or more. Servers call it before answering.
func (r *ScrollbackRequest) Validate() error {
	switch {
	case r.Pane <= 0:
		return fmt.Errorf("%w: no pane", ErrInvalidHistory)
	case r.From < 0:
		return fmt.Errorf("%w: from is below 0", ErrInvalidHistory)
	case r.Count < 1 || r.Count > MaxScrollbackLines:
		return fmt.Errorf("%w: count is outside 1 to %d", ErrInvalidHistory, MaxScrollbackLines)
	}
	return nil
}
