// Package newsession is the dialog that sets a new session's folder,
// starting program, and layout.
package newsession

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/runtimepick"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

const (
	maxDialogWidth = 88
	minDialogWidth = 28

	folderLabelRow = 0
	folderInputRow = 1
	startLabelRow  = 3
	startChoiceRow = 4
	layoutLabelRow = 6
	layoutFirstRow = 7
	hintsRow       = 12
)

const (
	// StartShell selects a terminal pane for the new session.
	StartShell = "shell"
	// StartClaudeCode selects a ready Claude Code connection for the new session.
	StartClaudeCode = "claude-code"
)

type field int

const (
	folderField field = iota
	startField
	layoutField
)

var hints = []ui.Hint{{Key: "Tab", Desc: "next row"}, {Key: "Enter", Desc: "start"}, {Key: "Esc", Desc: "cancel"}}

var layoutRows = func() []ui.ListItem {
	rows := []ui.ListItem{{Cols: []string{"No layout"}}}
	for _, preset := range api.LayoutPresets {
		rows = append(rows, ui.ListItem{Cols: []string{preset.Name, preset.Desc}})
	}
	return rows
}()

// Choice is the folder and layout selected for the new session. An empty
// layout means no preset.
type Choice struct {
	Folder     string
	Start      string
	Connection string
	Layout     string
}

// Model is one open new-session dialog.
type Model struct {
	folder   string
	start    int
	layout   int
	focus    field
	runtimes []api.RuntimeConnection
	rows     int
}

// New opens the dialog with folder prefilled.
func New(folder string) *Model { return &Model{folder: folder} }

// SetRuntimes replaces the connections used to decide whether Claude Code
// can be offered in this dialog.
func (m *Model) SetRuntimes(list []api.RuntimeConnection) {
	m.runtimes = append([]api.RuntimeConnection(nil), list...)
	if m.start >= len(m.startItems()) {
		m.start = 0
	}
}

// SetSize records the area available to the dialog.
func (m *Model) SetSize(_ int, rows int) {
	m.rows = rows
	if m.start >= len(m.startItems()) {
		m.start = 0
	}
}

func (m *Model) startItems() []ui.ListItem {
	items := []ui.ListItem{{Cols: []string{"Plain shell"}}}
	if m.rows > 0 && m.rows < 16 {
		return items
	}
	for _, connection := range m.runtimes {
		if runtimepick.Ready(connection) {
			return append(items, ui.ListItem{Cols: []string{"Claude Code"}})
		}
	}
	return items
}

// Key handles a key press in the dialog.
func (m *Model) Key(k tea.Key) picker.Result {
	switch {
	case k.Code == tea.KeyEscape:
		return picker.Cancelled
	case k.Code == tea.KeyEnter:
		return picker.Accepted
	case k.Code == tea.KeyTab && k.Mod.Contains(tea.ModShift):
		m.focus = field(picker.Move(int(m.focus), -1, 3))
		return picker.Open
	case k.Code == tea.KeyTab:
		m.focus = field(picker.Move(int(m.focus), 1, 3))
		return picker.Open
	case k.Code == tea.KeyUp || k.Code == tea.KeyDown:
		delta := 1
		if k.Code == tea.KeyUp {
			delta = -1
		}
		switch m.focus {
		case startField:
			m.start = picker.Move(m.start, delta, len(m.startItems()))
		case layoutField:
			m.layout = picker.Move(m.layout, delta, len(layoutRows))
		}
		return picker.Open
	default:
		if m.focus == folderField {
			if folder, ok := keys.Edit(m.folder, k); ok {
				m.folder = folder
			}
		}
		return picker.Open
	}
}

// Paste adds pasted text to the folder while that row is focused.
func (m *Model) Paste(text string) {
	if m.focus == folderField {
		m.folder += text
	}
}

// Click handles a mouse click on a screen of cols by rows cells.
func (m *Model) Click(th theme.Theme, cols, rows, x, y int) picker.Result {
	box, left, top := m.View(th, cols, rows)
	w, h := ui.Size(box)
	if box == "" || x < left || x >= left+w || y < top || y >= top+h {
		return picker.Cancelled
	}
	if x == left || x == left+w-1 || y == top || y == top+h-1 {
		return picker.Open
	}
	row := y - top - 1
	switch {
	case row == folderLabelRow || row == folderInputRow:
		m.focus = folderField
	case row == startLabelRow:
		m.focus = startField
	case row >= startChoiceRow && row < startChoiceRow+len(m.startItems()):
		m.focus, m.start = startField, row-startChoiceRow
	case row == m.layoutLabelRow():
		m.focus = layoutField
	case row >= m.layoutFirstRow() && row < m.layoutFirstRow()+len(layoutRows):
		m.focus, m.layout = layoutField, row-m.layoutFirstRow()
	case row == m.hintsRow():
		contentX := x - left - 1
		for _, span := range ui.HintSpans(hints, w-2) {
			if contentX < span.X || contentX >= span.X+span.W {
				continue
			}
			switch span.Index {
			case 0:
				m.focus = field(picker.Move(int(m.focus), 1, 3))
			case 1:
				return picker.Accepted
			case 2:
				return picker.Cancelled
			}
		}
	}
	return picker.Open
}

func (m *Model) layoutFirstRow() int { return layoutFirstRow + len(m.startItems()) - 1 }

func (m *Model) layoutLabelRow() int { return layoutLabelRow + len(m.startItems()) - 1 }

func (m *Model) hintsRow() int { return hintsRow + len(m.startItems()) - 1 }

// View draws the dialog centered on a screen of cols by rows cells.
func (m *Model) View(th theme.Theme, cols, rows int) (box string, x, y int) {
	w := min(maxDialogWidth, cols-4)
	if w < minDialogWidth || rows < 14 {
		return "", 0, 0
	}
	inner := w - 2
	body := []string{
		section(th, "Folder", m.focus == folderField),
		ui.Input(th, ui.InputOptions{Value: m.folder, Width: inner, Focused: m.focus == folderField}),
		"",
		section(th, "Start with", m.focus == startField),
		ui.List(th, ui.ListOptions{Items: m.startItems(), Selected: m.start, Width: inner}),
		"",
		section(th, "Layout", m.focus == layoutField),
		ui.List(th, ui.ListOptions{Items: layoutRows, Selected: m.layout, Width: inner}),
		"",
		ui.KeyHints(th, hints, inner),
	}
	box = ui.Panel(th, ui.PanelOptions{Title: "New session", Body: strings.Join(body, "\n"), Width: w})
	boxW, boxH := ui.Size(box)
	if boxW > cols || boxH > rows {
		return "", 0, 0
	}
	return box, max((cols-boxW)/2, 0), max((rows-boxH)/2, 0)
}

// Choice returns the folder and selected layout.
func (m *Model) Choice() Choice {
	choice := Choice{Folder: m.folder, Start: StartShell}
	if m.start == 1 && len(m.startItems()) > 1 {
		choice.Start = StartClaudeCode
		for _, connection := range m.runtimes {
			if runtimepick.Ready(connection) {
				choice.Connection = connection.ID
				break
			}
		}
	}
	if m.layout > 0 && m.layout <= len(api.LayoutPresets) {
		choice.Layout = api.LayoutPresets[m.layout-1].Name
	}
	return choice
}

func section(th theme.Theme, text string, focused bool) string {
	color := th.Muted
	if focused {
		color = th.Accent
	}
	return lipgloss.NewStyle().Foreground(color).Render(text)
}
