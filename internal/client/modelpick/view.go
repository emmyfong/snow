package modelpick

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

const (
	maxWidth    = 76
	minWidth    = 32
	maxListRows = 12
)

// layout is where the picker goes and what it shows. The row and column
// fields are screen cells, for mapping a click.
type layout struct {
	box               string
	x, y, w, h        int
	listTop, listRows int
	leftX, leftW      int
	leftFirst         int
	rightX, rightW    int
	rightFirst        int
	effortTop         int
	chips             []chip
}

// chip is one effort level on a row of the picker.
type chip struct {
	line, x, w, index int
	enabled           bool
}

// View draws the picker centered on a window of cols by rows cells and
// returns the box with the cell of its top-left corner. It returns "" when
// the window is too small; the keys still answer.
func (m *Model) View(th theme.Theme, cols, rows int) (box string, x, y int) {
	l, ok := m.layout(th, cols, rows)
	if !ok {
		return "", 0, 0
	}
	return l.box, l.x, l.y
}

// Click handles a left click. A provider changes the models shown, a model
// chooses it, and an effort level selects it. A click outside closes the
// picker.
func (m *Model) Click(th theme.Theme, cols, rows, x, y int) picker.Result {
	l, ok := m.layout(th, cols, rows)
	if !ok || x < l.x || x >= l.x+l.w || y < l.y || y >= l.y+l.h {
		return picker.Cancelled
	}
	if y >= l.listTop && y < l.listTop+l.listRows {
		i := y - l.listTop
		switch {
		case x >= l.leftX && x < l.leftX+l.leftW:
			if j := l.leftFirst + i; j < len(m.providers) {
				m.setProvider(j)
				m.focus = focusProviders
			}
		case x >= l.rightX && x < l.rightX+l.rightW:
			m.focus = focusModels
			if j := l.rightFirst + i; j < len(m.shown()) {
				m.setModel(j)
				return m.accept(false)
			}
		}
		return picker.Open
	}
	for _, c := range l.chips {
		if c.enabled && y == l.effortTop+c.line && x >= l.x+1+c.x && x < l.x+1+c.x+c.w {
			m.focus, m.level, m.touched = focusEffort, c.index, true
			return picker.Open
		}
	}
	return picker.Open
}

func (m *Model) layout(th theme.Theme, cols, rows int) (layout, bool) {
	w := min(maxWidth, cols-4)
	if w < minWidth {
		return layout{}, false
	}
	inner := w - 2
	leftW := min(max(inner/3, 16), 26)
	// The leading and trailing cell keep the rows off the frame.
	rightW := inner - leftW - 3
	if rightW < 10 {
		return layout{}, false
	}
	efforts, chips := m.effortLines(th, inner)
	want := max(len(m.providers), len(m.shown()), 3)
	n := min(max(rows-5-len(efforts), 1), want, maxListRows)
	h := n + 5 + len(efforts)
	if h > rows {
		return layout{}, false
	}
	x, y := max((cols-w)/2, 0), max((rows-h)/2, 0)
	l := layout{
		w: w, h: h, x: x, y: y,
		listTop: y + 1, listRows: n,
		leftX: x + 2, leftW: leftW,
		rightX: x + leftW + 4, rightW: rightW,
		effortTop: y + 1 + n + 1,
		chips:     chips,
	}

	left, leftFirst := m.column(th, m.providerItems(), m.provider, focusProviders, leftW, n, m.providerNote())
	right, rightFirst := m.column(th, m.modelItems(), m.model, focusModels, rightW, n, m.modelNote())
	l.leftFirst, l.rightFirst = leftFirst, rightFirst
	leftLines, rightLines := strings.Split(left, "\n"), strings.Split(right, "\n")
	body := make([]string, 0, n+3+len(efforts))
	for i := 0; i < n; i++ {
		body = append(body, " "+pad(line(leftLines, i), leftW)+"  "+pad(line(rightLines, i), rightW))
	}
	body = append(body, "")
	body = append(body, efforts...)
	body = append(body, m.statusLine(th, inner-1))
	body = append(body, " "+ui.KeyHints(th, hints, inner-1))
	l.box = ui.Panel(th, ui.PanelOptions{Title: "Model", Body: strings.Join(body, "\n"), Width: w})
	return l, true
}

// column draws one list of the picker, or its note when it has no rows.
func (m *Model) column(th theme.Theme, items []ui.ListItem, selected int, focused focus, width, height int, note string) (string, int) {
	sel := -1
	if m.focus == focused {
		sel = selected
	}
	first, _ := ui.ListWindow(len(items), sel, height)
	if len(items) == 0 {
		line := " " + lipgloss.NewStyle().Foreground(th.Muted).Render(note)
		return pad(line, width), first
	}
	return ui.List(th, ui.ListOptions{Items: items, Selected: sel, Width: width, Height: height}), first
}

// line returns line i of s, or "".
func line(s []string, i int) string {
	if i < 0 || i >= len(s) {
		return ""
	}
	return s[i]
}

