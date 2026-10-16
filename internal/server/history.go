package server

import (
	"encoding/json"
	"fmt"

	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// findPane returns the terminal of pane id in c's session, or nil. The
// model goroutine only looks the pane up; callers read it outside, so a
// large read never stalls the other clients.
func (s *Server) findPane(c *client, id int) *term.Pane {
	var tp *term.Pane
	s.call(func(st *state) {
		if p := st.paneOf(c, id); p != nil {
			tp = p.term
		}
	})
	return tp
}

// reply queues m for c from c's reader goroutine. It waits for the model
// goroutine, so a client's replies keep the order of its requests.
func (s *Server) reply(c *client, m api.Message) {
	s.call(func(st *state) { st.enqueue(c, m) })
}

// refuse answers a history request that cannot be served.
func (s *Server) refuse(c *client, code string, err error) {
	s.reply(c, &api.Error{Code: code, Message: err.Error()})
}

// scrollback answers a ScrollbackRequest with a page of the pane's history.
func (s *Server) scrollback(c *client, r *api.ScrollbackRequest) {
	if err := r.Validate(); err != nil {
		s.refuse(c, api.CodeBadCommand, err)
		return
	}
	tp := s.findPane(c, r.Pane)
	if tp == nil {
		s.refuse(c, api.CodeCommandFailed, fmt.Errorf("no pane %d", r.Pane))
		return
	}
	lines, total, dropped := tp.History(r.From, r.Count)
	lines = fitPage(lines, maxPageBytes)
	s.reply(c, &api.Scrollback{Pane: r.Pane, From: r.From, Total: total, Dropped: dropped, Lines: lines})
}

// maxPageBytes bounds the encoded lines of one Scrollback, half a frame, so
// the reply always fits with room for its other fields.
const maxPageBytes = api.MaxFrameSize / 2

// fitPage returns the longest start of lines whose JSON encoding stays
// within limit bytes, and at least one line, so paging always moves on.
// Styles make a line long: JSON writes each escape byte as six.
func fitPage(lines []string, limit int) []string {
	size := 0
	for i, l := range lines {
		enc, err := json.Marshal(l)
		if err != nil {
			return lines[:i]
		}
		size += len(enc) + 1
		if size > limit && i > 0 {
			return lines[:i]
		}
	}
	return lines
}

// search answers a SearchRequest with every match in the pane's history.
func (s *Server) search(c *client, r *api.SearchRequest) {
	if err := r.Validate(); err != nil {
		s.refuse(c, api.CodeBadCommand, err)
		return
	}
	tp := s.findPane(c, r.Pane)
	if tp == nil {
		s.refuse(c, api.CodeCommandFailed, fmt.Errorf("no pane %d", r.Pane))
		return
	}
	found, truncated, dropped := tp.Search(r.Query)
	res := &api.SearchResult{Pane: r.Pane, Query: r.Query, Truncated: truncated, Dropped: dropped, Matches: make([]api.Match, len(found))}
	for i, m := range found {
		res.Matches[i] = api.Match{Line: m.Line, Col: m.Col, Len: m.Len}
	}
	s.reply(c, res)
}
