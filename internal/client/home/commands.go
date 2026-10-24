package home

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/confirm"
	"github.com/emmyfong/snow/pkg/api"
)

// SendMsg asks the client to send Msg to the server, such as a request to
// end or rename a session. The home page itself holds no connection.
type SendMsg struct{ Msg api.Message }

// runs maps each home command in internal/client/command to what it does
// on this page.
var runs = map[string]func(m *Model, rest string) tea.Cmd{
	"new":      (*Model).cmdNew,
	"sessions": (*Model).cmdSessions,
	"rename":   (*Model).cmdRename,
	"kill":     (*Model).cmdKill,
	"theme":    (*Model).cmdTheme,
	"help":     (*Model).cmdHelp,
	"quit":     (*Model).cmdQuit,
}

// commanding reports whether the input is a slash command.
func (m *Model) commanding() bool { return strings.HasPrefix(m.input, "/") }

// commandWord splits the typed command into its name and the rest.
func (m *Model) commandWord() (word, rest string) {
	word, rest, _ = strings.Cut(strings.TrimPrefix(m.input, "/"), " ")
	return word, strings.TrimSpace(rest)
}

// matches returns the commands that match what is typed so far.
func (m *Model) matches() []command.Command {
	word, _ := m.commandWord()
	return command.Match(word, command.Home)
}

// pickingSession reports whether the arrows pick a session: the typed word
// names a command that acts on one. While the word is partial, the arrows
// move in the command list.
func (m *Model) pickingSession() bool {
	word, _ := m.commandWord()
	c, ok := command.Find(word, command.Home)
	return ok && c.Session && len(m.sessions) > 0
}

// pickSession moves the command's session by delta, from the first one when
// none is picked yet.
func (m *Model) pickSession(delta int) {
	i := m.targetIndex()
	if i < 0 {
		i = 0
	} else {
		i = min(max(i+delta, 0), len(m.sessions)-1)
	}
	m.target = m.sessions[i].Name
}

// commandKey handles the keys that act on the command list, and reports
// whether it handled k.
func (m *Model) commandKey(k tea.Key) (tea.Cmd, bool) {
	switch k.Code {
	case tea.KeyUp:
		m.move(-1)
	case tea.KeyDown:
		m.move(1)
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

// move moves the picked session when the arrows pick one, else the
// highlight in the command list.
func (m *Model) move(delta int) {
	if m.pickingSession() {
		m.pickSession(delta)
		return
	}
	m.cmdSel = min(max(m.cmdSel+delta, 0), max(len(m.matches())-1, 0))
}

// complete puts command c in the input, ready for its argument.
func (m *Model) complete(c command.Command) {
	m.input = "/" + c.Name
	if c.Args != "" {
		m.input += " "
	}
	m.cmdSel = 0
}

// runCommand runs the typed command: the one named exactly, else the
// highlighted match.
func (m *Model) runCommand() tea.Cmd {
	word, rest := m.commandWord()
	c, ok := command.Find(word, command.Home)
	if ms := m.matches(); !ok && m.cmdSel < len(ms) {
		c, ok = ms[m.cmdSel], true
	}
	if !ok {
		m.notice = "Unknown command /" + word + ". Type / to see the commands."
		return nil
	}
	return runs[c.Name](m, rest)
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
	case len(m.sessions) == 0:
		m.notice = "No sessions to rename."
		return nil
	case session != "" && !m.exists(session):
		m.notice = "No session named " + session + "."
		return nil
	case rest == "" && session != "":
		m.startNaming(session)
		return nil
	case rest == "":
		m.notice = "Pick a session with the arrows, or type /rename <session> <new name>."
		return nil
	case session == "":
		m.notice = "Select a session first, or type /rename <session> <new name>."
		return nil
	}
	if err := api.ValidSessionName(name); err != nil {
		m.notice = "/rename: " + command.Reason(err)
		return nil
	}
	m.done()
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

func sendCmd(msg api.Message) tea.Cmd { return func() tea.Msg { return SendMsg{Msg: msg} } }
