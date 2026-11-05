package server

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/store"
	"github.com/emmyfong/snow/pkg/api"
)

// noteState is the state of the test side kind "note".
type noteState struct {
	Text string `json:"text"`
}

func (noteState) Kind() string    { return "note" }
func (noteState) Validate() error { return nil }

// notePane is a side pane that holds a line of text.
type notePane struct {
	text   string
	closed *atomic.Int32
}

func (n *notePane) State() api.SideState { return noteState{Text: n.text} }

func (n *notePane) Act(action string, args map[string]string) error {
	if action != "set" {
		return fmt.Errorf("note has no action %q", action)
	}
	n.text = args["text"]
	return nil
}

func (n *notePane) Close() error {
	n.closed.Add(1)
	return nil
}

// noteKind registers notePane; closed counts its closes.
func noteKind(closed *atomic.Int32) sideKind {
	return sideKind{Name: "note", Title: "Note", Open: func(ctx sideContext) (sidePane, error) {
		return &notePane{text: "in " + ctx.Folder, closed: closed}, nil
	}}
}

func TestPaneKindRegistration(t *testing.T) {
	open := func(sideContext) (sidePane, error) { return nil, nil }
	tests := []struct {
		name    string
		chats   bool
		side    []sideKind
		want    []string
		wantErr error
	}{
		{name: "built in", want: []string{api.PaneTerminal}},
		{name: "with chats", chats: true, want: []string{api.PaneTerminal, api.PaneChat}},
		{
			name: "side kinds keep their order", chats: true,
			side: []sideKind{{Name: "files", Open: open}, {Name: "git", Open: open}, {Name: "agenda", Open: open}},
			want: []string{api.PaneTerminal, api.PaneChat, "files", "git", "agenda"},
		},
		{name: "duplicate side kind", side: []sideKind{{Name: "files", Open: open}, {Name: "files", Open: open}}, wantErr: ErrDuplicateKind},
		{name: "duplicate of a built in", side: []sideKind{{Name: api.PaneTerminal, Open: open}}, wantErr: ErrDuplicateKind},
		{name: "chat taken when chats are on", chats: true, side: []sideKind{{Name: api.PaneChat, Open: open}}, wantErr: ErrDuplicateKind},
		{name: "malformed name", side: []sideKind{{Name: "Files", Open: open}}, wantErr: api.ErrInvalidKind},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k, err := newPaneKinds(tt.chats, tt.side...)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			var got []string
			for _, info := range k.kinds() {
				got = append(got, info.Name)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("kinds %v, want %v", got, tt.want)
			}
		})
	}
	if _, err := newPaneKinds(false, sideKind{Name: "files"}); err == nil {
		t.Error("a kind with no Open registered")
	}
}

// newNoteServer is a test server with the note kind registered.
func newNoteServer(t *testing.T) (*Server, *atomic.Int32) {
	t.Helper()
	closed := new(atomic.Int32)
	opts := testOptions()
	opts.sideKinds = []sideKind{noteKind(closed)}
	s := New(opts)
	t.Cleanup(s.Close)
	return s, closed
}

// splitNote splits pane target with a note pane and returns its ID.
func splitNote(t *testing.T, c *apitest.Client, target int) int {
	t.Helper()
	c.Send(&api.Command{Op: api.OpSplit, Target: target, Args: map[string]string{api.ArgDir: api.DirRight, api.ArgKind: "note"}})
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	for _, p := range shownPanes(l) {
		if p.ID != target {
			if p.Kind != "note" {
				t.Fatalf("new pane %+v, want kind note", p)
			}
			return p.ID
		}
	}
	t.Fatal("no new pane")
	return 0
}

