// Package command is the command registry: the names, arguments, and
// descriptions of the commands the home page (typed after "/") and the
// session screen (Ctrl+b :) offer, and the fuzzy match that lists them. It
// holds no behavior: each screen maps a name to what it does there, so the
// registry imports no screen and other command lists can reuse it.
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
)

// Command is one command.
type Command struct {
	Name    string
	Args    string // the arguments, written for people: "[name]" is optional, "<name>" required
	Desc    string
	Session bool  // acts on a session the user can pick from the home page's list
	Where   Place // where it runs
}

// Line is the command with its arguments, as a list shows it.
func (c Command) Line() string { return strings.TrimSpace(c.Name + " " + c.Args) }

// commands lists every command in the order lists show them. A name may
// appear once per place, with arguments that fit that place.
var commands = []Command{
	{Name: "new", Args: "[name]", Desc: "Start or open a session", Where: Home | InSession},
	{Name: "sessions", Desc: "Show the sessions", Where: Home},
	{Name: "switch", Args: "[session]", Desc: "Switch to another session", Where: InSession},
	{Name: "rename", Args: "[session] <name>", Desc: "Rename a session", Session: true, Where: Home},
	{Name: "rename", Args: "[name]", Desc: "Rename this session", Where: InSession},
	{Name: "kill", Args: "[session]", Desc: "End a session and its programs", Session: true, Where: Home | InSession},
	{Name: "theme", Args: "[name]", Desc: "Pick the colors", Where: Home | InSession},
	{Name: "help", Desc: "Show commands and keys", Where: Home},
	{Name: "quit", Desc: "Leave Snow; sessions keep running", Where: Home},
}

// In returns the commands that run in where, in table order.
func In(where Place) []Command {
	var out []Command
	for _, c := range commands {
		if c.Where&where != 0 {
			out = append(out, c)
		}
	}
	return out
}

// Find returns the command called name that runs in where.
func Find(name string, where Place) (Command, bool) {
	cs := In(where)
	if i := slices.IndexFunc(cs, func(c Command) bool { return c.Name == name }); i >= 0 {
		return cs[i], true
	}
	return Command{}, false
}

// Match returns the commands in where whose names fuzzy-match query, best
// first; equal scores keep the table order.
func Match(query string, where Place) []Command {
	type scored struct {
		c     Command
		score int
	}
	var found []scored
	for _, c := range In(where) {
		if s, ok := fuzzyScore(query, c.Name); ok {
			found = append(found, scored{c, s})
		}
	}
	slices.SortStableFunc(found, func(a, b scored) int { return b.score - a.score })
	out := make([]Command, len(found))
	for i, f := range found {
		out[i] = f.c
	}
	return out
}

// Split cuts typed text into the command's name and the rest, trimmed.
func Split(text string) (word, rest string) {
	word, rest, _ = strings.Cut(strings.TrimSpace(text), " ")
	return word, strings.TrimSpace(rest)
}

// Reason returns err's message without the protocol's error prefix, to
// tell the user why a command's argument was refused.
func Reason(err error) string {
	msg := err.Error()
	if p := api.ErrInvalidSessionRequest.Error() + ": "; errors.Is(err, api.ErrInvalidSessionRequest) {
		msg = strings.TrimPrefix(msg, p)
	}
	return msg
}
