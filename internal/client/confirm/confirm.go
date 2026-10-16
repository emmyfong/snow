// Package confirm is the confirmation popup. It asks before a destructive
// action, such as closing a pane or a window, and the mouse can answer it:
// it has clickable Close and Cancel buttons. The keys y, n, and Escape work
// too, so keys and mouse share one confirmation.
package confirm

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
)

// Result is what a key press or a click did to the popup.
type Result int

// Results.
const (
	Open Result = iota // the popup stays open
	Yes                // the user confirmed
	No                 // the user cancelled
)

// Model is one open confirmation.
type Model struct {
	title, question, yes string
	focus                int // 0 is the confirm button, 1 is Cancel
}

const cancelLabel = "Cancel"

// New asks question under title. Yes labels the confirm button, for example
// "Close". Cancel has the focus, so Enter never destroys anything.
func New(title, question, yes string) *Model {
	return &Model{title: title, question: question, yes: yes, focus: 1}
}

// Key handles one key press: y confirms, n and Escape cancel, arrows, Tab,
// h, and l move between the buttons, and Enter presses the focused one.
func (m *Model) Key(k tea.Key) Result {
	switch {
	case k.Text == "y" || k.Text == "Y":
		return Yes
	case k.Text == "n" || k.Text == "N" || k.Code == tea.KeyEscape:
		return No
	case k.Code == tea.KeyEnter:
		if m.focus == 0 {
			return Yes
		}
		return No
	case k.Code == tea.KeyTab || k.Code == tea.KeyLeft || k.Code == tea.KeyRight || k.Text == "h" || k.Text == "l":
		m.focus = 1 - m.focus
	}
	return Open
}

// layout holds where the popup and its buttons are.
type layout struct {
	box        string
	x, y       int
	w, h       int
	buttonRow  int // screen row of the buttons
	yesX, yesW int // screen columns of the confirm button
	noX, noW   int
}

func (m *Model) layout(th theme.Theme, cols, rows int) (layout, bool) {
	yes, no := "[ "+m.yes+" ]", "[ "+cancelLabel+" ]"
	const gap = "  "
	pointer := ansi.StringWidth(th.Glyphs.Pointer)
	buttonsW := ansi.StringWidth(yes + gap + no)
	// Frame, pointer, and one cell of margin around the text.
	chrome := 2 + pointer + 1
	textW := min(max(buttonsW, min(ansi.StringWidth(m.question), 44)), cols-chrome)
	if textW < buttonsW || rows < 5 {
		return layout{}, false
	}
	lines := strings.Split(ansi.Wrap(m.question, textW, ""), "\n")
	lines = lines[:min(len(lines), rows-4)]
	focused := lipgloss.NewStyle().Background(th.Selection).Bold(true)
	styled := func(s string, on bool) string {
		if on {
			return focused.Render(s)
		}
		return s
	}
	items := append(append([]string{}, lines...), "", styled(yes, m.focus == 0)+gap+styled(no, m.focus == 1))
	width := textW + chrome
	box := ui.Popup(th, ui.PopupOptions{Title: m.title, Items: items, Selected: -1, Width: width})
	l := layout{box: box, w: width, h: len(items) + 2}
	l.x, l.y = max((cols-l.w)/2, 0), max((rows-l.h)/2, 0)
	l.buttonRow = l.y + 1 + len(items) - 1
	l.yesX, l.yesW = l.x+1+pointer, ansi.StringWidth(yes)
	l.noX, l.noW = l.yesX+l.yesW+ansi.StringWidth(gap), ansi.StringWidth(no)
	return l, true
}

// View draws the popup centered on a screen of cols by rows cells and
// returns the box with the cell of its top-left corner. It returns "" when
// the screen is too small; the keys still answer.
func (m *Model) View(th theme.Theme, cols, rows int) (box string, x, y int) {
	l, ok := m.layout(th, cols, rows)
	if !ok {
		return "", 0, 0
	}
	return l.box, l.x, l.y
}

// Click handles a left click at screen cell x, y. A click on a button
// answers; a click elsewhere in the popup does nothing; a click outside
// cancels, because that is the safe answer.
func (m *Model) Click(th theme.Theme, cols, rows, x, y int) Result {
	l, ok := m.layout(th, cols, rows)
	if !ok || x < l.x || x >= l.x+l.w || y < l.y || y >= l.y+l.h {
		return No
	}
	if y == l.buttonRow {
		switch {
		case x >= l.yesX && x < l.yesX+l.yesW:
			return Yes
		case x >= l.noX && x < l.noX+l.noW:
			return No
		}
	}
	return Open
}
