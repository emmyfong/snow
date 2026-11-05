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
	Alt                // the user chose the second action
)

// Model is one open confirmation.
type Model struct {
	title, question string
	buttons         []string // the actions, then Cancel
	focus           int      // index in buttons; Cancel is last
	altKey          string   // the key that chooses Alt
}

const cancelLabel = "Cancel"

// New asks question under title. Yes labels the confirm button, for example
// "Close". Cancel has the focus, so Enter never destroys anything.
func New(title, question, yes string) *Model {
	return &Model{title: title, question: question, buttons: []string{yes, cancelLabel}, focus: 1}
}

// NewChoice is New with a second action, alt, between the confirm button
// and Cancel. The key altKey chooses it, as y chooses yes.
func NewChoice(title, question, yes, alt, altKey string) *Model {
	return &Model{title: title, question: question, buttons: []string{yes, alt, cancelLabel}, focus: 2, altKey: altKey}
}

// result is what pressing button i means.
func (m *Model) result(i int) Result {
	switch {
	case i == len(m.buttons)-1:
		return No
	case i == 0:
		return Yes
	}
	return Alt
}

// Key handles one key press: y confirms, the alt key chooses the second
// action, n and Escape cancel, arrows, Tab, h, and l move between the
// buttons, and Enter presses the focused one.
func (m *Model) Key(k tea.Key) Result {
	n := len(m.buttons)
	switch {
	case k.Text == "y" || k.Text == "Y":
		return Yes
	case m.altKey != "" && strings.EqualFold(k.Text, m.altKey):
		return Alt
	case k.Text == "n" || k.Text == "N" || k.Code == tea.KeyEscape:
		return No
	case k.Code == tea.KeyEnter:
		return m.result(m.focus)
	case k.Code == tea.KeyLeft || k.Text == "h" || (k.Code == tea.KeyTab && k.Mod.Contains(tea.ModShift)):
		m.focus = (m.focus + n - 1) % n
	case k.Code == tea.KeyTab || k.Code == tea.KeyRight || k.Text == "l":
		m.focus = (m.focus + 1) % n
	}
	return Open
}

// layout holds where the popup and its buttons are.
type layout struct {
	box       string
	x, y      int
	w, h      int
	buttonRow int      // screen row of the buttons
	spans     [][2]int // screen columns of each button: start, width
}

func (m *Model) layout(th theme.Theme, cols, rows int) (layout, bool) {
	const gap = "  "
	labels := make([]string, len(m.buttons))
	buttonsW := 0
	for i, b := range m.buttons {
		labels[i] = "[ " + b + " ]"
		buttonsW += ansi.StringWidth(labels[i])
	}
	buttonsW += ansi.StringWidth(gap) * (len(labels) - 1)
	pointer := ansi.StringWidth(th.Glyphs.Pointer)
	// Frame, pointer, and one cell of margin around the text.
	chrome := 2 + pointer + 1
	textW := min(max(buttonsW, min(ansi.StringWidth(m.question), 44)), cols-chrome)
	if textW < buttonsW || rows < 5 {
		return layout{}, false
	}
	lines := strings.Split(ansi.Wrap(m.question, textW, ""), "\n")
	lines = lines[:min(len(lines), rows-4)]
	focused := lipgloss.NewStyle().Foreground(th.SelectionFG).Background(th.Selection).Bold(true)
	var row strings.Builder
	for i, label := range labels {
		if i > 0 {
			row.WriteString(gap)
		}
		if i == m.focus {
			label = focused.Render(label)
		}
		row.WriteString(label)
	}
	items := append(append([]string{}, lines...), "", row.String())
	width := textW + chrome
	box := ui.Popup(th, ui.PopupOptions{Title: m.title, Items: items, Selected: -1, Width: width})
	l := layout{box: box, w: width, h: len(items) + 2}
	l.x, l.y = max((cols-l.w)/2, 0), max((rows-l.h)/2, 0)
	l.buttonRow = l.y + 1 + len(items) - 1
	x := l.x + 1 + pointer
	for _, label := range labels {
		w := ansi.StringWidth(label)
		l.spans = append(l.spans, [2]int{x, w})
		x += w + ansi.StringWidth(gap)
	}
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
		for i, s := range l.spans {
			if x >= s[0] && x < s[0]+s[1] {
				return m.result(i)
			}
		}
	}
	return Open
}
