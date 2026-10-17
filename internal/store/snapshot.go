// Package store saves sessions to disk so they survive a reboot or a server
// crash. Each session is one JSON file, written atomically. A Saver batches
// rapid changes into one write.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Version is the snapshot format version that Save writes. Load rejects
// files with a newer or missing version.
const Version = 1

// MaxScrollback is how many scrollback lines Save keeps per pane: the last
// ones, the newest at the end.
const MaxScrollback = 2000

// Snapshot is everything needed to rebuild one session.
type Snapshot struct {
	Version int       `json:"version"`
	Name    string    `json:"name"`
	Saved   time.Time `json:"saved"` // set by Save
	Active  int       `json:"active"`
	Windows []Window  `json:"windows"`
}

// Window is one window of a session.
type Window struct {
	Index int    `json:"index"` // the window's number, as the status bar shows it
	Name  string `json:"name"`
	// Layout is the window's layout tree, encoded by internal/layout. The
	// store does not read it.
	Layout  json.RawMessage `json:"layout,omitempty"`
	Focused int             `json:"focused,omitempty"` // ID of the focused pane
	Panes   []Pane          `json:"panes"`
}

// Pane is one pane of a window. ID matches the pane's leaf in the layout.
type Pane struct {
	ID      int    `json:"id"`
	Profile string `json:"profile"` // the shell profile's name
	Folder  string `json:"folder,omitempty"`
	// Scrollback holds the pane's last lines, oldest first, with styles as
	// ANSI escape sequences: its history, the scrollback then the screen.
	Scrollback []string `json:"scrollback,omitempty"`
}

// DecodeError reports a snapshot file that cannot be read back: bad JSON,
// a missing or newer version, or no session name.
type DecodeError struct {
	Path string // empty when the data did not come from a file
	Err  error
}

func (e *DecodeError) Error() string {
	if e.Path == "" {
		return "decode snapshot: " + e.Err.Error()
	}
	return fmt.Sprintf("decode snapshot %s: %v", e.Path, e.Err)
}

// Unwrap returns the underlying error.
func (e *DecodeError) Unwrap() error { return e.Err }

// encode stamps the format version and keeps the last MaxScrollback lines of
// each pane. It does not change the caller's slices.
func encode(s Snapshot) ([]byte, error) {
	s.Version = Version
	windows := make([]Window, len(s.Windows))
	for i, w := range s.Windows {
		panes := make([]Pane, len(w.Panes))
		for j, p := range w.Panes {
			if n := len(p.Scrollback); n > MaxScrollback {
				p.Scrollback = p.Scrollback[n-MaxScrollback:]
			}
			panes[j] = p
		}
		w.Panes = panes
		windows[i] = w
	}
	s.Windows = windows
	b, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("encode snapshot %q: %w", s.Name, err)
	}
	return b, nil
}

func decode(b []byte) (Snapshot, error) {
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return Snapshot{}, &DecodeError{Err: err}
	}
	switch {
	case s.Version == 0:
		return Snapshot{}, &DecodeError{Err: errors.New("no format version")}
	case s.Version > Version:
		return Snapshot{}, &DecodeError{Err: fmt.Errorf("format version %d is newer than %d", s.Version, Version)}
	case s.Name == "":
		return Snapshot{}, &DecodeError{Err: errors.New("no session name")}
	}
	return s, nil
}
