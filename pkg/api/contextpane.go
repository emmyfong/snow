package api

import (
	"fmt"
	"time"
)

// PaneContext is the context pane: the context occupancy a runtime reports
// and the conversation's observable context sources. Its state is
// ContextState.
const PaneContext = "context"

// ContextActRefresh re-reads the conversation's records.
const ContextActRefresh = "refresh"

// Bounds on a context pane state. A conversation with more sources or
// repeated reads than these shows the biggest and says how many it left
// out.
const (
	MaxContextSources = 64
	MaxContextReads   = 32
	MaxContextNotes   = 8
	MaxContextName    = 256
)

// ContextState is the state of a context pane. It follows one conversation,
// so it rebuilds from that conversation's records after a restore.
type ContextState struct {
	Folder string `json:"folder,omitempty"`
	// Conversation is the conversation the pane follows.
	Conversation string `json:"conversation,omitempty"`
	Title        string `json:"title,omitempty"`
	// Runtime names the external runtime that runs the conversation, or ""
	// for Snow's direct-API loop.
	Runtime   string           `json:"runtime,omitempty"`
	Model     string           `json:"model,omitempty"`
	Occupancy ContextOccupancy `json:"occupancy"`
	// Sources are the files the conversation's tool calls read or changed,
	// most read first.
	Sources []ContextSource `json:"sources,omitempty"`
	// Reads are the files read more than once, most repeated first. Each
	// one compares the tool result text of its reads, not a saved file
	// version.
	Reads []ContextRead `json:"reads,omitempty"`
	// Notes explain a limit of the data, such as a breakdown no runtime
	// reports.
	Notes   []string `json:"notes,omitempty"`
	Loading bool     `json:"loading,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// ContextOccupancy is how full the model's context window was. It is known
// only when the source reports it; Snow never estimates the hidden prompt.
type ContextOccupancy struct {
	Known  bool `json:"known"`
	Tokens int  `json:"tokens,omitempty"`
	// Window is the model's context window in tokens; 0 means unknown.
	Window int `json:"window,omitempty"`
	// Source names what Tokens measures, such as the last prompt of a
	// direct-API reply.
	Source string `json:"source,omitempty"`
	// Reason says why the occupancy is unavailable, when it is.
	Reason string `json:"reason,omitempty"`
}

// ContextSource is a file the conversation's tool calls touched.
type ContextSource struct {
	Path      string `json:"path"`
	Reads     int    `json:"reads,omitempty"`
	Changed   bool   `json:"changed,omitempty"`
	FirstTurn int    `json:"firstTurn,omitempty"`
	LastTurn  int    `json:"lastTurn,omitempty"`
	// TokensKnown is always false: neither tool call reports how many
	// tokens its content added.
	TokensKnown bool `json:"tokensKnown,omitempty"`
}

// ContextRead is one file and range read more than once.
type ContextRead struct {
	Path  string `json:"path"`
	Range string `json:"range,omitempty"`
	Reads int    `json:"reads"`
	// Same counts the reads whose result text matched the read before
	// them; Changed counts those that did not. A file change can justify a
	// second read, so this is evidence, not waste.
	Same    int `json:"same,omitempty"`
	Changed int `json:"changed,omitempty"`
	// Result is "unchanged", "changed", or "mixed", from Same and Changed.
	Result string `json:"result"`
	// Evidence names what was compared: the tool result text.
	Evidence  string    `json:"evidence,omitempty"`
	FirstTurn int       `json:"firstTurn,omitempty"`
	LastTurn  int       `json:"lastTurn,omitempty"`
	FirstAt   time.Time `json:"firstAt,omitempty"`
	LastAt    time.Time `json:"lastAt,omitempty"`
}

// Kind implements SideState.
func (*ContextState) Kind() string { return PaneContext }

// Validate checks that st can be drawn.
func (st *ContextState) Validate() error {
	for name, s := range map[string]string{
		"title": st.Title, "model": st.Model, "runtime": st.Runtime,
		"conversation": st.Conversation, "folder": st.Folder,
	} {
		if len(s) > MaxContextName {
			return fmt.Errorf("%w: %s is %d bytes, at most %d", ErrInvalidSidePane, name, len(s), MaxContextName)
		}
	}
	if st.Occupancy.Tokens < 0 || st.Occupancy.Window < 0 {
		return fmt.Errorf("%w: occupancy %d over %d", ErrInvalidSidePane, st.Occupancy.Tokens, st.Occupancy.Window)
	}
	if st.Occupancy.Known && st.Occupancy.Window > 0 && st.Occupancy.Tokens > st.Occupancy.Window {
		return fmt.Errorf("%w: occupancy %d over a window of %d", ErrInvalidSidePane, st.Occupancy.Tokens, st.Occupancy.Window)
	}
	if len(st.Sources) > MaxContextSources {
		return fmt.Errorf("%w: %d sources, at most %d", ErrInvalidSidePane, len(st.Sources), MaxContextSources)
	}
	if len(st.Reads) > MaxContextReads {
		return fmt.Errorf("%w: %d repeated reads, at most %d", ErrInvalidSidePane, len(st.Reads), MaxContextReads)
	}
	if len(st.Notes) > MaxContextNotes {
		return fmt.Errorf("%w: %d notes, at most %d", ErrInvalidSidePane, len(st.Notes), MaxContextNotes)
	}
	for _, s := range st.Sources {
		if s.Path == "" || len(s.Path) > MaxContextName {
			return fmt.Errorf("%w: source path %q", ErrInvalidSidePane, s.Path)
		}
		if s.Reads < 0 {
			return fmt.Errorf("%w: source %q has %d reads", ErrInvalidSidePane, s.Path, s.Reads)
		}
	}
	for _, r := range st.Reads {
		if r.Path == "" || len(r.Path) > MaxContextName || len(r.Range) > MaxContextName {
			return fmt.Errorf("%w: repeated read %q %q", ErrInvalidSidePane, r.Path, r.Range)
		}
		if r.Reads < 2 || r.Same < 0 || r.Changed < 0 || r.Same+r.Changed != r.Reads-1 {
			return fmt.Errorf("%w: repeated read %q has %d reads, %d same, %d changed",
				ErrInvalidSidePane, r.Path, r.Reads, r.Same, r.Changed)
		}
		switch r.Result {
		case "unchanged", "changed", "mixed":
		default:
			return fmt.Errorf("%w: repeated read %q has result %q", ErrInvalidSidePane, r.Path, r.Result)
		}
	}
	return nil
}
