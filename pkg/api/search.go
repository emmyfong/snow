package api

import (
	"encoding/json"
	"fmt"
)

// Search limits.
const (
	// MaxQueryBytes bounds a SearchRequest's query.
	MaxQueryBytes = 256
	// MaxMatches bounds the matches in one SearchResult.
	MaxMatches = 5000
)

// SearchRequest asks the server to find Query in a pane's history, the
// numbered lines that ScrollbackRequest reads. The query is plain text, not
// a pattern. It matches regardless of case unless it has an upper-case
// letter. The server answers with a SearchResult, or an Error when the pane
// is gone.
type SearchRequest struct {
	Pane  int    `json:"pane"`
	Query string `json:"query"`
}

// SearchResult answers a SearchRequest. Matches are in line order and, within
// a line, left to right; they do not overlap. Truncated is set when more than
// MaxMatches were found. Query repeats the request's, so a client can drop a
// result for a search it has replaced. Lines are numbered as when the server
// answered; see Scrollback for how they shift.
type SearchResult struct {
	Pane      int     `json:"pane"`
	Query     string  `json:"query"`
	Matches   []Match `json:"matches"`
	Truncated bool    `json:"truncated,omitempty"`
}

// Match is one place a query was found: the history line, the cell column
// where the match starts, and its width in cells.
type Match struct {
	Line int `json:"line"`
	Col  int `json:"col"`
	Len  int `json:"len"`
}

// Type implements Message.
func (*SearchRequest) Type() MsgType { return TypeSearchRequest }
func (*SearchRequest) message()      {}

// Type implements Message.
func (*SearchResult) Type() MsgType { return TypeSearchResult }
func (*SearchResult) message()      {}

// MarshalJSON writes Matches as [] when it is empty.
func (r SearchResult) MarshalJSON() ([]byte, error) {
	type wire SearchResult
	if r.Matches == nil {
		r.Matches = []Match{}
	}
	return json.Marshal(wire(r))
}

// Validate checks that r names a pane and a query of 1 to MaxQueryBytes
// bytes. Servers call it before searching.
func (r *SearchRequest) Validate() error {
	switch {
	case r.Pane <= 0:
		return fmt.Errorf("%w: no pane", ErrInvalidHistory)
	case r.Query == "" || len(r.Query) > MaxQueryBytes:
		return fmt.Errorf("%w: query is empty or over %d bytes", ErrInvalidHistory, MaxQueryBytes)
	}
	return nil
}
