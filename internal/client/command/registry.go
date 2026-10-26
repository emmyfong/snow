package command

import (
	"errors"
	"fmt"
	"slices"
)

// Errors about the registry.
var (
	// ErrDuplicate means a command with the same name already runs in one
	// of the places the new command names.
	ErrDuplicate = errors.New("command: duplicate name")
)

// DuplicateError reports a command that is already registered.
type DuplicateError struct {
	Name  string
	Place Place
}

// Error implements error.
func (e *DuplicateError) Error() string {
	return fmt.Sprintf("%v %q in %s", ErrDuplicate, e.Name, e.Place)
}

// Unwrap makes errors.Is(err, ErrDuplicate) true.
func (e *DuplicateError) Unwrap() error { return ErrDuplicate }

// places lists the places in the order a registry checks and returns them.
var places = []Place{Home, InSession, Chat}

// Registry holds the commands of every place. Commands keep the order they
// were registered in; Match reorders them by score for one query.
type Registry struct {
	lists map[Place][]Command
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{lists: make(map[Place][]Command, len(places))}
}

// Register adds c to every place it names. It returns a *DuplicateError
// when the same name already runs in one of those places, so a later
// package cannot silently replace a command.
func (r *Registry) Register(c Command) error {
	if c.Name == "" {
		return errors.New("command: empty name")
	}
	if c.Where == 0 {
		return fmt.Errorf("command: %q runs nowhere", c.Name)
	}
	for _, p := range places {
		if c.Where&p == 0 {
			continue
		}
		if slices.ContainsFunc(r.lists[p], func(old Command) bool { return old.Name == c.Name }) {
			return &DuplicateError{Name: c.Name, Place: p}
		}
	}
	for _, p := range places {
		if c.Where&p != 0 {
			r.lists[p] = append(r.lists[p], c)
		}
	}
	return nil
}

// In returns the commands that run in where, in registration order.
func (r *Registry) In(where Place) []Command {
	return slices.Clone(r.lists[where])
}

// Find returns the command called name that runs in where.
func (r *Registry) Find(name string, where Place) (Command, bool) {
	if i := slices.IndexFunc(r.lists[where], func(c Command) bool { return c.Name == name }); i >= 0 {
		return r.lists[where][i], true
	}
	return Command{}, false
}

// Match returns the commands in where whose names fuzzy-match query, best
// first; equal scores keep the registration order.
func (r *Registry) Match(query string, where Place) []Command {
	type scored struct {
		c     Command
		score int
	}
	var found []scored
	for _, c := range r.lists[where] {
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

// defaultRegistry holds the commands Snow ships with. A feature package
// adds its own with Register, so the screens need no edit.
var defaultRegistry = NewRegistry()

// Default returns the registry the screens read.
func Default() *Registry { return defaultRegistry }

// Register adds c to the registry the screens read. Call it while the
// program starts, for example from an init function.
func Register(c Command) error { return defaultRegistry.Register(c) }

// In returns the commands that run in where, in registration order.
func In(where Place) []Command { return defaultRegistry.In(where) }

// Find returns the command called name that runs in where.
func Find(name string, where Place) (Command, bool) { return defaultRegistry.Find(name, where) }

// Match returns the commands in where that fuzzy-match query, best first.
func Match(query string, where Place) []Command { return defaultRegistry.Match(query, where) }

// builtins are the commands Snow ships with, in the order lists show them.
var builtins = []Command{
	{Name: "new", Args: "[name]", Desc: "Start or open a session", Group: GroupConversation, Where: Home | InSession},
	{Name: "sessions", Desc: "Show the sessions", Group: GroupConversation, Where: Home},
	{Name: "switch", Args: "[session]", Desc: "Switch to another session", Group: GroupConversation, Where: InSession},
	{Name: "rename", Args: "[session] <name>", Desc: "Rename a session", Group: GroupConversation, Session: true, Where: Home},
	{Name: "rename", Args: "[name]", Desc: "Rename this session", Group: GroupConversation, Where: InSession},
	{Name: "kill", Args: "[session]", Desc: "End a session and its programs", Group: GroupConversation, Session: true, Where: Home | InSession},
	{Name: "chat", Desc: "Open a chat with the agent beside this pane", Group: GroupConversation, Where: InSession},
	{Name: "theme", Args: "[name]", Desc: "Pick the colors", Group: GroupSnow, Where: Home | InSession},
	{Name: "help", Desc: "Show commands and keys", Group: GroupSnow, Where: Home},
	{Name: "quit", Desc: "Leave Snow; sessions keep running", Group: GroupSnow, Where: Home},
}

func init() {
	for _, c := range builtins {
		if err := defaultRegistry.Register(c); err != nil {
			panic(err)
		}
	}
}
