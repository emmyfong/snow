package server

import (
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/pkg/api"
)

func focusedID(l *api.Layout) int {
	for _, p := range shownPanes(l) {
		if p.Focused {
			return p.ID
		}
	}
	return 0
}

// TestOpenKind: open adds a pane of the kind on the right, focused; a
// second open focuses that pane instead of adding another.
func TestOpenKind(t *testing.T) {
	dir := t.TempDir() // before the server, so its shell has ended when the folder goes
	s, _ := newNoteServer(t)
	c := connect(t, s, 100, 30)
	c.Send(&api.Attach{Session: "notes", Create: true, Dir: dir})
	first := shownPanes(nextLayout(t, c, func(*api.Layout) bool { return true }))[0].ID

	open := func() {
		c.Send(&api.Command{Op: api.OpOpen, Target: first, Args: map[string]string{api.ArgKind: "note"}})
	}
	open()
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	note := focusedID(l)
	var notePane api.PaneInfo
	for _, p := range shownPanes(l) {
		if p.ID == note {
			notePane = p
		}
	}
	if note == first || notePane.Kind != "note" || notePane.Rect.X == 0 {
		t.Fatalf("after open: focused %+v, want a note pane on the right", notePane)
	}

	c.Send(&api.Command{Op: api.OpFocus, Target: first})
	nextLayout(t, c, func(l *api.Layout) bool { return focusedID(l) == first })
	open()
	l = nextLayout(t, c, func(l *api.Layout) bool { return focusedID(l) == note })
	if n := len(shownPanes(l)); n != 2 {
		t.Fatalf("second open made %d panes, want 2", n)
	}
}

// TestOpenUnknownKind: open of a kind the server does not have fails and
// names the kinds it has.
func TestOpenUnknownKind(t *testing.T) {
	dir := t.TempDir() // before the server, so its shell has ended when the folder goes
	s, _ := newNoteServer(t)
	c := connect(t, s, 100, 30)
	c.Send(&api.Attach{Session: "notes", Create: true, Dir: dir})
	first := shownPanes(nextLayout(t, c, func(*api.Layout) bool { return true }))[0].ID
	c.Send(&api.Command{Op: api.OpOpen, Target: first, Args: map[string]string{api.ArgKind: "files"}})
	_, e := apitest.Reply[*api.Layout](c)
	if e == nil || !strings.Contains(e.Message, "terminal, note") {
		t.Fatalf("open files answered %v, want an error naming terminal, note", e)
	}
}