// TestPaneKindProtocol: a side pane opens by kind, its state reaches the
// client, an action changes it, and closing it closes the side pane.
func TestPaneKindProtocol(t *testing.T) {
	dir := t.TempDir() // before the server, so its shell has ended when the folder goes
	s, closed := newNoteServer(t)
	c := connect(t, s, 100, 30)
	c.Send(&api.Attach{Session: "notes", Create: true, Dir: dir})
	first := shownPanes(nextLayout(t, c, func(*api.Layout) bool { return true }))[0].ID
	if _, e := apitest.Reply[*api.PaneKinds](c); e != nil {
		t.Fatal(e)
	}
	if got := c.Kinds(); len(got) != 2 || got[1] != (api.PaneKindInfo{Name: "note", Title: "Note"}) {
		t.Fatalf("kinds %+v, want terminal then note", got)
	}

	note := splitNote(t, c, first)
	var st noteState
	c.WaitSide(note, func(s *api.SidePaneState) bool { return s.Rev == 1 })
	if err := api.DecodeSideState(c.Side(note), &st); err != nil || !strings.HasPrefix(st.Text, "in ") {
		t.Fatalf("first state %+v, %v", st, err)
	}

	c.Send(&api.SidePaneAction{Pane: note, Action: "set", Args: map[string]string{"text": "hello"}})
	c.WaitSide(note, func(s *api.SidePaneState) bool { return s.Rev == 2 })
	if err := api.DecodeSideState(c.Side(note), &st); err != nil || st.Text != "hello" {
		t.Fatalf("state after set %+v, %v", st, err)
	}

	c.Send(&api.SidePaneAction{Pane: note, Action: "explode"})
	if _, e := apitest.Reply[*api.SidePaneState](c); e == nil || e.Code != api.CodeCommandFailed {
		t.Fatalf("unknown action answered %v, want a command failure", e)
	}
	c.Send(&api.SidePaneAction{Pane: first, Action: "set"})
	if _, e := apitest.Reply[*api.SidePaneState](c); e == nil || e.Code != api.CodeBadCommand {
		t.Fatalf("action on a terminal answered %v, want a bad command", e)
	}

	c.Send(&api.Command{Op: api.OpClose, Target: note})
	nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 1 })
	eventually(t, "the note pane closes", func() bool { return closed.Load() == 1 })
}

// TestPaneKindSecondClient: a client that attaches later gets the side
// pane's current state.
func TestPaneKindSecondClient(t *testing.T) {
	dir := t.TempDir() // before the server, so its shell has ended when the folder goes
	s, _ := newNoteServer(t)
	a := connect(t, s, 100, 30)
	a.Send(&api.Attach{Session: "notes", Create: true, Dir: dir})
	first := shownPanes(nextLayout(t, a, func(*api.Layout) bool { return true }))[0].ID
	note := splitNote(t, a, first)
	a.Send(&api.SidePaneAction{Pane: note, Action: "set", Args: map[string]string{"text": "shared"}})
	a.WaitSide(note, func(s *api.SidePaneState) bool { return s.Rev == 2 })

	b := connect(t, s, 100, 30)
	b.Attach("notes")
	var st noteState
	b.WaitSide(note, func(s *api.SidePaneState) bool { return s.Rev == 2 })
	if err := api.DecodeSideState(b.Side(note), &st); err != nil || st.Text != "shared" {
		t.Fatalf("second client state %+v, %v", st, err)
	}
}

// TestUnknownKindRefused: a well-formed kind the server does not have is
// refused, and the window is unchanged.
func TestUnknownKindRefused(t *testing.T) {
	dir := t.TempDir() // before the server, so its shell has ended when the folder goes
	s, _ := newNoteServer(t)
	c := connect(t, s, 100, 30)
	c.Send(&api.Attach{Session: "notes", Create: true, Dir: dir})
	first := shownPanes(nextLayout(t, c, func(*api.Layout) bool { return true }))[0].ID
	c.Send(&api.Command{Op: api.OpSplit, Target: first, Args: map[string]string{api.ArgDir: api.DirRight, api.ArgKind: "files"}})
	e := nextError(t, c)
	if e.Code != api.CodeCommandFailed || !strings.Contains(e.Message, "unknown pane kind") {
		t.Fatalf("error %+v, want an unknown kind refusal", e)
	}
}

// TestRestoreSidePane: a saved side pane opens again from its kind and
// folder, and a saved kind this server does not have opens a terminal.
func TestRestoreSidePane(t *testing.T) {
	tests := []struct {
		kind, wantKind string
	}{
		{kind: "note", wantKind: "note"},
		{kind: "files", wantKind: api.PaneTerminal},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			dir, folder := t.TempDir(), t.TempDir()
			snap := twoPanes(t, "")
			snap.Windows[0].Panes[1] = store.Pane{ID: 9, Kind: tt.kind, Folder: folder}
			writeSnapshot(t, dir, snap)
			opts := testOptions()
			opts.SessionsDir = dir
			opts.sideKinds = []sideKind{noteKind(new(atomic.Int32))}
			s := New(opts)
			t.Cleanup(s.Close)

			c := connect(t, s, 80, 24)
			c.Send(&api.Attach{Session: "api"})
			panes := nextLayout(t, c, func(*api.Layout) bool { return true }).Windows[0].Panes
			if len(panes) != 2 || panes[1].Kind != tt.wantKind {
				t.Fatalf("panes %+v, want the second of kind %s", panes, tt.wantKind)
			}
			if tt.wantKind != "note" {
				return
			}
			var st noteState
			c.WaitSide(panes[1].ID, func(*api.SidePaneState) bool { return true })
			if err := api.DecodeSideState(c.Side(panes[1].ID), &st); err != nil || st.Text != "in "+folder {
				t.Fatalf("restored state %+v, %v; want it opened in %s", st, err, folder)
			}
		})
	}
}
