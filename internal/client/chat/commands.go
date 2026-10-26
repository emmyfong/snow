package chat

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/pkg/api"
)

// Host is what a chat command may do in a chat pane. It cannot read the
// transcript or reach the rest of the model, so a command stays a small
// piece that a feature package can add.
type Host interface {
	// Send asks the server to act: kind is an api.ChatInput kind, text is
	// its argument.
	Send(kind, text string)
	// Ask asks the session to open a picker or another overlay.
	Ask(request string)
	// Notice tells the user something without sending it to the server.
	Notice(text string)
	// SetInput replaces the message being typed.
	SetInput(text string)
	// Input returns the message being typed.
	Input() string
	// Model returns the chat's model as "provider/model", and its effort.
	Model() (model, effort string)
	// SwitchModel asks the server to make model the next turn's model. An
	// empty effort lets the provider choose.
	SwitchModel(model, effort string)
	// SetMode asks the server to set the approval mode now.
	SetMode(mode string)
	// SetEffort asks the server to set the effort from the next turn. An
	// empty level lets the provider choose.
	SetEffort(level string)
}

// RunFunc runs one command in a chat pane. rest is what the user typed
// after the command's name.
type RunFunc func(h Host, rest string)

// Register adds c to the shared registry and binds what it does in the chat.
// A feature package calls it while the program starts, so the chat pane and
// the home page need no edit for a new command. A command that only sends an
// input or shows a notice stays inside its own package. One that opens a
// popup also needs an Ask constant (see errors.go) and a case in the
// session's ownResult, because the session owns every popup.
func Register(c command.Command, run RunFunc) error {
	if run == nil {
		return fmt.Errorf("command %q: no run function", c.Name)
	}
	c.Where |= command.Chat
	if err := command.Register(c); err != nil {
		return err
	}
	chatRuns[c.Name] = run
	return nil
}

// chatRuns maps each chat command's name to what it does in this screen.
var chatRuns = map[string]RunFunc{}

// host is a command's narrow view of one chat pane.
type host struct{ m *Model }

func (h host) Send(kind, text string)  { h.m.sent = h.m.chatInput(kind, text) }
func (h host) Ask(request string)      { h.m.asked = request }
func (h host) Notice(text string)      { h.m.note = text }
func (h host) SetInput(text string)    { h.m.setInput(text) }
func (h host) Input() string           { return string(h.m.input) }
func (h host) Model() (string, string) { return h.m.snap.Model, h.m.snap.Effort }
func (h host) SwitchModel(model, effort string) {
	h.m.sent = h.m.SwitchModelInput(model, effort, false)
}

func (h host) SetMode(mode string) {
	in := h.m.chatInput(api.ChatInputMode, "")
	in.Mode = mode
	h.m.sent = in
}

func (h host) SetEffort(level string) {
	in := h.m.chatInput(api.ChatInputEffort, "")
	in.Effort = level
	h.m.sent = in
}

// commandQuery returns the text after the slash that filters the list, and
// whether the input is a command name being typed. A space before the
// cursor means the user is typing an argument, so the list closes.
func (m *Model) commandQuery() (string, bool) {
	if len(m.input) == 0 || m.input[0] != '/' || m.cursor == 0 {
		return "", false
	}
	for _, r := range m.input[1:m.cursor] {
		if r == ' ' {
			return "", false
		}
	}
	return string(m.input[1:m.cursor]), true
}

// commanding reports whether the input is a slash command of any kind.
func (m *Model) commanding() bool { return len(m.input) > 0 && m.input[0] == '/' }

// commandMatches returns the chat commands that match what is typed,
// grouped in the order the list shows them.
func (m *Model) commandMatches() []command.Command {
	q, ok := m.commandQuery()
	if !ok || m.reg == nil {
		return nil
	}
	return m.reg.Match(q, command.Chat)
}

// syncCommands keeps the list in step with the input. It opens the list
// when the first character is a slash and no argument has started, resets
// the selection when the filter changes, and closes it otherwise.
func (m *Model) syncCommands() {
	q, ok := m.commandQuery()
	if !ok {
		m.cmdOpen, m.cmdSel, m.cmdQuery = false, 0, ""
		return
	}
	if q != m.cmdQuery {
		m.cmdSel, m.cmdQuery = 0, q
	}
	m.cmdOpen = true
	n := len(m.commandMatches())
	if n == 0 {
		m.cmdSel = 0
		return
	}
	m.cmdSel = min(max(m.cmdSel, 0), n-1)
}

// commandKey handles the keys the list takes while it is open, and reports
// whether it handled k.
func (m *Model) commandKey(k tea.Key) (*api.ChatInput, bool) {
	ms := m.commandMatches()
	switch k.Code {
	case tea.KeyUp:
		m.cmdSel = max(m.cmdSel-1, 0)
	case tea.KeyDown:
		m.cmdSel = min(m.cmdSel+1, max(len(ms)-1, 0))
	case tea.KeyTab:
		if len(ms) > 0 {
			m.complete(ms[min(m.cmdSel, len(ms)-1)])
		}
	case tea.KeyEnter:
		if len(ms) == 0 {
			m.note = "No command matches. Type /help to see them all."
			return nil, true
		}
		return m.runSelected(), true
	case tea.KeyEscape:
		m.setInput("")
		m.cmdOpen = false
	default:
		return nil, false
	}
	return nil, true
}

// complete puts command c in the input, ready for its argument.
func (m *Model) complete(c command.Command) {
	text := "/" + c.Name
	if c.Args != "" {
		text += " "
	}
	m.setInput(text)
}

// setInput replaces the input and puts the cursor at its end.
func (m *Model) setInput(text string) {
	m.unselect()
	m.input = []rune(text)
	m.cursor = len(m.input)
	m.Follow()
	m.syncCommands()
}

// runSelected runs the highlighted match.
func (m *Model) runSelected() *api.ChatInput {
	ms := m.commandMatches()
	if len(ms) == 0 {
		return nil
	}
	return m.run(ms[min(m.cmdSel, len(ms)-1)], "")
}

// runCommand runs the typed command by its name, else the highlighted
// match. An unknown command becomes a note; it is never sent as a message.
func (m *Model) runCommand() *api.ChatInput {
	word, rest := command.Split(strings.TrimPrefix(string(m.input), "/"))
	if word == "" {
		m.note = "Type a command after /."
		return nil
	}
	if c, ok := m.reg.Find(word, command.Chat); ok {
		return m.run(c, rest)
	}
	if m.cmdOpen {
		if ms := m.commandMatches(); len(ms) > 0 {
			return m.run(ms[min(m.cmdSel, len(ms)-1)], "")
		}
	}
	m.note = "Unknown command /" + word + ". Type / to see the commands."
	return nil
}

// run clears the input and runs c with rest.
func (m *Model) run(c command.Command, rest string) *api.ChatInput {
	m.setInput("")
	m.cmdOpen = false
	run, ok := m.runs[c.Name]
	if !ok {
		m.note = "The command /" + c.Name + " is not available in this chat."
		return nil
	}
	run(host{m}, rest)
	return m.takeSent()
}

// takeSent returns the message a command asked to send, once.
func (m *Model) takeSent() *api.ChatInput {
	in := m.sent
	m.sent = nil
	return in
}
