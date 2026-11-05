package server

import (
	"strings"
	"sync/atomic"
	"testing"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/provider/providertest"
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
	// The two panes and the border between them fill the window's width.
	width := 1
	for _, p := range shownPanes(l) {
		width += p.Rect.W
	}
	if width != 100 {
		t.Fatalf("panes and border are %d columns wide, want 100", width)
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
	if e == nil || !strings.Contains(e.Message, "terminal, git, note") {
		t.Fatalf("open files answered %v, want an error naming terminal, git, note", e)
	}
}

// chatSideServer is a chat server with side kinds, and the fake provider

// chatSideServer is a chat server with side kinds, and the fake provider
// playing replies.
func chatSideServer(t *testing.T, side []sideKind, replies ...providertest.Reply) *Server {
	t.Helper()
	opts, _, _ := chatOptions(t, replies...)
	opts.sideKinds = side
	s := New(opts)
	t.Cleanup(s.Close)
	return s
}

// TestOpenTooSmall: /open in a window with no room refuses and leaves the
// panes as they were.
func TestOpenTooSmall(t *testing.T) {
	dir := t.TempDir() // before the server, so its shell has ended when the folder goes
	s, _ := newNoteServer(t)
	c := connect(t, s, 2, 30) // two columns: a pane on the right cannot fit
	c.Send(&api.Attach{Session: "notes", Create: true, Dir: dir})
	first := shownPanes(nextLayout(t, c, func(*api.Layout) bool { return true }))[0].ID
	c.Send(&api.Command{Op: api.OpOpen, Target: first, Args: map[string]string{api.ArgKind: "note"}})
	if e := nextError(t, c); e.Code != api.CodeCommandFailed || !strings.Contains(e.Message, "minimum") {
		t.Fatalf("got %+v, want the open refused", e)
	}
	var n int
	s.call(func(st *state) { n = len(st.sessions["notes"].shown().panes) })
	if n != 1 {
		t.Fatalf("%d panes after the refused open, want 1", n)
	}
}

// TestOpenKeepsChatRunning: a chat turn that streams goes on while /open
// adds a pane beside it.
func TestOpenKeepsChatRunning(t *testing.T) {
	hold := make(chan struct{})
	r := reply("hel", "lo")
	r.Hold = hold
	s := chatSideServer(t, []sideKind{noteKind(new(atomic.Int32))}, r)
	c := connect(t, s, 100, 30)
	chat := openChat(t, c)
	say(c, chat, "hi")
	c.WaitChat(chat, func(s *api.ChatSnapshot) bool { return s.Running })
	c.Send(&api.Command{Op: api.OpOpen, Target: chat, Args: map[string]string{api.ArgKind: "note"}})
	nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	if !c.Chat(chat).Running {
		t.Fatal("the turn stopped when the pane opened")
	}
	close(hold)
	got := c.WaitChat(chat, func(s *api.ChatSnapshot) bool { return !s.Running })
	if lastReply(got) != "hello" {
		t.Fatalf("reply %q, want the whole stream", lastReply(got))
	}
}
