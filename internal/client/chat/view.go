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
// width: the transcript, then a row that is a divider or, during a turn,
// the activity line, then the message being typed.
func (m *Model) View() []string {
	if m.width <= 0 || m.height <= 0 {
		return nil
	}
	input := m.inputRows()
	rows := make([]string, 0, m.height)
	all := m.transcriptRows()
	area := m.transcriptHeight()
	end := max(len(all)-m.scroll, 0)
	start := max(end-area, 0)
	shown := all[start:end]
	for range area - len(shown) {
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
// message being typed.
func (m *Model) Cursor() (x, y int, ok bool) {
	if m.width <= 0 || m.height <= 0 {
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
	return max(m.height-ui.TextAreaRows(m.inputOptions())-1, 0)
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

// transcriptRows draws the whole transcript at the pane's width.
func (m *Model) transcriptRows() []string {
	out := ui.Transcript(m.th, ui.TranscriptOptions{Width: m.width, Entries: m.entries()})
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// entries turns the snapshot into what the transcript draws.
func (m *Model) entries() []ui.TranscriptEntry {
	var out []ui.TranscriptEntry
	if m.snap.Earlier > 0 {
		out = append(out, ui.TranscriptEntry{Kind: ui.EntryNote, Text: fmt.Sprintf("%d earlier entries are not shown", m.snap.Earlier)})
	}
	if len(m.snap.Entries) == 0 && len(m.snap.Queue) == 0 {
		return append(out, ui.TranscriptEntry{Kind: ui.EntryNote, Text: "No messages yet. Model: " + m.snap.Model})
	}
	for _, e := range m.snap.Entries {
		switch e.Kind {
		case api.EntryUser:
			out = append(out, ui.TranscriptEntry{Kind: ui.EntryUser, Text: e.Text})
		case api.EntryAssistant:
			out = append(out, ui.TranscriptEntry{Kind: ui.EntryAgent, Text: e.Text, Cut: e.Cut})
		case api.EntryThinking:
			out = append(out, ui.TranscriptEntry{Kind: ui.EntryThinking, Text: e.Text})
		case api.EntryTool:
			if e.Tool != nil {
				out = append(out, ui.TranscriptEntry{Kind: ui.EntryNote, Text: "tool: " + e.Tool.Name})
			}
		case api.EntryModel:
			out = append(out, ui.TranscriptEntry{Kind: ui.EntryNote, Text: "switched to " + e.Text})
		case api.EntryInterrupted:
			out = append(out, ui.TranscriptEntry{Kind: ui.EntryNote, Text: "interrupted"})
		case api.EntryError:
			msg := "failed"
			if e.Error != nil {
				msg = e.Error.Message
			}
			out = append(out, ui.TranscriptEntry{Kind: ui.EntryError, Text: msg})
		}
	}
	for _, q := range m.snap.Queue {
		out = append(out, ui.TranscriptEntry{Kind: ui.EntryQueued, Text: "queued: " + q})
	}
	return out
}
