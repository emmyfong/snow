package chat

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// Requests a chat raises for the session, through Asked, for choices it
// cannot carry out itself.
const (
	AskLogin     = "login"      // save a provider key
	AskLogout    = "logout"     // remove a provider key
	AskPickModel = "pick-model" // choose another model
	AskResume    = "resume"     // choose a past conversation
	AskPickMode  = "pick-mode"  // choose the approval mode
	AskHelp      = "help"       // show the commands and keys
)

// errorTitles name each failure in its box's top edge.
var errorTitles = map[string]string{
	api.ChatErrNoKey:          "Connect a provider",
	api.ChatErrBadKey:         "Key rejected",
	api.ChatErrModelNotFound:  "Model not found",
	api.ChatErrRateLimited:    "Rate limited",
	api.ChatErrOverloaded:     "Provider overloaded",
	api.ChatErrContextTooLong: "Conversation too long for the model",
	api.ChatErrNetwork:        "Connection lost",
	api.ChatErrSave:           "Conversation not saved",
}

// errorNotes add what the user cannot tell from the message.
var errorNotes = map[string]string{
	api.ChatErrSave:           "Nothing was sent to the model.",
	api.ChatErrContextTooLong: "Start a new chat to go on.",
	api.ChatErrModelNotFound:  "The chat keeps its model until you pick another.",
}

var choiceLabels = map[string]string{
	api.ChatChoiceRetry:     "Retry",
	api.ChatChoiceWait:      "Retry",
	api.ChatChoicePickModel: "Pick another model",
	api.ChatChoiceLogin:     "Log in",
}

var errorHints = []ui.Hint{{Key: "Enter", Desc: "choose"}, {Key: "Left Right", Desc: "move"}}

// errorChoices are the choices e offers that this client knows, without
// repeats: "wait" and "retry" both retry.
func errorChoices(e *api.ChatError) []string {
	var out []string
	seen := map[string]bool{}
	for _, c := range e.Choices {
		l, ok := choiceLabels[c]
		if !ok || seen[l] {
			continue
		}
		seen[l] = true
		if c == api.ChatChoiceWait {
			c = api.ChatChoiceRetry
		}
		out = append(out, c)
	}
	return out
}

// liveError returns the index of the failure whose choices take keys: the
// last entry, when it failed and no turn runs. Else -1.
func (m *Model) liveError() int {
	n := len(m.snap.Entries) - 1
	if n < 0 || m.snap.Running {
		return -1
	}
	if e := m.snap.Entries[n]; e.Kind != api.ChatEntryError || e.Error == nil || len(errorChoices(e.Error)) == 0 {
		return -1
	}
	if m.errAt != n {
		m.errAt, m.errChoice = n, 0
	}
	return n
}

// errorBox draws failure e at width; live boxes get their choices.
func (m *Model) errorBox(e *api.ChatError, live bool, width int) (string, ui.ChoiceOptions) {
	title, ok := errorTitles[e.Code]
	if !ok {
		title = "Turn failed"
	}
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	var body []string
	for _, l := range strings.Split(ansi.Wrap(ui.PlainLine(e.Message), max(width-4, 1), ""), "\n") {
		body = append(body, " "+l)
	}
	if e.RetryAfter > 0 {
		body = append(body, " "+fmt.Sprintf("The provider asks to wait %s before a retry.", elapsed(seconds(e.RetryAfter))))
	}
	if note := errorNotes[e.Code]; note != "" {
		body = append(body, " "+muted.Render(note))
	}
	o := ui.ChoiceOptions{Width: width, Title: title, Body: body, Selected: -1, Alert: true}
	if live {
		for _, c := range errorChoices(e) {
			o.Choices = append(o.Choices, choiceLabels[c])
		}
		o.Selected = m.errChoice
		o.Body = append(o.Body, " "+ui.KeyHints(m.th, errorHints, max(width-3, 0)))
	}
	return ui.ChoiceBox(m.th, o), o
}

// errorKey handles Enter, Left, and Right on an empty message line while a
// failure offers choices. It reports false for other keys, and for these
// keys while something is typed, so typing a new message always works.
func (m *Model) errorKey(k tea.Key) (*api.ChatInput, bool) {
	i := m.liveError()
	if i < 0 || len(m.input) > 0 || k.Mod != 0 {
		return nil, false
	}
	n := len(errorChoices(m.snap.Entries[i].Error))
	switch k.Code {
	case tea.KeyLeft:
		m.errChoice = (m.errChoice + n - 1) % n
	case tea.KeyRight:
		m.errChoice = (m.errChoice + 1) % n
	case tea.KeyEnter:
		return m.pickError(i, m.errChoice), true
	default:
		return nil, false
	}
	m.invalidate()
	return nil, true
}

// pickError acts on choice c of failure i. A retry goes to the server; the
// chat never switches the model by itself, so picking a model or logging
// in becomes a request for the session.
func (m *Model) pickError(i, c int) *api.ChatInput {
	choices := errorChoices(m.snap.Entries[i].Error)
	if c < 0 || c >= len(choices) {
		return nil
	}
	m.errChoice = c
	m.invalidate()
	switch choices[c] {
	case api.ChatChoiceRetry:
		return m.Retry()
	case api.ChatChoiceLogin:
		m.asked = AskLogin
	case api.ChatChoicePickModel:
		m.asked = AskPickModel
	}
	return nil
}

// Asked returns the request the last key or click raised, AskLogin,
// AskLogout, or AskPickModel, or "", and clears it.
func (m *Model) Asked() string {
	a := m.asked
	m.asked = ""
	return a
}

func seconds(s float64) time.Duration { return time.Duration(math.Ceil(s)) * time.Second }
