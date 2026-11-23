package session

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

// historyOf returns n numbered lines, as the server holds them.
func historyOf(n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("build step %d: compiling package", i)
	}
	return lines
}

// serve answers the scrollback requests r has recorded since the last call,
// from history, like the server.
func serve(m *Model, r *recorder, history []string) {
	for done := 0; done < len(r.sent); {
		req, ok := r.sent[done].(*api.ScrollbackRequest)
		done++
		if !ok {
			continue
		}
		var page []string
		if req.From < len(history) {
			page = history[req.From:min(req.From+req.Count, len(history))]
		}
		m.Update(&api.Scrollback{Pane: req.Pane, From: req.From, Total: len(history), Lines: page})
	}
	r.sent = r.sent[:0]
}

func inCopyMode(t *testing.T, cols, rows int) (*Model, *recorder, []string) {
	t.Helper()
	r := &recorder{}
	m := attached(t, cols, rows)
	m.send = r.send
	history := historyOf(300)
	// A wheel notch opens copy mode, as it does for the user.
	m.Update(tea.MouseWheelMsg{X: 5, Y: 6, Button: tea.MouseWheelUp})
	serve(m, r, history)
	// The notch leaves the view above the bottom; a notch back down returns
	// to it, with the cursor where the pane's was.
	m.Update(tea.MouseWheelMsg{X: 5, Y: 6, Button: tea.MouseWheelDown})
	return m, r, history
}

func TestPrefixBracketEntersCopyMode(t *testing.T) {
	m, r := newPrefixScreen(t)
	m.Update(ctrlB)
	m.Update(press('[', 0, "["))
	if len(r.sent) != 1 {
		t.Fatalf("sent %v, want one request for the history", r.sent)
	}
	req, ok := r.sent[0].(*api.ScrollbackRequest)
	if !ok || req.Pane != 7 {
		t.Fatalf("sent %#v, want a ScrollbackRequest for the focused pane 7", r.sent[0])
	}
}

// TestCopyModeKeepsKeysFromThePane: every key but Ctrl+C stays out of the
// pane while copy mode is open.
func TestCopyModeKeepsKeysFromThePane(t *testing.T) {
	m, r, _ := inCopyMode(t, 80, 24)
	r.sent = nil
	for _, k := range []tea.KeyPressMsg{
		press('k', 0, "k"),
		press('x', 0, "x"),
		press('b', tea.ModCtrl, ""), // the prefix key pages in copy mode
		press(tea.KeyUp, 0, ""),
		press(tea.KeyEnter, 0, ""),
	} {
		m.Update(k)
		if got := r.keys(); len(got) != 0 {
			t.Fatalf("copy mode sent keys %q to the pane for %+v", got, k)
		}
	}
	m.Update(press('c', tea.ModCtrl, ""))
	if m.copy != nil {
		t.Fatal("Ctrl+C left copy mode open")
	}
	if got := r.keys(); len(got) != 1 || got[0] != "ctrl+c" {
		t.Fatalf("the pane got %q, want [ctrl+c]", got)
	}
	// The pane gets keys again.
	m.Update(press('a', 0, "a"))
	if got := r.keys(); len(got) != 2 || got[1] != "a" {
		t.Fatalf("after Ctrl+C the pane got %q, want [ctrl+c a]", got)
	}
}

// TestCopyModeCtrlCReachesPane: Ctrl+C leaves copy mode and reaches the pane
// the history belongs to, so the program stops.
func TestCopyModeCtrlCReachesPane(t *testing.T) {
	r := &recorder{}
	m := attached(t, 80, 24)
	m.send = r.send
	m.Update(tea.MouseWheelMsg{X: 5, Y: 6, Button: tea.MouseWheelUp})
	serve(m, r, historyOf(300))
	r.sent = nil
	m.Update(press('c', tea.ModCtrl, ""))
	if m.copy != nil {
		t.Fatal("Ctrl+C left copy mode open")
	}
	if got := r.keys(); len(got) != 1 || got[0] != "ctrl+c" {
		t.Fatalf("the pane got %q, want [ctrl+c]", got)
	}
	if in := r.sent[0].(*api.Input); in.Pane != 1 {
		t.Fatalf("the key went to pane %d, want 1", in.Pane)
	}
}

