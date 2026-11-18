package api

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// PaneViewer is the read-only file viewer. Its state is ViewerState.
const PaneViewer = "viewer"

// ViewerActRefresh is the viewer action that reads the file again.
const ViewerActRefresh = "refresh"

// Bounds on a viewer state. The reader cuts a file past these and says so.
const (
	MaxViewerLines   = 2000
	MaxViewerLineLen = 500
)

// ViewerState is the state of a viewer pane.
type ViewerState struct {
	Folder  string `json:"folder"`
	Path    string `json:"path,omitempty"` // from Folder
	Loading bool   `json:"loading,omitempty"`
	Binary  bool   `json:"binary,omitempty"`
	// Truncated says the file is larger than the reader shows.
	Truncated bool `json:"truncated,omitempty"`
	// Limit says what the reader left out, in words, or "".
	Limit string       `json:"limit,omitempty"`
	Lines []ViewerLine `json:"lines,omitempty"`
	// Reveal is the one-based line to show first, or 0.
	Reveal int `json:"reveal,omitempty"`
	// Attribution names the change evidence that still matches the
	// file, such as "edit, turn 3". It is empty without evidence.
	Attribution string `json:"attribution,omitempty"`
	// Stale says the conversation recorded a change to this file, but the
	// text no longer matches, so the viewer shows no marks.
	Stale bool `json:"stale,omitempty"`
	// Error is the last failure, such as a file that could not be read.
	Error string `json:"error,omitempty"`
}

// ViewerLine is one row of the viewer. A removed line has Number 0 and
// Mark "-", and it sits where the change removed it.
type ViewerLine struct {
	Number int    `json:"number"`
	Text   string `json:"text,omitempty"`
	// Mark is "+" for a line a recorded change added, "-" for a line it
	// removed, and "" otherwise.
	Mark string `json:"mark,omitempty"`
}

// Kind implements SideState.
func (*ViewerState) Kind() string { return PaneViewer }

// Validate checks that st can be drawn.
func (st *ViewerState) Validate() error {
	if st.Path != "" && !validRel(st.Path) {
		return fmt.Errorf("%w: viewer path %q", ErrInvalidSidePane, st.Path)
	}
	if len(st.Lines) > MaxViewerLines {
		return fmt.Errorf("%w: %d viewer lines, at most %d", ErrInvalidSidePane, len(st.Lines), MaxViewerLines)
	}
	for _, l := range st.Lines {
		if l.Number < 0 {
			return fmt.Errorf("%w: viewer line number %d", ErrInvalidSidePane, l.Number)
		}
		if l.Mark != "" && l.Mark != "+" && l.Mark != "-" {
			return fmt.Errorf("%w: viewer line mark %q", ErrInvalidSidePane, l.Mark)
		}
		if utf8.RuneCountInString(l.Text) > MaxViewerLineLen {
			return fmt.Errorf("%w: viewer line of %d characters", ErrInvalidSidePane, utf8.RuneCountInString(l.Text))
		}
	}
	if st.Reveal < 0 {
		return fmt.Errorf("%w: viewer reveal %d", ErrInvalidSidePane, st.Reveal)
	}
	if len(st.Limit) > 200 {
		return fmt.Errorf("%w: viewer limit of %d characters", ErrInvalidSidePane, len(st.Limit))
	}
	if len(st.Attribution) > 200 {
		return fmt.Errorf("%w: viewer attribution of %d characters", ErrInvalidSidePane, len(st.Attribution))
	}
	if strings.ContainsRune(st.Path, '\\') {
		return fmt.Errorf("%w: viewer path %q", ErrInvalidSidePane, st.Path)
	}
	return nil
}
