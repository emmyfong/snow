package chat

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/textdiff"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

const (
	diffContext   = 3  // unchanged lines shown around a change
	maxDiffRows   = 40 // rows of a diff in an expanded cell
	maxResultRows = 12 // rows of a result in an expanded cell
)

// subjectKeys name, per built-in tool, the input field a cell shows
// after the tool's name.
var subjectKeys = map[string]string{
	"read": "path", "write": "path", "edit": "path", "list": "path",
	"run": "command", "search": "pattern",
}

// changeKeys are input fields a diff shows better than their text.
var changeKeys = []string{"old", "new", "content"}

// toolInput reads a tool call's input as top-level fields. Input that is
// not a JSON object gives none.
func toolInput(raw json.RawMessage) map[string]any {
	var in map[string]any
	if json.Unmarshal(raw, &in) != nil {
		return nil
	}
	return in
}

// diffOf returns the change a call makes: the diff the server sent, else,
// for an edit, its old and new text. Nil when unknown.
func diffOf(t *api.ChatTool) []textdiff.Line {
	if t.Diff != nil {
		return textdiff.Lines(t.Diff.Old, t.Diff.New)
	}
	if t.Name == "edit" {
		in := toolInput(t.Input)
		before, _ := in["old"].(string)
		after, _ := in["new"].(string)
		if before != "" || after != "" {
			return textdiff.Lines(before, after)
		}
	}
	return nil
}

func uiDiff(lines []textdiff.Line) []ui.DiffLine {
	out := make([]ui.DiffLine, len(lines))
	for i, l := range lines {
		out[i] = ui.DiffLine{Op: map[textdiff.Op]ui.DiffOp{textdiff.Keep: ui.DiffKeep, textdiff.Add: ui.DiffAdd, textdiff.Remove: ui.DiffRemove}[l.Op], Text: l.Text}
	}
	return out
}

// subject is what the call works on, on one row; hidden lines are
// counted, so a multi-line command never looks like its first line.
func subject(t *api.ChatTool) string {
	s, _ := toolInput(t.Input)[subjectKeys[t.Name]].(string)
	return inputValue(s)
}

// inputRows lists every field of a tool call's input, but those in skip,
// with its whole value: cleaned of escape sequences and wrapped to width.
// Input that is not a JSON object shows as its text.
func inputRows(raw json.RawMessage, skip []string, width int) []string {
	width = max(width, 1)
	in := toolInput(raw)
	if in == nil {
		return wrapRows(ui.PlainText(string(raw)), width)
	}
	keys := make([]string, 0, len(in))
	for k := range in {
		if !slices.Contains(skip, k) {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	var rows []string
	for _, k := range keys {
		v, ok := in[k].(string)
		if !ok {
			b, _ := json.Marshal(in[k])
			v = string(b)
		}
		label := ui.PlainLine(k) + ": "
		pad := strings.Repeat(" ", min(ansi.StringWidth(label), width/2))
		for i, r := range wrapRows(ui.PlainText(v), max(width-len(pad), 1)) {
			if i == 0 {
				rows = append(rows, label+r)
			} else {
				rows = append(rows, pad+r)
			}
		}
	}
	return rows
}

// wrapRows splits s into rows of at most width cells, at line breaks and
// wherever a line is too long.
func wrapRows(s string, width int) []string {
	var rows []string
	for _, l := range strings.Split(s, "\n") {
		rows = append(rows, strings.Split(ansi.Wrap(l, width, ""), "\n")...)
	}
	return rows
}

// summary says what came of a finished call.
func summary(t *api.ChatTool) string {
	if !t.Done {
		return ""
	}
	if t.IsError {
		return firstLine(t.Result)
	}
	switch t.Name {
	case "write", "edit":
		return ""
	}
	return plural(len(lines(t.Result)), "line")
}

// toolCell draws the cell of call t.
func (m *Model) toolCell(t *api.ChatTool, width int) string {
	o := ui.ToolCellOptions{
		Width: width, Name: t.Name, Subject: subject(t), Summary: summary(t),
		Selected: m.selected == t.ID, Expanded: m.expanded[t.ID],
	}
	switch {
	case !t.Done && m.snap.Pending != nil && m.snap.Pending.ID == t.ID:
		o.State = ui.ToolWaiting
	case !t.Done:
		o.State = ui.ToolRunning
	case t.IsError:
		o.State = ui.ToolFailed
	default:
		o.State = ui.ToolDone
	}
	diff := diffOf(t)
	if diff != nil {
		o.Added, o.Removed = textdiff.Counts(diff)
	}
	if o.Expanded {
		o.Details = m.toolDetails(t, diff, width-2)
	}
	return ui.ToolCell(m.th, o)
}

// toolDetails are the rows of an expanded cell: the input, the diff, and
// the result.
func (m *Model) toolDetails(t *api.ChatTool, diff []textdiff.Line, width int) []string {
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	var rows []string
	in := toolInput(t.Input)
	keys := make([]string, 0, len(in))
	for k := range in {
		if diff != nil && slices.Contains(changeKeys, k) {
			continue
		}
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		rows = append(rows, muted.Render(ui.PlainLine(k)+": "+inputValue(in[k])))
	}
	if diff != nil {
		rows = append(rows, strings.Split(ui.DiffView(m.th, ui.DiffOptions{Width: width, Lines: uiDiff(diff), Context: diffContext, MaxRows: maxDiffRows}), "\n")...)
	}
	if t.Done && t.Result != "" {
		style := lipgloss.NewStyle()
		if t.IsError {
			style = style.Foreground(m.th.Removed)
		}
		res := lines(t.Result)
		for i, l := range res {
			if i == maxResultRows-1 && len(res) > maxResultRows {
				rows = append(rows, muted.Render(fmt.Sprintf("... %d more lines", len(res)-i)))
				break
			}
			rows = append(rows, style.Render(l))
		}
	}
	return rows
}

// inputValue shows one input field on one row.
func inputValue(v any) string {
	switch v := v.(type) {
	case string:
		if n := len(lines(v)); n > 1 {
			return fmt.Sprintf("%s (+%s)", firstLine(v), plural(n-1, "line"))
		}
		return ui.PlainLine(v)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// lines splits text from a model or a tool into rows, cleaned of escape
// sequences and control characters.
func lines(s string) []string {
	s = strings.TrimSuffix(ui.PlainText(s), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func firstLine(s string) string {
	if l := lines(s); len(l) > 0 {
		return l[0]
	}
	return ""
}

func plural(n int, what string) string {
	if n == 1 {
		return "1 " + what
	}
	return fmt.Sprintf("%d %ss", n, what)
}
