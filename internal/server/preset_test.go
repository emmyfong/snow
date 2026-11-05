package server

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/pkg/api"
)

// TestPresetsMatchAPI: every preset the protocol lists has a valid
// template, and the server has no other.
func TestPresetsMatchAPI(t *testing.T) {
	if len(presets) != len(api.LayoutPresets) {
		t.Fatalf("%d templates for %d presets", len(presets), len(api.LayoutPresets))
	}
	for _, p := range api.LayoutPresets {
		tmpl, ok := presets[p.Name]
		if !ok {
			t.Fatalf("no template for %q", p.Name)
		}
		if err := tmpl.Validate(); err != nil {
			t.Errorf("%s: %v", p.Name, err)
		}
	}
}

// layoutCmd asks c to arrange the window of pane target as preset name.
func layoutCmd(c *apitest.Client, target int, name string) {
	c.Send(&api.Command{Op: api.OpLayout, Target: target, Args: map[string]string{api.ArgName: name}})
}

// paneState returns what the server holds for pane id: its chat and its
// terminal, to compare before and after a layout.
func paneState(t *testing.T, s *Server, id int) (chat, term any) {
	t.Helper()
	s.call(func(st *state) {
		for _, sess := range st.sessions {
			if _, p := sess.findPane(id); p != nil {
				chat, term = p.chat, p.term
			}
		}
	})
	return chat, term
}

func windowNamed(l *api.Layout, name string) *api.WindowInfo {
	for i := range l.Windows {
		if l.Windows[i].Name == name {
			return &l.Windows[i]
		}
	}
	return nil
}

// TestLayoutKeepsPanes: /layout ide keeps the chat and the shell, the same
// panes with the same conversation and PTY, and puts the chat over the
// shell.
func TestLayoutKeepsPanes(t *testing.T) {
	s, _, _ := newHeldServer(t)
	c := connect(t, s, 100, 30)
	chat, shell := openChatBesideShell(t, c)
	chatBefore, _ := paneState(t, s, chat)
	_, termBefore := paneState(t, s, shell)

	layoutCmd(c, chat, "ide")
	if e := nextError(t, c); e.Code != api.CodeNotice {
		t.Fatalf("got %+v, want the notice on the skipped kinds", e)
	}
	l := nextLayout(t, c, func(l *api.Layout) bool {
		ps := shownPanes(l)
		return len(ps) == 2 && ps[0].ID == chat && ps[1].ID == shell && ps[1].Rect.Y > ps[0].Rect.Y
	})
	if windowNamed(l, "moved") != nil {
		t.Fatal("a moved window with nothing to move")
	}
	if got, _ := paneState(t, s, chat); got != chatBefore {
		t.Fatal("the chat pane was replaced")
	}
	if _, got := paneState(t, s, shell); got != termBefore {
		t.Fatal("the shell's PTY was replaced")
	}
}

// TestLayoutSkipsUnregisteredKind: the kinds the server cannot open yet
// are left out, and a notice names them.
func TestLayoutSkipsUnregisteredKind(t *testing.T) {
	s, _, _ := newHeldServer(t)
	c := connect(t, s, 100, 30)
	chat, _ := openChatBesideShell(t, c)
	layoutCmd(c, chat, "ide")
	e := nextError(t, c)
	if e.Code != api.CodeNotice || !strings.Contains(e.Message, "files, viewer, git") {
		t.Fatalf("got %+v, want a notice naming files, viewer, git", e)
	}
}

// TestLayoutMovesOtherPanes: panes the preset does not hold go to a new
// window named moved, which is not shown; nothing closes.
func TestLayoutMovesOtherPanes(t *testing.T) {
	s, _, _ := newHeldServer(t)
	c := connect(t, s, 100, 30)
	chat, shell := openChatBesideShell(t, c)
	_, termBefore := paneState(t, s, shell)
	layoutCmd(c, chat, "focus")
	l := nextLayout(t, c, func(l *api.Layout) bool { return windowNamed(l, "moved") != nil })
	if ps := shownPanes(l); len(ps) != 1 || ps[0].ID != chat {
		t.Fatalf("shown panes %+v, want the chat alone", ps)
	}
	moved := windowNamed(l, "moved")
	if l.Active == moved.Index || len(moved.Panes) != 1 || moved.Panes[0].ID != shell {
		t.Fatalf("moved window %+v, active %d", moved, l.Active)
	}
	if _, got := paneState(t, s, shell); got != termBefore {
		t.Fatal("the moved shell was replaced")
	}
}

