package contextpane

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// View returns the pane's rows, each at most its width.
func (m *Model) View() []string {
	if m.Width <= 0 || m.Height <= 0 {
		return nil
	}
	rows := []string{m.header()}
	body := m.lines()
	first := min(max(m.Top, 0), max(len(body)-m.BodyHeight(), 0))
	for i := first; i < len(body) && len(rows) < m.Height-1; i++ {
		rows = append(rows, body[i])
	}
	if len(rows) < m.Height-1 {
		rows = append(rows, make([]string, m.Height-1-len(rows))...)
	}
	rows = append(rows, m.hints())
	rows = rows[:min(len(rows), m.Height)]
	for i, r := range rows {
		rows[i] = ansi.Truncate(r, m.Width, "")
	}
	return rows
}

// header is the first row: the pane's name, and the conversation's title.
func (m *Model) header() string {
	left := lipgloss.NewStyle().Bold(true).Render(" Context")
	right := ""
	if m.st.Loading {
		right = lipgloss.NewStyle().Foreground(m.th.Muted).Render("reading ")
	} else if m.st.Title != "" {
		right = lipgloss.NewStyle().Foreground(m.th.Muted).Render(ui.PlainLine(m.st.Title))
	}
	if right == "" {
		return left
	}
	gap := m.Width - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 1 {
		return left
	}
	return left + strings.Repeat(" ", gap) + right
}

// hints are the keys the pane offers on its bottom row.
func (m *Model) hints() string {
	return " " + ui.KeyHints(m.th, []ui.Hint{{Key: "r", Desc: "refresh"}}, max(m.Width-1, 0))
}

// lines builds the body rows: occupancy, sources, repeated reads, notes.
func (m *Model) lines() []string {
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	switch {
	case !m.loaded:
		return []string{" " + muted.Render("Reading the conversation...")}
	case m.st.Conversation == "" && m.st.Error == "":
		return []string{" " + muted.Render("No conversation yet. Send a message in a chat pane.")}
	}
	var out []string
	if m.st.Error != "" {
		style := lipgloss.NewStyle().Foreground(m.th.ErrorFG).Background(m.th.Error)
		out = append(out, style.Render(" "+ui.PlainLine(m.st.Error)))
	}
	out = append(out, m.occupancyLines()...)
	out = append(out, m.sourceLines()...)
	out = append(out, m.readLines()...)
	out = append(out, m.noteLines()...)
	return out
}

// field renders one labelled value of a section.
func (m *Model) field(label, value string) string {
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	return "  " + muted.Render(fmt.Sprintf("%-9s", label)) + " " + value
}

// occupancyLines say how full the context window was, or why that is
// unavailable. A hidden prompt is never estimated.
func (m *Model) occupancyLines() []string {
	bold := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	o := m.st.Occupancy
	out := []string{bold.Render(" Occupancy")}
	switch {
	case o.Known && o.Window > 0:
		out = append(out, m.field("context", fmt.Sprintf("%s of %s (%d%%)",
			ui.ShortCount(o.Tokens), ui.ShortCount(o.Window), o.Tokens*100/o.Window)))
	case o.Known:
		out = append(out, m.field("context", ui.ShortCount(o.Tokens)+" (window unknown)"))
	default:
		out = append(out, m.field("context", "unavailable"))
	}
	if o.Source != "" {
		out = append(out, m.field("source", o.Source))
	}
	if !o.Known && o.Reason != "" {
		for _, line := range ui.WrapText(o.Reason, max(m.Width-2, 1)) {
			out = append(out, "  "+muted.Render(line))
		}
	}
	return out
}

// sourceLines list the files the tool calls read or changed.
func (m *Model) sourceLines() []string {
	bold := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	out := []string{"", bold.Render(" Sources")}
	if len(m.st.Sources) == 0 {
		out = append(out, "  "+muted.Render("No successful file reads or changes yet."))
		return out
	}
	out = append(out, "  "+muted.Render("per-source token counts are unavailable"))
	for _, s := range m.st.Sources {
		out = append(out, fmt.Sprintf("  %-34s %s", tail(s.Path, 34), sourceTag(s)))
	}
	return out
}

// sourceTag says how a source was used.
func sourceTag(s api.ContextSource) string {
	switch {
	case s.Changed && s.Reads > 0:
		return fmt.Sprintf("read %dx, changed", s.Reads)
	case s.Changed:
		return "changed"
	case s.Reads > 0:
		return fmt.Sprintf("read %dx", s.Reads)
	}
	return ""
}

// readLines list the files read more than once and what the results show.
func (m *Model) readLines() []string {
	bold := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	out := []string{"", bold.Render(" Reads, repeated")}
	if len(m.st.Reads) == 0 {
		out = append(out, "  "+muted.Render("No file was read more than once."))
		return out
	}
	for _, r := range m.st.Reads {
		label := r.Path
		if r.Range != "" {
			label += ":" + r.Range
		}
		out = append(out, fmt.Sprintf("  %-34s %d reads  %-9s #%d-#%d",
			tail(label, 34), r.Reads, r.Result, r.FirstTurn, r.LastTurn))
		out = append(out, "    "+muted.Render(fmt.Sprintf("%s%d same, %d changed; evidence: %s",
			readTiming(r), r.Same, r.Changed, evidence(r.Evidence))))
	}
	return out
}

// readTiming names when the first and last read happened, when the records
// carry times.
func readTiming(r api.ContextRead) string {
	if r.FirstAt.IsZero() || r.LastAt.IsZero() {
		return ""
	}
	return fmt.Sprintf("#%d %s, #%d %s; ",
		r.FirstTurn, r.FirstAt.Format("15:04"), r.LastTurn, r.LastAt.Format("15:04"))
}

// noteLines explain the limits of the data.
func (m *Model) noteLines() []string {
	if len(m.st.Notes) == 0 {
		return nil
	}
	bold := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	out := []string{"", bold.Render(" Notes")}
	for _, note := range m.st.Notes {
		for _, line := range ui.WrapText(ui.PlainLine(note), max(m.Width-2, 1)) {
			out = append(out, "  "+muted.Render(line))
		}
	}
	return out
}

// evidence names what a comparison looked at.
func evidence(s string) string {
	if s == "" {
		return "unavailable"
	}
	return s
}

// tail cuts s to at most n runes, keeping its end, so a path's file name
// stays visible.
func tail(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[len(r)-n:])
	}
	return "…" + string(r[len(r)-(n-1):])
}
