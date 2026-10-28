package mcp

import (
	"context"
	"encoding/json"
	"errors"
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

const readPaneDescription = `Read a terminal pane of your Snow session as plain text. ` +
	`Returns the screen rows, the newest lines above the screen, the cursor, and the size. ` +
	`Lines carry no styles, so colors and cursor moves are gone. ` +
	`Call list_panes first to find the pane IDs.`

// readPaneArgs is the input schema of read_pane.
var readPaneArgs = json.RawMessage(`{
  "type": "object",
  "properties": {
    "pane": {"type": "integer", "description": "The pane ID that list_panes shows."},
    "scrollback": {"type": "integer", "description": "How many lines above the screen to return. The default is 50 and the most is 500."}
  },
  "required": ["pane"],
  "additionalProperties": false
}`)

const sendKeysDescription = `Type into a terminal pane of your Snow session: paste text and press keys. ` +
	`The text goes in first, then the keys in order, as if the user typed them. ` +
	`A key is a named key (enter, tab, space, escape, up, f5) or one character, ` +
	`with shift, alt, ctrl, or meta before it: "ctrl+c", "shift+tab", "alt+enter". ` +
	`This changes the pane, so the user approves each call. ` +
	`It refuses your own pane, another agent's pane, a chat pane, and an ended pane. ` +
	`Call read_pane afterwards to see the result.`

// sendKeysArgs is the input schema of send_keys.
var sendKeysArgs = json.RawMessage(`{
  "type": "object",
  "properties": {
    "pane": {"type": "integer", "description": "The pane ID that list_panes shows."},
    "text": {"type": "string", "description": "Text to paste into the pane, such as a command line. Omit it to press keys only."},
    "keys": {"type": "array", "items": {"type": "string"}, "description": "Keys to press after the text, such as [\"enter\"] or [\"ctrl+c\"]."}
  },
  "required": ["pane"],
  "additionalProperties": false
}`)

// toolList returns the tools, in the order an agent should try them. A
// session outside Snow has none, which tools/list reports as [].
func (s *server) toolList() []toolDef {
	if s.panes == nil {
		return []toolDef{}
	}
	return []toolDef{
		{
			Name:        "list_panes",
			Description: listPanesDescription,
			InputSchema: noArgs,
		},
		{
			Name:        "read_pane",
			Description: readPaneDescription,
			InputSchema: readPaneArgs,
		},
		{
			Name:        "send_keys",
			Description: sendKeysDescription,
			InputSchema: sendKeysArgs,
		},
	}
}

// listPanes answers list_panes.
func (s *server) listPanes(ctx context.Context) (string, error) {
	panes, err := s.panes.List(ctx)
	if err != nil {
		return "", err
	}
	return formatPanes(panes), nil
}

// readPane answers read_pane.
func (s *server) readPane(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Pane       int `json:"pane"`
		Scrollback int `json:"scrollback"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &in); err != nil {
			return "", fmt.Errorf("read_pane: %w", err)
		}
	}
	if in.Pane < 1 {
		return "", errors.New("read_pane needs a pane ID; call list_panes first")
	}
	res, err := s.panes.Read(ctx, in.Pane, in.Scrollback)
	if err != nil {
		return "", err
	}
	return formatRead(res), nil
}

// sendKeys answers send_keys.
func (s *server) sendKeys(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Pane int      `json:"pane"`
		Text string   `json:"text"`
		Keys []string `json:"keys"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &in); err != nil {
			return "", fmt.Errorf("send_keys: %w", err)
		}
	}
	if in.Pane < 1 {
		return "", errors.New("send_keys needs a pane ID; call list_panes first")
	}
	if in.Text == "" && len(in.Keys) == 0 {
		return "", errors.New("send_keys needs text, keys, or both")
	}
	keys := make([]api.Key, 0, len(in.Keys))
	for _, name := range in.Keys {
		k, err := api.ParseKey(name)
		if err != nil {
			return "", fmt.Errorf("send_keys: %w", err)
		}
		keys = append(keys, k)
	}
	res, err := s.panes.Send(ctx, in.Pane, in.Text, keys)
	if err != nil {
		return "", err
	}
	return formatSend(res), nil
}

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