// TestLayoutOpensMissingKind: a kind the window lacks opens through the
// registry; here focus opens a chat beside nothing but a shell.
func TestLayoutOpensMissingKind(t *testing.T) {
	s, h, _ := newHeldServer(t)
	c := connect(t, s, 100, 30)
	chat, shell := openChatBesideShell(t, c)
	c.Send(&api.Command{Op: api.OpClose, Target: chat})
	nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 1 })
	starts := h.calls.Load()
	layoutCmd(c, shell, "focus")
	l := nextLayout(t, c, func(l *api.Layout) bool { return chatIn(l) != 0 })
	if ps := shownPanes(l); len(ps) != 1 || ps[0].Kind != api.PaneChat || !ps[0].Focused {
		t.Fatalf("shown panes %+v, want one focused chat", ps)
	}
	if n := h.calls.Load(); n != starts+1 {
		t.Fatalf("%d conversation starts, want 1", n-starts)
	}
}

// TestLayoutTooSmall: a window too small for the preset refuses, and the
// layout stays as it was.
func TestLayoutTooSmall(t *testing.T) {
	s, _, _ := newHeldServer(t)
	c := connect(t, s, 100, 2) // one row for panes: chat over shell cannot fit
	chat, shell := openChatBesideShell(t, c)
	layoutCmd(c, chat, "ide")
	if e := nextError(t, c); e.Code != api.CodeCommandFailed || !strings.Contains(e.Message, "minimum") {
		t.Fatalf("got %+v, want the layout refused", e)
	}
	var ids []int
	s.call(func(st *state) {
		for _, sess := range st.sessions {
			for _, id := range sess.shown().tree.Panes() {
				ids = append(ids, int(id))
			}
		}
	})
	if !slices.Contains(ids, chat) || !slices.Contains(ids, shell) || len(ids) != 2 {
		t.Fatalf("panes %v after the refused layout", ids)
	}
}

// TestLayoutUnknownPreset: an unknown name fails and lists the presets.
func TestLayoutUnknownPreset(t *testing.T) {
	s, _, _ := newHeldServer(t)
	c := connect(t, s, 100, 30)
	chat := openChat(t, c)
	layoutCmd(c, chat, "grid")
	if e := nextError(t, c); e.Code != api.CodeCommandFailed || !strings.Contains(e.Message, "ide, focus, review") {
		t.Fatalf("got %+v, want an error listing the layouts", e)
	}
}

// TestLayoutFailedOpenKeepsIDs: a layout whose second new pane fails to
// open closes the first, and its ID is not given to the next pane, so the
// closed pane's exit cannot remove that pane.
func TestLayoutFailedOpenKeepsIDs(t *testing.T) {
	broken := sideKind{Name: "git", Title: "Git", Open: func(sideContext) (sidePane, error) { return nil, errors.New("no git") }}
	s := chatSideServer(t, []sideKind{broken})
	c := connect(t, s, 100, 30)
	chat := openChat(t, c)
	var reserved int
	s.call(func(st *state) { reserved = st.nextPane })
	layoutCmd(c, chat, "ide") // opens a terminal, then git fails
	if e := nextError(t, c); e.Code != api.CodeCommandFailed {
		t.Fatalf("got %+v, want the layout refused", e)
	}
	c.Send(&api.Command{Op: api.OpSplit, Target: chat, Args: map[string]string{api.ArgDir: api.DirLeft}})
	l := nextLayout(t, c, func(l *api.Layout) bool { return terminalIn(l) != 0 })
	if id := terminalIn(l); id <= reserved {
		t.Fatalf("new pane %d reuses an ID the failed layout reserved (from %d)", id, reserved)
	}
}
