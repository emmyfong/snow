package chat

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// maxInputRows is the most rows the message being typed takes; longer
// messages scroll inside them.
const maxInputRows = 6

const (
	prompt      = "> "
	placeholder = "Message snow. Enter sends, Shift+Enter adds a line."
)

// View draws the pane in exactly its height in rows, each at most its
// width: the top bar, the transcript, then a row that is a divider or, during a turn,
// the activity line, then the message being typed.
func (m *Model) View() []string {
	if m.width <= 0 || m.height <= 0 {
		return nil
	}
	input := m.inputRows()
	rows := make([]string, 0, m.height)
	if m.hasBar() {
		rows = append(rows, ui.TopBar(m.th, m.barOptions()))
	}
	start, n := m.visible()
	shown := m.transcriptRows()[start : start+n]
	for range m.transcriptHeight() - n {
		rows = append(rows, "")
	}
	rows = append(rows, shown...)
	if m.height > len(input) {
		rows = append(rows, m.middleRow())
	}
	rows = append(rows, input...)
	if len(rows) > m.height {
		rows = rows[len(rows)-m.height:]
	}
	return rows
}

// Cursor returns where the terminal's cursor goes in the pane: in the
// message being typed. While an approval waits, its box takes the keys
// and draws its own cursor, so there is none.
func (m *Model) Cursor() (x, y int, ok bool) {
	if m.width <= 0 || m.height <= 0 || m.snap.Pending != nil {
		return 0, 0, false
	}
	o := m.inputOptions()
	cx, cy := ui.TextAreaCursor(o)
	return cx, m.height - ui.TextAreaRows(o) + cy, true
}

func (m *Model) inputOptions() ui.TextAreaOptions {
	return ui.TextAreaOptions{
		Width: m.width, Height: min(maxInputRows, max(m.height/3, 1)), Prompt: prompt,
		Value: string(m.input), Cursor: m.cursor, Placeholder: placeholder,
	}
}

func (m *Model) inputRows() []string {
	return strings.Split(ui.TextArea(m.th, m.inputOptions()), "\n")
}

// transcriptHeight is how many rows the transcript gets.
func (m *Model) transcriptHeight() int {
	return max(m.height-ui.TextAreaRows(m.inputOptions())-1-m.barRows(), 0)
}

// barRows is the rows the top bar takes.
func (m *Model) barRows() int {
	if m.hasBar() {
		return 1
	}
	return 0
}

// middleRow is the divider between the transcript and the input, or the
// activity line while a turn runs. Scrolled up, the divider says that
// newer text is below.
func (m *Model) middleRow() string {
	if m.snap.Running {
		return ui.Activity(m.th, ui.ActivityOptions{Width: m.width, Frame: m.frame, Text: m.activity(), Hint: "Esc interrupt"})
	}
	line := strings.Repeat(m.th.Glyphs.Horizontal, m.width)
	if m.scroll > 0 {
		note := " newer text below; End returns "
		line = ansi.Truncate(strings.Repeat(m.th.Glyphs.Horizontal, 2)+note+line, m.width, "")
	}
	return lipgloss.NewStyle().Foreground(m.th.Border).Render(line)
}

// activity says what the running turn does, and for how long.
func (m *Model) activity() string {
	what := "working"
	switch m.snap.State {
	case api.ChatStateTool:
		what = "running a tool"
	case api.ChatStateApproval:
		what = "waiting for you"
	}
	if m.started.IsZero() {
		return what
	}
	return what + " " + elapsed(m.now().Sub(m.started))
}

func elapsed(d time.Duration) string {
	s := int(d.Seconds())
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	return fmt.Sprintf("%dm %02ds", s/60, s%60)
}
