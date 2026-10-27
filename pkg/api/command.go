package api

import (
	"errors"
	"fmt"
	"strconv"
)

// Command operations. Pane operations take a pane ID as Target; window
// operations take a window index, which counts from 0 as in tmux. Each
// operation's arguments are listed with it.
const (
	OpSplit        = "split"         // dir; optional profile, folder, kind, agent
	OpClose        = "close"         // close pane Target
	OpResize       = "resize"        // dir, cells: move Target's edge on side dir
	OpZoom         = "zoom"          // toggle zoom on Target
	OpSwap         = "swap"          // dir: swap Target with its neighbor
	OpFocus        = "focus"         // optional dir: focus Target, or its neighbor
	OpEven         = "even"          // even out the row or column holding Target
	OpNewWindow    = "new-window"    // optional profile, folder, kind
	OpRenameWindow = "rename-window" // name
	OpSelectWindow = "select-window" // show window Target
	OpNextWindow   = "next-window"
	OpPrevWindow   = "prev-window"
	OpCloseWindow  = "close-window" // close window Target and its panes
)

// Command argument keys.
const (
	ArgDir     = "dir"
	ArgProfile = "profile" // a name from ProfileList; empty means the default
	ArgFolder  = "folder"  // empty means the target pane's folder
	ArgCells   = "cells"   // a positive whole number
	ArgName    = "name"
	ArgKind    = "kind" // PaneTerminal or PaneChat; empty means PaneTerminal
	// ArgAgent asks a split for an agent's terminal pane in place of a
	// profile: the server picks the agent's profile that fits the target
	// pane, for example the one inside the same WSL distribution. When it
	// finds none, it answers with a command-failed Error that says how to
	// install the agent. Only AgentClaude is known.
	ArgAgent = "agent"
)

// Agents for ArgAgent.
const (
	AgentClaude = "claude" // Claude Code
)

// Directions for ArgDir.
const (
	DirLeft  = "left"
	DirRight = "right"
	DirUp    = "up"
	DirDown  = "down"
)

// ErrInvalidCommand is returned by Command.Validate.
var ErrInvalidCommand = errors.New("api: invalid command")

// Validate checks that c names a known operation with the target and
// arguments it needs. Servers call it before acting on a Command.
func (c *Command) Validate() error {
	needTarget := func() error {
		if c.Target <= 0 {
			return fmt.Errorf("%w: %s needs a target", ErrInvalidCommand, c.Op)
		}
		return nil
	}
	needWindow := func() error {
		if c.Target < 0 {
			return fmt.Errorf("%w: %s needs a window index of 0 or more", ErrInvalidCommand, c.Op)
		}
		return nil
	}
	needDir := func(optional bool) error {
		d, ok := c.Args[ArgDir]
		if !ok && optional {
			return nil
		}
		switch d {
		case DirLeft, DirRight, DirUp, DirDown:
			return nil
		}
		return fmt.Errorf("%w: %s needs a dir of left, right, up, or down", ErrInvalidCommand, c.Op)
	}
	if k, ok := c.Args[ArgKind]; ok && k != PaneTerminal && k != PaneChat {
		return fmt.Errorf("%w: unknown pane kind %q", ErrInvalidCommand, k)
	}
	if a, ok := c.Args[ArgAgent]; ok {
		if a != AgentClaude || c.Op != OpSplit {
			return fmt.Errorf("%w: %s cannot take agent %q", ErrInvalidCommand, c.Op, a)
		}
		// The server picks the agent's profile, and an agent runs in a
		// terminal pane.
		if _, set := c.Args[ArgProfile]; set || c.Args[ArgKind] == PaneChat {
			return fmt.Errorf("%w: an agent takes no profile and no chat kind", ErrInvalidCommand)
		}
	}
	switch c.Op {
	case OpSplit, OpSwap:
		return errors.Join(needTarget(), needDir(false))
	case OpResize:
		if err := errors.Join(needTarget(), needDir(false)); err != nil {
			return err
		}
		if c.Cells() <= 0 {
			return fmt.Errorf("%w: resize needs a positive cells", ErrInvalidCommand)
		}
		return nil
	case OpFocus:
		return errors.Join(needTarget(), needDir(true))
	case OpClose, OpZoom, OpEven:
		return needTarget()
	case OpSelectWindow, OpCloseWindow:
		return needWindow()
	case OpRenameWindow:
		if c.Args[ArgName] == "" {
			return fmt.Errorf("%w: rename-window needs a name", ErrInvalidCommand)
		}
		return needWindow()
	case OpNewWindow, OpNextWindow, OpPrevWindow:
		return nil
	}
	return fmt.Errorf("%w: unknown op %q", ErrInvalidCommand, c.Op)
}

// Cells returns the cells argument, or 0 when it is missing or not a whole
// number.
func (c *Command) Cells() int {
	n, err := strconv.Atoi(c.Args[ArgCells])
	if err != nil {
		return 0
	}
	return n
}
