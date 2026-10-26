// Package command is the command registry for the home page (typed after
// "/"), the session screen (Ctrl+b :), and the chat input. It holds command
// names, arguments, groups, and matching, but no screen behavior. Each
// screen maps a name to what it does there, so the registry imports no
// screen and feature packages can add their commands without editing the
// screens.
package command

import (
	"errors"
	"slices"
	"strings"

	"github.com/emmyfong/snow/pkg/api"
)

// Place says where a command runs. A command can run in several places.
type Place uint8

// Places.
const (
	Home      Place = 1 << iota // the home page
	InSession                   // the session screen's command list
	Chat                        // the chat input's slash command list
)

// String names a place for an error.
func (p Place) String() string {
	switch p {
	case Home:
		return "home page"
	case InSession:
		return "session screen"
	case Chat:
		return "chat input"
	default:
		return "unknown place"
	}
}

// Group is the section a command list shows a command under.
type Group string

// Groups, in the order the command list shows them.
const (
	GroupModel        Group = "Model"
	GroupConversation Group = "Conversation"
	GroupWorkspace    Group = "Workspace"
	GroupSnow         Group = "Snow"
)

// groups orders the sections of a command list.
var groups = []Group{GroupModel, GroupConversation, GroupWorkspace, GroupSnow}

// Command is one command.
type Command struct {
	Name    string
	Args    string // the arguments, written for people: "[name]" is optional, "<name>" required
	Desc    string
	Group   Group // the section a grouped list shows it under
	Session bool  // acts on a session the user can pick from the home page's list
	Where   Place // where it runs
}

// Line is the command with its arguments, as a list shows it.
func (c Command) Line() string { return strings.TrimSpace(c.Name + " " + c.Args) }

// Grouped sorts cmds by group, in the order a list shows them. The order
// within a group stays as it was.
func Grouped(cmds []Command) []Command {
	out := slices.Clone(cmds)
	rank := func(g Group) int {
		if i := slices.Index(groups, g); i >= 0 {
			return i
		}
		return len(groups)
	}
	slices.SortStableFunc(out, func(a, b Command) int { return rank(a.Group) - rank(b.Group) })
	return out
}

// Split cuts typed text into the command's name and the rest, trimmed.
func Split(text string) (word, rest string) {
	word, rest, _ = strings.Cut(strings.TrimSpace(text), " ")
	return word, strings.TrimSpace(rest)
}

// ThemeMsg asks the client to use the theme called Name everywhere and to
// save it in the config file. The home page and the session screen send it.
type ThemeMsg struct{ Name string }

// Reason returns err's message without the protocol's error prefix, to
// tell the user why a command's argument was refused.
func Reason(err error) string {
	msg := err.Error()
	if p := api.ErrInvalidSessionRequest.Error() + ": "; errors.Is(err, api.ErrInvalidSessionRequest) {
		msg = strings.TrimPrefix(msg, p)
	}
	return msg
}
