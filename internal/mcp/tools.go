package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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

const waitForDescription = `Wait until a terminal pane of your Snow session prints text, or stops changing. ` +
	`With text, waits for a line that holds the text, as plain text, ignoring case unless the text has an upper-case letter. ` +
	`Text already on the pane counts only with includeExisting. ` +
	`With quietMs, waits until the pane has produced no output for that many milliseconds. ` +
	`Give one of text and quietMs. Use it after send_keys instead of reading the pane in a loop. ` +
	`It refuses your own pane. ` +
	`The result says found with the matching line, quiet, exited with the program's exit status, or timed out. ` +
	`A timeout is a normal result. The default timeout is 30 seconds and the most is 10 minutes.`

// waitForArgs is the input schema of wait_for.
var waitForArgs = json.RawMessage(`{
  "type": "object",
  "properties": {
    "pane": {"type": "integer", "description": "The pane ID that list_panes shows."},
    "text": {"type": "string", "description": "Wait for a new line that holds this text. Give this or quietMs."},
    "quietMs": {"type": "integer", "description": "Wait until the pane has produced no output for this many milliseconds. Give this or text."},
    "timeoutMs": {"type": "integer", "description": "How long to wait at most. The default is 30000 and the most is 600000."},
    "includeExisting": {"type": "boolean", "description": "Also match text that was on the pane before the wait began. The default is false."}
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
		{
			Name:        "wait_for",
			Description: waitForDescription,
			InputSchema: waitForArgs,
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

// waitFor answers wait_for.
func (s *server) waitFor(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Pane            int    `json:"pane"`
		Text            string `json:"text"`
		QuietMS         int    `json:"quietMs"`
		TimeoutMS       int    `json:"timeoutMs"`
		IncludeExisting bool   `json:"includeExisting"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &in); err != nil {
			return "", fmt.Errorf("wait_for: %w", err)
		}
	}
	if in.Pane < 1 {
		return "", errors.New("wait_for needs a pane ID; call list_panes first")
	}
	if in.QuietMS < 0 || in.TimeoutMS < 0 {
		return "", errors.New("wait_for needs a quietMs and a timeoutMs of zero or more")
	}
	if (in.Text == "") == (in.QuietMS == 0) {
		return "", errors.New("wait_for needs exactly one of text and quietMs")
	}
	// Clamp before converting, so a huge request cannot overflow the
	// duration and reach Snow as a negative one.
	in.QuietMS = min(in.QuietMS, api.WaitMaxMS)
	in.TimeoutMS = min(in.TimeoutMS, api.WaitMaxMS)
	res, err := s.panes.Wait(ctx, in.Pane, in.Text,
		time.Duration(in.QuietMS)*time.Millisecond, time.Duration(in.TimeoutMS)*time.Millisecond, in.IncludeExisting)
	if err != nil {
		return "", err
	}
	return formatWait(res), nil
}