// pad cuts s to width cells and pads it to exactly width.
func pad(s string, width int) string {
	s = ansi.Truncate(s, width, "")
	return s + strings.Repeat(" ", max(width-ansi.StringWidth(s), 0))
}

// hints are the picker's keys, most important first so a narrow picker
// keeps the ones a user needs.
var hints = []ui.Hint{
	{Key: "↑↓", Desc: "move"},
	{Key: "enter", Desc: "use"},
	{Key: "esc", Desc: "close"},
	{Key: "tab", Desc: "column"},
	{Key: "ctrl+d", Desc: "save"},
}

// statusLine is the filter, or the note when the picker has one.
func (m *Model) statusLine(th theme.Theme, width int) string {
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	if m.note != "" {
		return pad(" "+lipgloss.NewStyle().Foreground(th.Error).Render(m.note), width)
	}
	if len(m.filter) == 0 {
		return pad(" "+muted.Render("filter")+"  "+muted.Render("type to filter"), width)
	}
	return pad(" "+muted.Render("filter")+"  "+string(m.filter), width)
}

// providerItems lists the providers with their connection state.
func (m *Model) providerItems() []ui.ListItem {
	items := make([]ui.ListItem, len(m.providers))
	for i, p := range m.providers {
		title := p.Title
		if title == "" {
			title = p.Name
		}
		// The parenthetical part of a provider's title repeats what the
		// model list shows, and the status needs the room.
		if base, _, ok := strings.Cut(title, " ("); ok {
			title = base
		}
		status := ""
		switch {
		case p.Refreshing:
			status = "loading"
		case !p.Connected:
			status = "no key"
		case p.Err != "":
			status = "error"
		}
		if status != "" {
			title += " (" + status + ")"
		}
		items[i] = ui.ListItem{Cols: []string{title}}
	}
	return items
}

// modelItems lists the selected provider's models that match the filter.
func (m *Model) modelItems() []ui.ListItem {
	shown := m.shown()
	items := make([]ui.ListItem, len(shown))
	for i, mi := range shown {
		name := mi.DisplayName
		if name == "" {
			name = mi.ID
		}
		cols := []string{name, "", ""}
		if mi.ContextWindow > 0 {
			cols[1] = ui.ShortCount(mi.ContextWindow)
		}
		if m.providerName()+"/"+mi.ID == m.current {
			cols[2] = "current"
		}
		items[i] = ui.ListItem{Cols: cols}
	}
	return items
}

// providerNote is what the provider column says while it has no rows.
func (m *Model) providerNote() string {
	if len(m.providers) == 0 {
		return "No providers."
	}
	return ""
}

// modelNote is what the model column says while it has no rows.
func (m *Model) modelNote() string {
	if len(m.providers) == 0 {
		return "No providers."
	}
	p := m.providers[m.provider]
	switch {
	case p.Refreshing && len(p.Models) == 0:
		return "Loading models."
	case len(p.Models) == 0 && !p.Connected:
		return "No key."
	case len(p.Models) == 0 && p.Err != "":
		return p.Err
	case len(p.Models) == 0:
		return "No models."
	case len(m.shown()) == 0:
		return "No match."
	}
	return ""
}

// effortLabel names effort level i; level 0 is the provider's default.
func effortLabel(i int) string {
	if i == 0 {
		return "default"
	}
	return api.EffortLevels[i-1]
}

// effortLines draws the effort levels, wrapped to width. A level the model
// does not take is muted. The current level is bracketed, and filled when
// the effort row has the keyboard.
func (m *Model) effortLines(th theme.Theme, width int) ([]string, []chip) {
	const gap = "  "
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	selected := lipgloss.NewStyle().Foreground(th.Accent).Bold(true)
	if m.focus == focusEffort {
		selected = lipgloss.NewStyle().Foreground(th.SelectionFG).Background(th.Selection).Bold(true)
	}
	prefix := " " + muted.Render("effort") + gap
	prefixW := ansi.StringWidth(prefix)
	indent := strings.Repeat(" ", prefixW)
	var lines []string
	var chips []chip
	line, count := prefix, 0
	for i := 0; i <= len(api.EffortLevels); i++ {
		enabled := m.levelEnabled(i)
		text := effortLabel(i)
		shown := text
		switch {
		case i == m.level:
			shown = selected.Render("[ " + text + " ]")
		case !enabled:
			shown = muted.Render(text)
		}
		w := ansi.StringWidth(shown)
		gapW := 0
		if count > 0 {
			gapW = len(gap)
		}
		if count > 0 && ansi.StringWidth(line)+gapW+w > width {
			lines = append(lines, line)
			line, count, gapW = indent, 0, 0
		}
		x := ansi.StringWidth(line) + gapW
		if count > 0 {
			line += gap
		}
		line += shown
		chips = append(chips, chip{line: len(lines), x: x, w: w, index: i, enabled: enabled})
		count++
	}
	return append(lines, line), chips
}
