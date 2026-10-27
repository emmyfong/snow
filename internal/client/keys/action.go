package keys

import (
	"strconv"
	"strings"
)

// Action is something the user can do with a key or the mouse. Name is the
// word the config file uses; Desc is what which-key and the menu show.
// Actions with the same Group are listed as one row in which-key.
type Action struct {
	Name  string
	Desc  string
	Group string // "" means the action is listed alone
}

// group describes a set of related actions in which-key.
type group struct {
	desc string
}

var groups = map[string]group{
	"select-window": {"Select window"},
	"focus":         {"Focus pane"},
	"resize":        {"Resize pane by 1"},
	"resize-5":      {"Resize pane by 5"},
}

var directions = []string{"left", "right", "up", "down"}

// actions lists every action in the order which-key shows them.
var actions = buildActions()

func buildActions() []Action {
	as := []Action{
		{Name: "split-right", Desc: "Split right"},
		{Name: "split-down", Desc: "Split down"},
		{Name: "new-window", Desc: "New window"},
		{Name: "new-claude", Desc: "Claude Code pane"},
		{Name: "new-chat", Desc: "New chat"},
		{Name: "model-picker", Desc: "Pick model"},
		{Name: "next-window", Desc: "Next window"},
		{Name: "prev-window", Desc: "Previous window"},
	}
	for i := range 10 {
		n := strconv.Itoa(i)
		as = append(as, Action{Name: "select-window-" + n, Desc: "Select window " + n, Group: "select-window"})
	}
	as = append(as,
		Action{Name: "rename-window", Desc: "Rename window"},
		Action{Name: "close-pane", Desc: "Close pane"},
		Action{Name: "close-window", Desc: "Close window"},
		Action{Name: "zoom", Desc: "Zoom"},
		Action{Name: "even-out", Desc: "Even out"},
	)
	for _, d := range directions {
		as = append(as, Action{Name: "focus-" + d, Desc: "Focus " + d, Group: "focus"})
	}
	for _, d := range directions {
		as = append(as, Action{Name: "resize-" + d, Desc: "Resize " + d + " by 1", Group: "resize"})
	}
	for _, d := range directions {
		as = append(as, Action{Name: "resize-" + d + "-5", Desc: "Resize " + d + " by 5", Group: "resize-5"})
	}
	return append(as,
		Action{Name: "copy-mode", Desc: "Copy mode"},
		Action{Name: "sessions", Desc: "Switch session"},
		Action{Name: "command-prompt", Desc: "Command prompt"},
		Action{Name: "detach", Desc: "Detach"},
		Action{Name: "send-prefix", Desc: "Send the prefix key"},
	)
}

// Actions returns every action, in which-key order. The caller must not
// change the result.
func Actions() []Action { return actions }

// ActionByName returns the action called name.
func ActionByName(name string) (Action, bool) {
	for _, a := range actions {
		if a.Name == name {
			return a, true
		}
	}
	return Action{}, false
}

// SendPrefix is the action that types the prefix key into the pane.
const SendPrefix = "send-prefix"

// defaults are tmux's bindings, in the order which-key lists them.
var defaults = buildDefaults()

type binding struct{ key, action string }

func buildDefaults() []binding {
	b := []binding{
		{"%", "split-right"}, {`"`, "split-down"}, {"c", "new-window"}, {"a", "new-claude"}, {"m", "model-picker"}, {"n", "next-window"}, {"p", "prev-window"},
	}
	for i := range 10 {
		n := strconv.Itoa(i)
		b = append(b, binding{n, "select-window-" + n})
	}
	b = append(b, binding{",", "rename-window"}, binding{"x", "close-pane"}, binding{"&", "close-window"},
		binding{"z", "zoom"}, binding{"E", "even-out"})
	for _, d := range directions {
		name := strings.ToUpper(d[:1]) + d[1:]
		b = append(b, binding{name, "focus-" + d}, binding{"C-" + name, "resize-" + d}, binding{"M-" + name, "resize-" + d + "-5"})
	}
	return append(b, binding{"[", "copy-mode"}, binding{"s", "sessions"}, binding{":", "command-prompt"}, binding{"d", "detach"})
}
