package chat

import (
	"strconv"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// questionState is the question box's state for the open question. It
// starts over when another question comes.
type questionState struct {
	id     string
	choice int          // the highlighted row; len(Options) is "Other"
	picked map[int]bool // the options checked, for a multi-select question
	typing bool         // the field for another answer shows
	text   string       // the answer typed so far
}

// maxPreviewRows bounds the preview of the highlighted option.
const maxPreviewRows = 8

// otherLabel is the row for an answer the user types. The ask tool keeps
// it out of the model's options.
const otherLabel = "Other: type an answer"

// question returns the open question, with the box's state reset when the
// question is new, or nil.
func (m *Model) question() *api.ChatQuestion {
	q := m.snap.Question
	if q == nil {
		return nil
	}
	if m.ask.id != q.ID {
		m.ask = questionState{id: q.ID, picked: map[int]bool{}}
	}
	return q
}

// questionView is the box drawn for q at width, and the row in the box
// of its first option, for clicks.
func (m *Model) questionView(q *api.ChatQuestion, width int) (ui.ChoiceOptions, int) {
	inner := width - 2
	var body []string
	for _, l := range strings.Split(ansi.Wordwrap(ui.PlainText(q.Text), max(inner-2, 1), ""), "\n") {
		body = append(body, " "+l)
	}
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	if !m.snap.Running {
		body = append(body, " "+muted.Render("The turn stopped. An answer starts a new one."))
	}
	body = append(body, "")
	first := len(body) + 1 // below the top edge
	items := make([]ui.ListItem, 0, len(q.Options)+1)
	for i, o := range q.Options {
		label := strconv.Itoa(i+1) + " " + ui.PlainLine(o.Label)
		if q.Multi {
			mark := m.th.Glyphs.Unchecked
			if m.ask.picked[i] {
				mark = m.th.Glyphs.Checked
			}
			label = mark + " " + label
		}
		items = append(items, ui.ListItem{Cols: []string{label, muted.Render(ui.PlainLine(o.Description))}})
	}
	other := otherLabel
	if q.Multi {
		other = strings.Repeat(" ", ansi.StringWidth(m.th.Glyphs.Unchecked)+1) + other
	}
	items = append(items, ui.ListItem{Cols: []string{other, ""}})
	for _, r := range strings.Split(ui.List(m.th, ui.ListOptions{Items: items, Selected: m.ask.choice, Width: max(inner-1, 1)}), "\n") {
		body = append(body, " "+r)
	}
	if c := m.ask.choice; c < len(q.Options) && q.Options[c].Preview != "" {
		body = append(body, "")
		rows := strings.Split(ui.PlainText(q.Options[c].Preview), "\n")
		if len(rows) > maxPreviewRows {
			rows = append(rows[:maxPreviewRows-1], "...")
		}
		for _, r := range rows {
			body = append(body, "   "+muted.Render(ansi.Truncate(r, max(inner-4, 0), "…")))
		}
	}
	hints := []ui.Hint{{Key: "↑↓", Desc: "move"}, {Key: "Enter", Desc: "answer"}}
	if q.Multi {
		hints = []ui.Hint{{Key: "↑↓", Desc: "move"}, {Key: "Space", Desc: "check"}, {Key: "Enter", Desc: "send"}}
	}
	if m.snap.Running {
		hints = append(hints, ui.Hint{Key: "Esc", Desc: "stop"})
	}
	if !m.ask.typing {
		// The field brings its own keys.
		body = append(body, "", " "+ui.KeyHints(m.th, hints, max(inner-1, 0)))
	}
	var choices []string
	if q.Multi {
		choices = []string{"Send"} // for the mouse; Enter sends too
	}
	return ui.ChoiceOptions{
		Width: width, Title: "Question", Body: body, Choices: choices, Selected: -1,
		Reason: m.ask.typing, ReasonValue: m.ask.text, ReasonPrompt: " Answer: ", ReasonPlaceholder: "type your answer",
		ReasonHints: []ui.Hint{{Key: "Enter", Desc: "send"}, {Key: "Esc", Desc: "back"}},
	}, first
}

// questionKey handles k while a question is open. It reports false for
// keys the box does not take, such as Esc to interrupt or PgUp to scroll.
// While the turn waits, the box takes the keys unless the user was already
// typing a message. After an interrupt or a restart, only Tab, the arrows,
// Space, and Enter reach it, and only while the message is empty, so a new
// message can still move on.
func (m *Model) questionKey(q *api.ChatQuestion, k tea.Key) (*api.ChatInput, bool) {
	defer m.invalidate()
	if m.ask.typing {
		switch k.Code {
		case tea.KeyEnter:
			return m.replyTyped(q), true
		case tea.KeyEscape:
			m.ask.typing = false
			return nil, true
		}
		if s, ok := keys.Edit(m.ask.text, k); ok {
			m.ask.text = s
		}
		return nil, true
	}
	if len(m.input) > 0 && k.Code != tea.KeyTab {
		return nil, false
	}
	n := len(q.Options) + 1
	nav := k.Code == tea.KeyUp || k.Code == tea.KeyDown || k.Code == tea.KeyTab || k.Code == tea.KeyEnter || k.Code == tea.KeySpace
	if !m.snap.Running && !nav {
		return nil, false
	}
	switch {
	case k.Code == tea.KeyUp || k.Text == "k" || (k.Code == tea.KeyTab && k.Mod == tea.ModShift):
		m.ask.choice = picker.Move(m.ask.choice, -1, n)
	case k.Code == tea.KeyDown || k.Text == "j" || k.Code == tea.KeyTab:
		m.ask.choice = picker.Move(m.ask.choice, 1, n)
	case k.Code == tea.KeySpace && q.Multi:
		m.toggle(q, m.ask.choice)
	case k.Code == tea.KeyEnter:
		return m.pickRow(q, m.ask.choice, false), true
	case len(k.Text) == 1 && k.Text[0] >= '1' && k.Text[0] <= '9':
		if i := int(k.Text[0] - '1'); i < len(q.Options) {
			m.ask.choice = i
			if q.Multi {
				m.toggle(q, i)
				return nil, true
			}
			return m.pickRow(q, i, false), true
		}
	default:
		return nil, k.Text != "" // typing goes nowhere while the turn waits
	}
	return nil, true
}

// toggle checks or unchecks option i of a multi-select question.
func (m *Model) toggle(q *api.ChatQuestion, i int) {
	if i < len(q.Options) {
		m.ask.picked[i] = !m.ask.picked[i]
	}
}

// pickRow acts on row i of the box: "Other" opens the field; an option
// answers a single-select question; in a multi-select one, Enter sends
// the checked options, or the highlighted one when none is checked, and
// a click checks the option.
func (m *Model) pickRow(q *api.ChatQuestion, i int, click bool) *api.ChatInput {
	m.ask.choice = i
	if i >= len(q.Options) {
		m.ask.typing = true
		return nil
	}
	if !q.Multi {
		return m.reply(q, []string{q.Options[i].Label}, "")
	}
	if click {
		m.toggle(q, i)
		return nil
	}
	var picked []string
	for j, o := range q.Options {
		if m.ask.picked[j] {
			picked = append(picked, o.Label)
		}
	}
	if len(picked) == 0 {
		picked = []string{q.Options[i].Label}
	}
	return m.reply(q, picked, "")
}

// replyTyped sends the typed answer, unless it is blank.
func (m *Model) replyTyped(q *api.ChatQuestion) *api.ChatInput {
	text := m.ask.text
	if strings.TrimSpace(text) == "" {
		return nil
	}
	// A long answer is cut, not refused, so it still goes.
	for len(text) > api.MaxChatMessage {
		_, size := utf8.DecodeLastRuneInString(text)
		text = text[:len(text)-size]
	}
	return m.reply(q, nil, text)
}

func (m *Model) reply(q *api.ChatQuestion, picked []string, text string) *api.ChatInput {
	in := m.chatInput(api.ChatInputReply, text)
	in.ID, in.Picked = q.ID, picked
	return in
}

// questionClick handles a click at x, y in the question box: on a row it
// picks the row, and on Send it sends the checked options.
func (m *Model) questionClick(x, y int) *api.ChatInput {
	q := m.question()
	if q == nil || m.ask.typing {
		return nil
	}
	defer m.invalidate()
	o, first := m.questionView(q, m.width-ui.TranscriptIndent(m.width))
	if i := y - first; i >= 0 && i <= len(q.Options) {
		return m.pickRow(q, i, true)
	}
	for _, b := range ui.ChoiceButtons(o) {
		if y == b.Y && x >= b.X && x < b.X+b.W {
			return m.pickRow(q, min(m.ask.choice, len(q.Options)-1), false)
		}
	}
	return nil
}
