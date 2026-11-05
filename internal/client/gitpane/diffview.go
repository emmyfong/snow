package gitpane

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

func (m *Model) diffLayout(l *layout) {
	d := m.st.Diff
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	side := "unstaged"
	if d.Staged {
		side = "staged"
	}
	l.add(lipgloss.NewStyle().Bold(true).Render(" "+ui.PlainLine(d.Path))+muted.Render("  "+side), hitNone, 0)
	h := m.diffHeight()
	rows := m.diffRows()
	m.scroll = min(m.scroll, m.maxScroll())
	for _, r := range rows[m.scroll:min(m.scroll+h, len(rows))] {
		l.add(r, hitNone, 0)
	}
	for len(l.rows) < 1+h {
		l.add("", hitNone, 0)
	}
	if m.height < 4 {
		return
	}
	l.hints = []hint{{ui.Hint{Key: "esc", Desc: "back"}, tea.Key{Code: tea.KeyEscape}}}
	if d.Staged {
		l.hints = append(l.hints, hint{ui.Hint{Key: "u", Desc: "unstage"}, tea.Key{Code: 'u', Text: "u"}})
	} else {
		l.hints = append(l.hints, hint{ui.Hint{Key: "s", Desc: "stage"}, tea.Key{Code: 's', Text: "s"}})
	}
	l.hints = append(l.hints,
		hint{ui.Hint{Key: "pgup", Desc: "up"}, tea.Key{Code: tea.KeyPgUp}},
		hint{ui.Hint{Key: "pgdn", Desc: "down"}, tea.Key{Code: tea.KeyPgDown}})
	hs := make([]ui.Hint, len(l.hints))
	for i, h := range l.hints {
		hs[i] = h.Hint
	}
	l.add(" "+ui.KeyHints(m.th, hs, m.width-1), hitHints, 0)
}

// diffRows are the open diff's rows, drawn whole: git already keeps only
// the lines near each change.
func (m *Model) diffRows() []string {
	d := m.st.Diff
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	switch {
	case d.Binary:
		return []string{" " + muted.Render("Binary file. Git shows no diff for it.")}
	case len(d.Lines) == 0:
		return []string{" " + muted.Render("No change to show.")}
	}
	lines := make([]ui.DiffLine, len(d.Lines))
	for i, l := range d.Lines {
		op := ui.DiffKeep
		switch l.Kind {
		case api.GitLineAdd:
			op = ui.DiffAdd
		case api.GitLineRemove:
			op = ui.DiffRemove
		}
		lines[i] = ui.DiffLine{Op: op, Text: l.Text}
	}
	rows := strings.Split(ui.DiffView(m.th, ui.DiffOptions{Width: m.width, Lines: lines, Context: len(lines)}), "\n")
	if d.Cut {
		rows = append(rows, " "+muted.Render("The rest of the diff is too long to show."))
	}
	return rows
}

func (m *Model) maxScroll() int {
	if m.st.Diff == nil {
		return 0
	}
	return max(len(m.diffRows())-m.diffHeight(), 0)
}
