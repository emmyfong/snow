package chat

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/client/textdiff"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// approval is the box's state for the request that waits. It starts over
// when another request comes.
type approval struct {
	id     string
	choice int    // the highlighted choice
	reason bool   // the reason field for a "no" shows
	text   string // the reason typed so far
	diff   bool   // the diff shows
}

// The box's choices, in order.
const (
	choiceYes = iota
	choiceAlways
	choiceNo
	choiceDiff
)

const maxBoxDiffRows = 20

// pending returns the request that waits, with the box's state reset when
// the request is new, or nil.
func (m *Model) pending() *api.ChatApproval {
	p := m.snap.Pending
	if p == nil {
		return nil
	}
	if m.box.id != p.ID {
		m.box = approval{id: p.ID}
	}
	return p
}

// choices are the box's button labels.
func choices(p *api.ChatApproval, diffShown bool) []string {
	out := []string{"Yes", "Always for " + p.Tool, "No"}
	switch {
	case p.Diff == nil:
	case diffShown:
		out = append(out, "Hide diff")
	default:
		out = append(out, "View diff")
	}
	return out
}

// boxOptions are what the box shows for p at width.
func (m *Model) boxOptions(p *api.ChatApproval, width int) ui.ChoiceOptions {
	head := p.Tool
	t := &api.ChatTool{Name: p.Tool, Input: p.Input, Diff: p.Diff}
	if s := subject(t); s != "" {
		head += " " + s
	}
	diff := diffOf(t)
	if diff != nil {
		a, r := textdiff.Counts(diff)
		head += "  " + lipgloss.NewStyle().Foreground(m.th.Added).Render("+"+strconv.Itoa(a)) + " " +
			lipgloss.NewStyle().Foreground(m.th.Removed).Render("-"+strconv.Itoa(r))
	}
	body := []string{" " + head}
	if len(p.Outside) > 0 {
		body = append(body, " "+lipgloss.NewStyle().Foreground(m.th.Removed).Render("Outside the folder: "+strings.Join(p.Outside, ", ")))
	}
	if m.box.diff && diff != nil {
		view := ui.DiffView(m.th, ui.DiffOptions{Width: width - 3, Lines: uiDiff(diff), Context: diffContext, MaxRows: maxBoxDiffRows})
		for _, r := range strings.Split(view, "\n") {
			body = append(body, " "+r)
		}
	}
	return ui.ChoiceOptions{
		Width: width, Title: "Allow " + p.Tool + "?", Body: body,
		Choices: choices(p, m.box.diff), Selected: m.box.choice,
		Reason: m.box.reason, ReasonValue: m.box.text,
	}
}

// approvalKey handles k while a request waits. It reports false for keys
// the box does not take, such as Esc to interrupt or PgUp to scroll.
func (m *Model) approvalKey(p *api.ChatApproval, k tea.Key) (*api.ChatInput, bool) {
	defer m.invalidate()
	if m.box.reason {
		switch k.Code {
		case tea.KeyEnter:
			return m.answer(p, api.DecisionNo, m.box.text), true
		case tea.KeyEscape:
			m.box.reason = false
			return nil, true
		}
		if s, ok := keys.Edit(m.box.text, k); ok {
			m.box.text = s
		}
		return nil, true
	}
	n := len(choices(p, m.box.diff))
	switch {
	case k.Code == tea.KeyLeft || (k.Code == tea.KeyTab && k.Mod == tea.ModShift):
		m.box.choice = (m.box.choice + n - 1) % n
	case k.Code == tea.KeyRight || k.Code == tea.KeyTab:
		m.box.choice = (m.box.choice + 1) % n
	case k.Code == tea.KeyEnter:
		return m.choose(p, m.box.choice), true
	case k.Text == "y":
		return m.choose(p, choiceYes), true
	case k.Text == "a":
		return m.choose(p, choiceAlways), true
	case k.Text == "n":
		return m.choose(p, choiceNo), true
	case k.Text == "d":
		return m.choose(p, choiceDiff), true
	default:
		return nil, k.Text != "" // typing goes nowhere while the box waits
	}
	return nil, true
}

// choose acts on choice i of the box for p.
func (m *Model) choose(p *api.ChatApproval, i int) *api.ChatInput {
	m.box.choice = i
	switch i {
	case choiceYes:
		return m.answer(p, api.DecisionYes, "")
	case choiceAlways:
		return m.answer(p, api.DecisionAlways, "")
	case choiceNo:
		m.box.reason = true
	case choiceDiff:
		if p.Diff != nil {
			m.box.diff = !m.box.diff
		}
	}
	m.invalidate()
	return nil
}

func (m *Model) answer(p *api.ChatApproval, decision, reason string) *api.ChatInput {
	in := m.chatInput(api.InputAnswer, reason)
	in.ID, in.Decision = p.ID, decision
	return in
}
