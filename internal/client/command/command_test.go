package command

import (
	"errors"
	"slices"
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

func names(cs []Command) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return out
}

// TestMatchCommands: the rank order for a query is the order the home page
// showed before the registry existed.
func TestMatchCommands(t *testing.T) {
	tests := []struct {
		query string
		want  []string
	}{
		{"", []string{"new", "sessions", "rename", "kill", "theme", "help", "quit"}},
		{"n", []string{"new", "rename", "sessions"}},
		{"re", []string{"rename"}},
		{"s", []string{"sessions"}},
		{"t", []string{"theme", "quit"}},
		{"xyz", nil},
	}
	for _, tt := range tests {
		if got := names(Match(tt.query, Home)); !slices.Equal(got, tt.want) {
			t.Errorf("Match(%q) = %v, want %v", tt.query, got, tt.want)
		}
	}
}

func TestRegistryRegister(t *testing.T) {
	r := NewRegistry()
	c := Command{Name: "fake", Args: "<name>", Desc: "A fake command", Group: GroupConversation, Where: Chat}
	if err := r.Register(c); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if got := names(r.Match("", Chat)); !slices.Equal(got, []string{"fake"}) {
		t.Fatalf("Match = %v, want the registered command", got)
	}
	err := r.Register(c)
	var dup *DuplicateError
	if !errors.As(err, &dup) || !errors.Is(err, ErrDuplicate) {
		t.Fatalf("second Register = %v, want a *DuplicateError", err)
	}
	if dup.Name != "fake" || dup.Place != Chat {
		t.Fatalf("DuplicateError = %+v", dup)
	}
}

// TestRegistryPlaces: one name may run in two places when the entries do
// not overlap.
func TestRegistryPlaces(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(Command{Name: "rename", Where: Home}); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(Command{Name: "rename", Where: Chat}); err != nil {
		t.Fatalf("rename in chat: %v", err)
	}
	if err := r.Register(Command{Name: "rename", Where: Home | Chat}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("overlapping Register = %v, want ErrDuplicate", err)
	}
}

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		query string
		first string
	}{
		{"ses", "sessions"},
		{"re", "rename"},
		{"k", "kill"},
		{"q", "quit"},
		{"nw", "new"},
		{"", "new"}, // no query keeps the table order
	}
	for _, tt := range tests {
		got := Match(tt.query, Home)
		if len(got) == 0 || got[0].Name != tt.first {
			t.Errorf("Match(%q) ranks %v first, want %s", tt.query, names(got), tt.first)
		}
	}
	if got := names(Match("exit", Home)); slices.Contains(got, "exit") {
		t.Errorf("Match(exit) = %v; /exit is gone, /quit quits", got)
	}
	if got := Match("xyz", Home); len(got) != 0 {
		t.Errorf("Match(xyz) = %v, want nothing", names(got))
	}
}

// TestMatchFiltersByPlace: each place lists only the commands that work
// there, in table order.
func TestMatchFiltersByPlace(t *testing.T) {
	home := names(Match("", Home))
	if want := []string{"new", "sessions", "rename", "kill", "theme", "help", "quit"}; !slices.Equal(home, want) {
		t.Errorf("home commands %v, want %v", home, want)
	}
	inSession := names(Match("", InSession))
	if want := []string{"new", "switch", "rename", "kill", "chat", "theme"}; !slices.Equal(inSession, want) {
		t.Errorf("session commands %v, want %v", inSession, want)
	}
	if chat := Match("", Chat); len(chat) != 0 {
		t.Errorf("chat commands %v, want none before the feature packages add them", names(chat))
	}
}

func TestFind(t *testing.T) {
	if c, ok := Find("rename", InSession); !ok || c.Args != "[name]" {
		t.Errorf("Find(rename, InSession) = %+v %v, want the current-session rename", c, ok)
	}
	if c, ok := Find("rename", Home); !ok || c.Args != "[session] <name>" || !c.Session {
		t.Errorf("Find(rename, Home) = %+v %v, want the rename that picks a session", c, ok)
	}
	if _, ok := Find("quit", InSession); ok {
		t.Error("quit is found in a session; it belongs to the home page")
	}
	if _, ok := Find("nope", Home); ok {
		t.Error("an unknown name is found")
	}
}

func TestSplit(t *testing.T) {
	tests := []struct{ in, word, rest string }{
		{"rename  backend ", "rename", "backend"},
		{"kill", "kill", ""},
		{"", "", ""},
		{"theme One Half Dark", "theme", "One Half Dark"},
	}
	for _, tt := range tests {
		if w, r := Split(tt.in); w != tt.word || r != tt.rest {
			t.Errorf("Split(%q) = %q, %q; want %q, %q", tt.in, w, r, tt.word, tt.rest)
		}
	}
}

func TestLine(t *testing.T) {
	c, _ := Find("kill", Home)
	if got := c.Line(); got != "kill [session]" {
		t.Errorf("Line = %q", got)
	}
	c, _ = Find("help", Home)
	if got := c.Line(); got != "help" {
		t.Errorf("Line = %q", got)
	}
}

func TestReason(t *testing.T) {
	if got := Reason(api.ValidSessionName("")); got != "the name is empty" {
		t.Errorf("Reason = %q, want the message without the prefix", got)
	}
}
