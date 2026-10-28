package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/emmyfong/snow/pkg/api"
)

// toolDef describes one tool to the client, as tools/list lists it.
type toolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// noArgs is the input schema of a tool that takes no arguments.
var noArgs = json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)

const listPanesDescription = `List the terminal panes of your Snow session. ` +
	`Each one has an ID, the window that holds it, its program, its folder, and its size. ` +
	`The other tools name a pane by its ID. Panes in other Snow sessions are not visible.`

// toolList returns the tools, in the order an agent should try them. A
// session outside Snow has none, which tools/list reports as [].
func (s *server) toolList() []toolDef {
	if s.panes == nil {
		return []toolDef{}
	}
	return []toolDef{{
		Name:        "list_panes",
		Description: listPanesDescription,
		InputSchema: noArgs,
	}}
}

// listPanes answers list_panes.
func (s *server) listPanes(ctx context.Context) (string, error) {
	panes, err := s.panes.List(ctx)
	if err != nil {
		return "", err
	}
	return formatPanes(panes), nil
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