// TestCopyModeCtrlCSendsToTheCopyPane: the wheel can open copy mode over a
// pane that does not have the focus. Ctrl+C goes to the pane whose history is
// shown, not to the focused pane.
func TestCopyModeCtrlCSendsToTheCopyPane(t *testing.T) {
	r := &recorder{}
	m := screen(80, 24)
	m.send = r.send
	m.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{
		Index: 1, Name: "dev",
		Panes: []api.PaneInfo{
			{ID: 1, Rect: api.Rect{Y: 1, W: 39, H: 21}, Profile: "sh"},
			{ID: 2, Rect: api.Rect{X: 40, Y: 1, W: 40, H: 21}, Profile: "sh", Focused: true},
		},
	}}})
	m.Update(tea.MouseWheelMsg{X: 5, Y: 6, Button: tea.MouseWheelUp})
	serve(m, r, historyOf(300))
	r.sent = nil
	m.Update(press('c', tea.ModCtrl, ""))
	if m.copy != nil {
		t.Fatal("Ctrl+C left copy mode open")
	}
	var inputs []*api.Input
	for _, msg := range r.sent {
		if in, ok := msg.(*api.Input); ok {
			inputs = append(inputs, in)
		}
	}
	if len(inputs) != 1 {
		t.Fatalf("sent %d inputs, want 1: %v", len(inputs), r.sent)
	}
	in := inputs[0]
	if in.Pane != 1 {
		t.Fatalf("the key went to pane %d, want the copy pane 1", in.Pane)
	}
	if in.Key == nil || in.Key.Code != "c" || len(in.Key.Mod) != 1 || in.Key.Mod[0] != api.ModCtrl || in.Key.Text != "" {
		t.Fatalf("the key was %#v, want Code c, Mod [ctrl], and no text", in.Key)
	}
}

func TestCopyModeClosesWithItsPane(t *testing.T) {
	m, r := newPrefixScreen(t)
	m.Update(ctrlB)
	m.Update(press('[', 0, "["))
	m.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{{ID: 8, Rect: api.Rect{W: 80, H: 24}}}}}})
	r.sent = nil
	m.Update(press('a', 0, "a"))
	if got := r.keys(); len(got) != 1 {
		t.Fatalf("keys after the pane closed: %q, want the key to reach the new pane", got)
	}
}

func goldenCopy(t *testing.T, cols, rows int, steps ...func(*Model, *recorder, []string)) {
	m, r, history := inCopyMode(t, cols, rows)
	for _, step := range steps {
		step(m, r, history)
		serve(m, r, history)
	}
	golden.RequireEqual(t, m.View().Content)
}

func scrollUp(pages int) func(*Model, *recorder, []string) {
	return func(m *Model, _ *recorder, _ []string) {
		for range pages {
			m.Update(press(tea.KeyPgUp, 0, ""))
		}
	}
}

func TestCopyModeBottomGolden80x24(t *testing.T)  { goldenCopy(t, 80, 24) }
func TestCopyModeBottomGolden120x40(t *testing.T) { goldenCopy(t, 120, 40) }

func TestCopyModeScrolledGolden80x24(t *testing.T)  { goldenCopy(t, 80, 24, scrollUp(3)) }
func TestCopyModeScrolledGolden120x40(t *testing.T) { goldenCopy(t, 120, 40, scrollUp(3)) }

func TestCopyModeDrawsInsideItsPane(t *testing.T) {
	m := screen(80, 24)
	panes := threePanes(80, 24)
	m.Update(&api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "dev", Panes: panes}}})
	fill(m, panes)
	r := &recorder{}
	m.send = r.send
	m.Update(ctrlB)
	m.Update(press('[', 0, "["))
	serve(m, r, historyOf(100))
	view := m.View().Content
	if !strings.Contains(view, "pane 1 row 0") || !strings.Contains(view, "pane 3 row 0") {
		t.Fatal("the other panes should keep drawing while copy mode is open")
	}
	if strings.Contains(view, "pane 2 row 0") {
		t.Fatal("the focused pane should show its history, not its screen")
	}
	if c := m.View().Cursor; c == nil || c.X < panes[1].Rect.X || c.Y >= panes[1].Rect.H {
		t.Fatalf("cursor %+v should sit inside pane 2", c)
	}
}

// searchFor types a search into copy mode and answers it from history, as
// the server would: every line that holds query is a match.
func searchFor(open string, query string) func(*Model, *recorder, []string) {
	return func(m *Model, _ *recorder, history []string) {
		m.Update(press(rune(open[0]), 0, open))
		for _, c := range query {
			m.Update(press(c, 0, string(c)))
		}
		m.Update(press(tea.KeyEnter, 0, ""))
		res := &api.SearchResult{Pane: 1, Query: query, Matches: []api.Match{}}
		for i, l := range history {
			if c := strings.Index(l, query); c >= 0 {
				res.Matches = append(res.Matches, api.Match{Line: i, Col: c, Len: len(query)})
			}
		}
		m.Update(res)
	}
}

func typing(open, text string) func(*Model, *recorder, []string) {
	return func(m *Model, _ *recorder, _ []string) {
		m.Update(press(rune(open[0]), 0, open))
		for _, c := range text {
			m.Update(press(c, 0, string(c)))
		}
	}
}

