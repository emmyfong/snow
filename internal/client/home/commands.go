package home

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/confirm"
	"github.com/emmyfong/snow/pkg/api"
)

// SendMsg asks the client to send Msg to the server, such as a request to
// end or rename a session. The home page itself holds no connection.
type SendMsg struct{ Msg api.Message }

// command is one slash command.
type command struct {
	name, args, desc string
	run              func(m *Model, rest string) tea.Cmd
}

// commands lists the slash commands in the order the list shows them.
var commands = []command{
	{"new", "[name]", "Start or open a session", (*Model).cmdNew},
	{"sessions", "", "Show the sessions", (*Model).cmdSessions},
	{"rename", "[session] <name>", "Rename a session", (*Model).cmdRename},
	{"kill", "[session]", "End a session and its programs", (*Model).cmdKill},
	{"theme", "[name]", "Pick the colors", (*Model).cmdTheme},
	{"help", "", "Show commands and keys", (*Model).cmdHelp},
	{"quit", "", "Leave Snow; sessions keep running", (*Model).cmdQuit},
}

// matchCommands returns the commands whose names fuzzy-match query, best
// first; equal scores keep the table order.
func matchCommands(query string) []command {
	type scored struct {
		c     command
		score int
	}
	var found []scored
	for _, c := range commands {
		if s, ok := fuzzyScore(query, c.name); ok {
			found = append(found, scored{c, s})
		}
	}
	slices.SortStableFunc(found, func(a, b scored) int { return b.score - a.score })
	out := make([]command, len(found))
	for i, f := range found {
		out[i] = f.c
	}
	return out
}

// commanding reports whether the input is a slash command.
func (m *Model) commanding() bool { return strings.HasPrefix(m.input, "/") }

// commandWord splits the typed command into its name and the rest.
func (m *Model) commandWord() (word, rest string) {
	word, rest, _ = strings.Cut(strings.TrimPrefix(m.input, "/"), " ")
	return word, strings.TrimSpace(rest)
}

// matches returns the commands that match what is typed so far.
func (m *Model) matches() []command {
	word, _ := m.commandWord()
	return matchCommands(word)
}

// commandKey handles the keys that act on the command list, and reports
// whether it handled k.
func (m *Model) commandKey(k tea.Key) (tea.Cmd, bool) {
	switch k.Code {
	case tea.KeyUp:
		m.cmdSel = max(m.cmdSel-1, 0)
	case tea.KeyDown:
		m.cmdSel = min(m.cmdSel+1, max(len(m.matches())-1, 0))
	case tea.KeyTab:
		if ms := m.matches(); m.cmdSel < len(ms) {
			m.complete(ms[m.cmdSel])
		}
	case tea.KeyEnter:
		return m.runCommand(), true
	default:
		return nil, false
	}
	return nil, true
}

// complete puts command c in the input, ready for its argument.
func (m *Model) complete(c command) {
	m.input = "/" + c.name
	if c.args != "" {
		m.input += " "
	}
	m.cmdSel = 0
}

// runCommand runs the typed command: the one named exactly, else the
// highlighted match.
func (m *Model) runCommand() tea.Cmd {
	word, rest := m.commandWord()
	var c *command
	for i := range commands {
		if commands[i].name == word {
			c = &commands[i]
		}
	}
	if ms := m.matches(); c == nil && m.cmdSel < len(ms) {
		c = &ms[m.cmdSel]
	}
	if c == nil {
		m.notice = "Unknown command /" + word + ". Type / to see the commands."
		return nil
	}
	return c.run(m, rest)
}

// done clears the input after a command that worked.
func (m *Model) done() { m.input, m.target, m.cmdSel = "", "", 0 }

func (m *Model) exists(name string) bool {
	return slices.ContainsFunc(m.sessions, func(s api.SessionInfo) bool { return s.Name == name })
}

func (m *Model) cmdNew(name string) tea.Cmd {
	o := OpenMsg{Session: name, Create: !m.exists(name)}
	if cmd := m.open(o); cmd != nil {
		m.done()
		return cmd
	}
	return nil
}

func (m *Model) cmdSessions(string) tea.Cmd {
	m.done()
	m.selected = min(0, len(m.sessions)-1)
	return nil
}

// cmdRename renames the highlighted session to rest, or, when rest starts
// with a session's name, that session to what follows.
func (m *Model) cmdRename(rest string) tea.Cmd {
	session, name := m.target, rest
	if first, after, ok := strings.Cut(rest, " "); ok && m.exists(first) {
		session, name = first, strings.TrimSpace(after)
	}
	switch {
	case rest == "":
		m.notice = "Type /rename <new name> for the selected session, or /rename <session> <new name>."
		return nil
	case session == "":
		m.notice = "Select a session first, or type /rename <session> <new name>."
		return nil
	}
	if err := api.ValidSessionName(name); err != nil {
		m.notice = "/rename: " + reason(err)
		return nil
	}
	m.done()
	if session == m.back {
		m.back = name
	}
	return sendCmd(&api.RenameSession{Session: session, Name: name})
}

// cmdKill asks before it ends the named or highlighted session.
func (m *Model) cmdKill(rest string) tea.Cmd {
	name := rest
	if name == "" {
		name = m.target
	}
	switch {
	case name == "":
		m.notice = "Select a session first, or type /kill <session>."
		return nil
	case !m.exists(name):
		m.notice = "No session named " + name + "."
		return nil
	}
	m.done()
	m.killing = name
	m.confirm = confirm.New("Kill session", fmt.Sprintf("Kill session %s and its programs?", name), "Kill")
	return nil
}

func (m *Model) cmdHelp(string) tea.Cmd {
	m.done()
	m.help = true
	return nil
}

func (m *Model) cmdQuit(string) tea.Cmd {
	m.done()
	return func() tea.Msg { return QuitMsg{} }
}

// confirmResult acts on the kill popup's answer.
func (m *Model) confirmResult(r confirm.Result) tea.Cmd {
	if r == confirm.Open {
		return nil
	}
	name := m.killing
	m.confirm, m.killing = nil, ""
	if r == confirm.Yes {
		return sendCmd(&api.KillSession{Session: name})
	}
	return nil
}

// reason returns err's message without the protocol's error prefix.
func reason(err error) string {
	msg := err.Error()
	if p := api.ErrInvalidSessionRequest.Error() + ": "; errors.Is(err, api.ErrInvalidSessionRequest) {
		msg = strings.TrimPrefix(msg, p)
	}
	return msg
}

func sendCmd(msg api.Message) tea.Cmd { return func() tea.Msg { return SendMsg{Msg: msg} } }
