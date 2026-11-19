package usage

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/ui"
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
	left := lipgloss.NewStyle().Bold(true).Render(" Usage")
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

// lines builds the body rows: totals, models, turns, then notes.
func (m *Model) lines() []string {
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	switch {
	case !m.loaded:
		return []string{" " + muted.Render("Reading the conversation...")}
	case m.st.Conversation == "" && m.st.Error == "":
		return []string{" " + muted.Render("No conversation yet. Send a message in a chat pane.")}
	case m.st.Turns == 0 && m.st.Error == "":
		return []string{" " + muted.Render("No turns yet. Send a message in the chat.")}
	}
	var out []string
	if m.st.Error != "" {
		style := lipgloss.NewStyle().Foreground(m.th.ErrorFG).Background(m.th.Error)
		out = append(out, style.Render(" "+ui.PlainLine(m.st.Error)))
	}
	out = append(out, m.totalLines()...)
	out = append(out, m.modelLines()...)
	out = append(out, m.turnLines()...)
	out = append(out, m.noteLines()...)
	return out
}

// totalLines are the conversation's totals, each category on its own row.
func (m *Model) totalLines() []string {
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	bold := lipgloss.NewStyle().Bold(true)
	field := func(label, value string) string {
		return "  " + muted.Render(fmt.Sprintf("%-9s", label)) + " " + value
	}
	turns := fmt.Sprintf("%d", m.st.Turns)
	if m.st.Unknown > 0 {
		turns += fmt.Sprintf(" (%d unknown)", m.st.Unknown)
	}
	if m.st.Incomplete > 0 {
		turns += fmt.Sprintf(" (%d partial)", m.st.Incomplete)
	}
	t := m.st.Totals
	out := []string{
		bold.Render(" Total"),
		field("turns", turns),
		field("tokens", fmt.Sprintf("in %s  out %s", ui.ShortCount(t.Input), ui.ShortCount(t.Output))),
		field("cache", fmt.Sprintf("read %s  write %s", ui.ShortCount(t.CacheRead), ui.ShortCount(t.CacheWrite))),
		field("cost", m.costText()),
		field("allowance", m.allowanceText()),
	}
	return out
}

// costText names the estimate and what it is not.
func (m *Model) costText() string {
	t := m.st.Totals
	switch {
	case t.Priced && t.Cumulative:
		return ui.Dollars(t.Cost) + " API est. (session)"
	case t.Priced:
		return ui.Dollars(t.Cost) + " est. (list prices)"
	case t.Input+t.Output+t.CacheRead+t.CacheWrite > 0:
		return "unknown: no price for this model"
	}
	return "unavailable"
}

// allowanceText says what Snow knows about the account's remaining use. It
// never derives an allowance from an API-equivalent price.
func (m *Model) allowanceText() string {
	if m.st.Runtime != "" {
		return "unavailable: this runtime reports no allowance"
	}
	return "none: API keys bill per token"
}

// modelLines is the per-model breakdown, biggest first.
func (m *Model) modelLines() []string {
	if len(m.st.Models) == 0 {
		return nil
	}
	bold := lipgloss.NewStyle().Bold(true)
	out := []string{"", bold.Render(" Models")}
	for _, model := range m.st.Models {
		line := fmt.Sprintf("  %-16s in %s  out %s", short(model.Model, 16),
			ui.ShortCount(model.Input), ui.ShortCount(model.Output))
		if model.Priced {
			line += "  " + ui.Dollars(model.Cost)
		}
		out = append(out, line)
	}
	return out
}

// turnLines is the per-turn list, newest first.
func (m *Model) turnLines() []string {
	if len(m.st.Rows) == 0 {
		return nil
	}
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	bold := lipgloss.NewStyle().Bold(true)
	out := []string{"", bold.Render(" Turns, newest first")}
	for _, r := range m.st.Rows {
		at := r.At.Format("01-02 15:04")
		model := short(r.Model, 16)
		var detail string
		switch {
		case r.Unknown && r.Interrupted:
			detail = "unknown, stopped"
		case r.Unknown:
			detail = "unknown"
		default:
			detail = fmt.Sprintf("in %s  out %s", ui.ShortCount(r.Input), ui.ShortCount(r.Output))
			if r.Priced {
				detail += "  " + ui.Dollars(r.Cost)
			}
			if r.Incomplete {
				detail += "  partial"
			}
			if r.Interrupted {
				detail += "  stopped"
			}
		}
		line := fmt.Sprintf("  #%-3d %s  %-16s %s", r.Turn, at, model, detail)
		if r.Unknown {
			line = muted.Render(line)
		}
		out = append(out, line)
	}
	if m.st.Earlier > 0 {
		out = append(out, "  "+muted.Render(fmt.Sprintf("%d earlier turns not shown", m.st.Earlier)))
	}
	return out
}

// noteLines explains the limits of the data.
func (m *Model) noteLines() []string {
	if len(m.st.Notes) == 0 {
		return nil
	}
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	bold := lipgloss.NewStyle().Bold(true)
	out := []string{"", bold.Render(" Notes")}
	for _, note := range m.st.Notes {
		for _, line := range ui.WrapText(ui.PlainLine(note), max(m.Width-2, 1)) {
			out = append(out, "  "+muted.Render(line))
		}
	}
	return out
}

// short cuts s to n cells for a column, keeping the file name's end.
func short(s string, n int) string {
	s = ui.PlainLine(s)
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		s = s[i+1:]
	}
	if ansi.StringWidth(s) <= n {
		return s
	}
	return ansi.Truncate(s, max(n-1, 0), "") + "…"
}