func TestSearchGolden80x24(t *testing.T)  { goldenCopy(t, 80, 24, searchFor("?", "step 2")) }
func TestSearchGolden120x40(t *testing.T) { goldenCopy(t, 120, 40, searchFor("?", "step 2")) }

func TestSearchPromptGolden80x24(t *testing.T)  { goldenCopy(t, 80, 24, typing("?", "step 2")) }
func TestSearchPromptGolden120x40(t *testing.T) { goldenCopy(t, 120, 40, typing("?", "step 2")) }

func TestSearchWrappedGolden80x24(t *testing.T) {
	goldenCopy(t, 80, 24, searchFor("/", "step 2"), func(m *Model, _ *recorder, _ []string) {
		for range 300 {
			m.Update(press('n', 0, "n"))
			if strings.Contains(statusLine(m), "Search wrapped") {
				return
			}
		}
		t.Fatal("the search never wrapped")
	})
}

// clipboardLog collects what the screen puts on the clipboard.
type clipboardLog struct{ texts []string }

func (c *clipboardLog) set(text string) tea.Cmd {
	c.texts = append(c.texts, text)
	return nil
}

func selecting(m *Model, _ *recorder, _ []string) {
	for _, k := range []tea.KeyPressMsg{
		press('k', 0, "k"), press('k', 0, "k"), press('0', 0, "0"), press('l', 0, "l"), press('l', 0, "l"), press('v', 0, "v"),
		press('j', 0, "j"), press('j', 0, "j"), press('l', 0, "l"), press('l', 0, "l"), press('l', 0, "l"),
	} {
		m.Update(k)
	}
}

func TestSelectionGolden80x24(t *testing.T)  { goldenCopy(t, 80, 24, selecting) }
func TestSelectionGolden120x40(t *testing.T) { goldenCopy(t, 120, 40, selecting) }

func TestYankCopiesAndLeaves(t *testing.T) {
	m, r, history := inCopyMode(t, 80, 24)
	log := &clipboardLog{}
	m.copy = nil
	m.setClip = log.set
	m.Update(ctrlB)
	m.Update(press('[', 0, "["))
	serve(m, r, history)
	selecting(m, r, history)
	cmd := m.Update(press('y', 0, "y"))
	if want := "ild step 278: compiling package\nbuild step 279: compiling package\nbuild"; len(log.texts) != 1 || log.texts[0] != want {
		t.Fatalf("copied %q", log.texts)
	}
	if m.copy != nil {
		t.Fatal("y should leave copy mode")
	}
	if got := statusLine(m); !strings.Contains(got, "Copied 71 characters") {
		t.Fatalf("status %q, want the copy count", got)
	}
	if cmd == nil {
		t.Fatal("no timer to clear the note")
	}
	m.Update(cmd())
	if got := statusLine(m); strings.Contains(got, "Copied") {
		t.Fatalf("status %q still shows the note after its timer", got)
	}
	// The pane gets keys again.
	r.sent = nil
	m.Update(press('a', 0, "a"))
	if got := r.keys(); len(got) != 1 || got[0] != "a" {
		t.Fatalf("after the copy the pane got %q, want [a]", got)
	}
}

func TestMouseDragInCopyMode(t *testing.T) {
	m, r, history := inCopyMode(t, 80, 24)
	log := &clipboardLog{}
	m.setClip = log.set
	m.copy = nil
	m.Update(ctrlB)
	m.Update(press('[', 0, "["))
	serve(m, r, history)
	m.Update(tea.MouseClickMsg{X: 6, Y: 2, Button: tea.MouseLeft})
	m.Update(tea.MouseMotionMsg{X: 10, Y: 3, Button: tea.MouseLeft})
	m.Update(tea.MouseReleaseMsg{X: 10, Y: 3, Button: tea.MouseLeft})
	if m.copy != nil || len(log.texts) != 1 || log.texts[0] != "step 279: compiling package\nbuild step" {
		t.Fatalf("copy mode open=%v, copied %q", m.copy != nil, log.texts)
	}
}

// TestClickDoesNotOpenCopyMode: a click focuses a pane (#54); only a drag or
// the wheel opens copy mode.
func TestClickDoesNotOpenCopyMode(t *testing.T) {
	m, _ := newPrefixScreen(t)
	m.Update(tea.MouseClickMsg{X: 1, Y: 1, Button: tea.MouseLeft})
	m.Update(tea.MouseReleaseMsg{X: 1, Y: 1, Button: tea.MouseLeft})
	if m.copy != nil {
		t.Fatal("a click opened copy mode")
	}
}
