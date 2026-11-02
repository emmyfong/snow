package mcp

import (
	"fmt"
	"strings"

	"github.com/emmyfong/snow/pkg/api"
)

// The tool results are plain text, written the way an agent reads them: the
// facts first, then what to do next.

// formatRead writes a pane copy the way an agent reads it: a header, the
// screen, and then the scrollback above it.
func formatRead(r *api.ReadPane) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Session %q, pane %d: %s, %s, %dx%d",
		r.Session, r.Pane, oneLine(r.Profile), folder(r.Folder), r.Cols, r.Rows)
	if r.Ended {
		b.WriteString(", the program ended")
	}
	if r.CursorVisible {
		fmt.Fprintf(&b, ", cursor at column %d, row %d", r.CursorX+1, r.CursorY+1)
	} else {
		b.WriteString(", no cursor")
	}
	fmt.Fprintf(&b, ".\nScreen, top row first:\n%s", strings.Join(r.Screen, "\n"))
	if len(r.Scrollback) > 0 {
		fmt.Fprintf(&b, "\n\nScrollback, the %d newest lines above the screen, oldest first:\n%s",
			len(r.Scrollback), strings.Join(r.Scrollback, "\n"))
	}
	if r.Omitted > 0 {
		fmt.Fprintf(&b, "\n\n%d older lines were left out.", r.Omitted)
	}
	return b.String()
}

// formatWait writes why a wait ended the way an agent reads it.
func formatWait(r *api.WaitFor) string {
	switch r.Result {
	case api.WaitFound:
		return fmt.Sprintf("Pane %d printed the text:\n%s", r.Pane, r.Line)
	case api.WaitQuiet:
		return fmt.Sprintf("Pane %d stopped changing.", r.Pane)
	case api.WaitExited:
		if r.Exit != nil {
			return fmt.Sprintf("Pane %d's program exited with status %d. Read the pane to see its last output.", r.Pane, *r.Exit)
		}
		return fmt.Sprintf("Pane %d's program exited. Read the pane to see its last output.", r.Pane)
	case api.WaitTimeout:
		return fmt.Sprintf("Pane %d reached the timeout first. It may still run; read the pane to see what it shows.", r.Pane)
	}
	return fmt.Sprintf("Pane %d: the wait ended with %q.", r.Pane, r.Result)
}

// formatSend writes what happened to the input the way an agent reads it.
func formatSend(r *api.SendKeys) string {
	if r.Dropped > 0 {
		return fmt.Sprintf("Typed into pane %d, but the program did not read %d bytes and dropped them. Read the pane to see what it shows.", r.Pane, r.Dropped)
	}
	return fmt.Sprintf("Typed into pane %d. Read the pane to see the result.", r.Pane)
}

// formatPanes writes the panes the way an agent reads them: one line per
// pane, with the fields a tool call needs.
func formatPanes(p *api.Panes) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Session %q, your pane is %d.\n", p.Session, p.Caller)
	fmt.Fprintf(&b, "%d terminal pane(s):\n", len(p.Panes))
	for _, e := range p.Panes {
		fmt.Fprintf(&b, "pane %d: window %d %q, %s, %s, %dx%d",
			e.ID, e.Window, oneLine(e.WindowName), oneLine(e.Profile), folder(e.Folder), e.Rect.W, e.Rect.H)
		if e.Focused {
			b.WriteString(", focused")
		}
		if !e.Shown {
			b.WriteString(", in a window that is not shown")
		}
		if e.Agent {
			b.WriteString(", agent")
			if e.AgentState != "" {
				b.WriteString(": " + e.AgentState)
			}
		}
		if e.Ended {
			b.WriteString(", ended")
		}
		if e.ID == p.Caller {
			b.WriteString(", this is you")
		}
		b.WriteByte('\n')
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// folder names a pane's folder, or says it has none.
func folder(f string) string {
	if f == "" {
		return "no folder"
	}
	return oneLine(f)
}

// oneLine keeps a server string on one output line, so the next field is
// never mistaken for part of it.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
