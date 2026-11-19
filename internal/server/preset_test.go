package server

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/layout"
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

func windowWithPane(l *api.Layout, id int) *api.WindowInfo {
	for i := range l.Windows {
		for _, p := range l.Windows[i].Panes {
			if p.ID == id {
				return &l.Windows[i]
			}
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
	l := nextLayout(t, c, func(l *api.Layout) bool {
		ps := shownPanes(l)
		if len(ps) != 5 {
			return false
		}
		byID := map[int]api.PaneInfo{}
		var files, git, viewer api.PaneInfo
		for _, p := range ps {
			byID[p.ID] = p
			switch p.Kind {
			case api.PaneFiles:
				files = p
			case api.PaneGit:
				git = p
			case api.PaneViewer:
				viewer = p
			}
		}
		cp, sh := byID[chat], byID[shell]
		return cp.ID != 0 && sh.ID != 0 && files.ID != 0 && git.ID != 0 && viewer.ID != 0 &&
			sh.Rect.Y > cp.Rect.Y && files.Rect.X < cp.Rect.X && git.Rect.X > cp.Rect.X
	})
	for _, p := range shownPanes(l) {
		if p.Rect.Y < 1 || p.Rect.H < 1 || p.Rect.Y+p.Rect.H > 28 {
			t.Fatalf("preset content ignores chrome: %+v", p.Rect)
		}
	}
	var cp, sh, viewer api.PaneInfo
	for _, p := range shownPanes(l) {
		if p.Kind == api.PaneViewer {
			viewer = p
		}
		switch p.ID {
		case chat:
			cp = p
		case shell:
			sh = p
		}
	}
	if viewer.Rect.Y != 1 || cp.Rect.Y != viewer.Rect.Y+viewer.Rect.H+1 || sh.Rect.Y != cp.Rect.Y+cp.Rect.H+1 {
		t.Fatalf("preset headers do not reuse divider: viewer %+v, chat %+v, shell %+v", viewer, cp, sh)
	}
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

// TestLayoutSkipsUnregisteredKind: a layout that names a kind the server
// cannot open leaves it out, and a notice names it.
func TestLayoutSkipsUnregisteredKind(t *testing.T) {
	presets["spreadsheet"] = layout.Template{Axis: layout.Horizontal, Ratios: []float64{0.5, 0.5}, Children: []layout.Template{
		{Kind: api.PaneTerminal}, {Kind: "spreadsheet"},
	}}
	t.Cleanup(func() { delete(presets, "spreadsheet") })
	s, _, _ := newHeldServer(t)
	c := connect(t, s, 100, 30)
	chat, _ := openChatBesideShell(t, c)
	layoutCmd(c, chat, "spreadsheet")
	e := nextError(t, c)
	if e.Code != api.CodeNotice || !strings.Contains(e.Message, "spreadsheet") {
		t.Fatalf("got %+v, want a notice naming spreadsheet", e)
	}
}

// TestLayoutMovesOtherPanes: panes the preset does not hold go to a new
// window named after the first moved pane, which is not shown; nothing closes.
func TestLayoutMovesOtherPanes(t *testing.T) {
	s, _, _ := newHeldServer(t)
	c := connect(t, s, 100, 30)
	chat, shell := openChatBesideShell(t, c)
	_, termBefore := paneState(t, s, shell)
	layoutCmd(c, chat, "focus")
	e := nextError(t, c)
	if e.Code != api.CodeNotice || e.Message != "Moved 1 pane to a new window: "+shellProfile()+"." {
		t.Fatalf("got %+v, want a moved-pane notice", e)
	}
	l := nextLayout(t, c, func(l *api.Layout) bool { return windowWithPane(l, shell) != nil })
	if ps := shownPanes(l); len(ps) != 1 || ps[0].ID != chat {
		t.Fatalf("shown panes %+v, want the chat alone", ps)
	}
	moved := windowWithPane(l, shell)
	if moved.Name != shellProfile() || l.Active == moved.Index || len(moved.Panes) != 1 || moved.Panes[0].ID != shell {
		t.Fatalf("moved window %+v, active %d", moved, l.Active)
	}
	if _, got := paneState(t, s, shell); got != termBefore {
		t.Fatal("the moved shell was replaced")
	}
}

// TestLayoutTwiceNamesWindows: repeated focus layouts name each background
// window after its first pane.
func TestLayoutTwiceNamesWindows(t *testing.T) {
	s, _, _ := newHeldServer(t)
	c := connect(t, s, 100, 30)
	chat, shell := openChatBesideShell(t, c)
	layoutCmd(c, chat, "focus")
	if e := nextError(t, c); e.Code != api.CodeNotice || !strings.Contains(e.Message, shellProfile()) {
		t.Fatalf("first layout notice %+v", e)
	}
	nextLayout(t, c, func(l *api.Layout) bool { return windowWithPane(l, shell) != nil })

	c.Send(&api.Command{Op: api.OpOpen, Target: chat, Args: map[string]string{api.ArgKind: api.PaneFiles}})
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	files := 0
	for _, p := range shownPanes(l) {
		if p.Kind == api.PaneFiles {
			files = p.ID
		}
	}
	if files == 0 {
		t.Fatal("files pane did not open")
	}
	c.Send(&api.Command{Op: api.OpOpen, Target: chat, Args: map[string]string{api.ArgKind: api.PaneGit}})
	l = nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 3 })
	layoutCmd(c, chat, "focus")
	if e := nextError(t, c); e.Code != api.CodeNotice || e.Message != "Moved 2 panes to a new window: files." {
		t.Fatalf("second layout notice %+v", e)
	}
	l = nextLayout(t, c, func(l *api.Layout) bool { return windowWithPane(l, files) != nil })
	first := windowWithPane(l, shell)
	second := windowWithPane(l, files)
	if first == nil || second == nil || first.Name != shellProfile() || second.Name != api.PaneFiles || first.Name == second.Name || len(second.Panes) != 2 {
		t.Fatalf("background windows %+v and %+v; want distinct pane names", first, second)
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
	if e := nextError(t, c); e.Code != api.CodeNotice || e.Message != "Moved 1 pane to a new window: "+shellProfile()+"." {
		t.Fatalf("layout notice %+v", e)
	}
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
	if e := nextError(t, c); e.Code != api.CodeCommandFailed || !strings.Contains(e.Message, "minimum 10x3") {
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
