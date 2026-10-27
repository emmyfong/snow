package authpick

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

const (
	minWidth    = 32
	maxWidth    = 64
	maxListRows = 6
	maxNoteRows = 5
)

// layout is where the picker goes and what it shows. listTop and listRows
// are screen cells, for mapping a click to a provider.
type layout struct {
	box               string
	x, y, w, h        int
	listTop, listRows int
	listItems         []int // provider index per list row
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

// Click handles a left click. A provider opens the key field; a click
// outside closes the picker.
func (m *Model) Click(th theme.Theme, cols, rows, x, y int) Result {
	l, ok := m.layout(th, cols, rows)
	if !ok || x < l.x || x >= l.x+l.w || y < l.y || y >= l.y+l.h {
		return Cancelled
	}
	if m.stage == stageList && y >= l.listTop && y < l.listTop+l.listRows {
		i := y - l.listTop
		if i >= 0 && i < len(l.listItems) {
			m.selected = l.listItems[i]
			m.startKey()
		}
	}
	return Open
}

func (m *Model) layout(th theme.Theme, cols, rows int) (layout, bool) {
	w := min(maxWidth, cols-4)
	if w < minWidth {
		return layout{}, false
	}
	inner := w - 2
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	errStyle := lipgloss.NewStyle().Foreground(th.Error)
	var body []string
	l := layout{w: w}
	add := func(text string) { body = append(body, " "+text) }
	addLines := func(lines []string) {
		for _, line := range lines {
			add(line)
		}
	}
	wrapMuted := func(text string, limit int) []string {
		lines := wrap(text, inner-1, limit)
		for i := range lines {
			lines[i] = muted.Render(lines[i])
		}
		return lines
	}

	switch m.stage {
	case stageList:
		list, items := m.providerList(th, inner-1, maxListRows)
		l.listItems = items
		if list == "" {
			text := "No providers."
			if !m.ready {
				text = "Loading providers."
			}
			add(muted.Render(text))
		} else {
			for _, line := range strings.Split(list, "\n") {
				add(line)
			}
		}
		l.listRows = len(body)
		if m.note != "" {
			body = append(body, "")
			addLines(wrapMuted(m.note, maxNoteRows))
		}
		body = append(body, "")
		add(ui.KeyHints(th, listHints, inner-1))
	case stageKey, stageChecking:
		p, ok := m.Provider()
		if !ok {
			return layout{}, false
		}
		title := providerTitle(p)
		addLines(wrap("Paste the API key for "+title+". Snow checks it before it saves it.", inner-1, 3))
		var extra []string
		if p.KeyURL != "" {
			extra = append(extra, wrapMuted("Create a key at "+p.KeyURL+".", 2)...)
		}
		if p.Source != "" {
			extra = append(extra, wrapMuted("A key is already saved from "+p.Source+".", 2)...)
		}
		if p.EnvVar != "" {
			extra = append(extra, wrapMuted("Snow uses "+p.EnvVar+" first.", 2)...)
		}
		if p.Err != "" {
			extra = append(extra, wrapMuted("The saved key cannot be read: "+p.Err, 2)...)
		}
		body = append(body, "")
		addLines(extra)
		if len(extra) > 0 {
			body = append(body, "")
		}
		body = append(body, " "+ui.Input(th, ui.InputOptions{
			Value: string(m.key), Prompt: "key ", Width: inner - 1, Focused: m.stage == stageKey, Mask: true,
		}))
		switch {
		case m.stage == stageChecking:
			body = append(body, "")
			add(muted.Render("Checking the key with " + title + "..."))
		case m.note != "":
			body = append(body, "")
			for _, line := range wrap(m.note, inner-1, maxNoteRows) {
				add(errStyle.Render(line))
			}
		}
		body = append(body, "")
		add(ui.KeyHints(th, m.hints(), inner-1))
	}

	h := len(body) + 2
	if h > rows {
		return layout{}, false
	}
	l.h = h
	l.x, l.y = max((cols-w)/2, 0), max((rows-h)/2, 0)
	l.listTop = l.y + 1
	l.box = ui.Panel(th, ui.PanelOptions{Title: m.title(), Body: strings.Join(body, "\n"), Width: w})
	return l, true
}

// providerList draws the providers with their key state, and maps each row
// to its provider.
func (m *Model) providerList(th theme.Theme, width, height int) (string, []int) {
	if len(m.providers) == 0 {
		return "", nil
	}
	items := make([]ui.ListItem, len(m.providers))
	for i, p := range m.providers {
		items[i] = ui.ListItem{Cols: []string{providerTitle(p), status(p)}}
	}
	return ui.ListRows(th, ui.ListOptions{Items: items, Selected: m.selected, Width: width, Height: height})
}

// title is the box's top edge.
func (m *Model) title() string {
	if m.stage == stageList {
		return "Log in"
	}
	p, ok := m.Provider()
	if !ok {
		return "Log in"
	}
	return "Log in · " + providerTitle(p)
}

// hints are the keys of the current stage.
func (m *Model) hints() []ui.Hint {
	if m.stage == stageChecking {
		return checkingHints
	}
	return keyHints
}

// wrap wraps text to width and cuts it to at most limit rows. A cut note
// ends in an ellipsis.
func wrap(text string, width, limit int) []string {
	if text == "" || width <= 0 || limit <= 0 {
		return nil
	}
	var out []string
	for _, para := range strings.Split(text, "\n") {
		if para == "" {
			out = append(out, "")
			continue
		}
		out = append(out, strings.Split(ansi.Wrap(para, width, ""), "\n")...)
	}
	if len(out) > limit {
		last := ansi.Truncate(out[limit-1], max(width-1, 0), "") + "…"
		out = append(out[:limit-1], last)
	}
	return out
}

// providerTitle names a provider in words.
func providerTitle(p api.ProviderAuth) string {
	if p.Title != "" {
		return p.Title
	}
	return p.Name
}

// status says where a provider's key comes from, or why it has none.
func status(p api.ProviderAuth) string {
	switch {
	case p.NoKey:
		return "no key needed"
	case p.Err != "":
		return "error"
	case p.Source != "":
		return p.Source
	case p.Connected:
		return "connected"
	default:
		return "no key"
	}
}

var (
	listHints     = []ui.Hint{{Key: "↑↓", Desc: "move"}, {Key: "enter", Desc: "choose"}, {Key: "esc", Desc: "close"}}
	keyHints      = []ui.Hint{{Key: "enter", Desc: "check"}, {Key: "esc", Desc: "back"}}
	checkingHints = []ui.Hint{{Key: "esc", Desc: "close"}}
)
