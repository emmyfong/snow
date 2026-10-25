package session

import (
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// commandRuns maps each session command in internal/client/command to what
// it does on this screen. Rest is what was typed after the command's name.
var commandRuns = map[string]func(m *Model, rest string) tea.Cmd{
	"new":    (*Model).cmdNew,
	"switch": (*Model).cmdSwitch,
	"rename": (*Model).cmdRename,
	"kill":   (*Model).cmdKill,
	"theme":  (*Model).cmdTheme,
	"chat":   (*Model).cmdChat,
}

// cmdChat opens a chat beside the focused pane.
func (m *Model) cmdChat(string) tea.Cmd {
	m.prompt = nil
	m.newChat()
	return nil
}

// current is the name of the session on screen, or "" before the first
// Layout.
func (m *Model) current() string {
	if m.layout == nil {
		return ""
	}
	return m.layout.Session
}

func (m *Model) exists(name string) bool {
	return slices.ContainsFunc(m.sessions, func(s api.SessionInfo) bool { return s.Name == name })
}

// cmdNew opens the named session, creating it when it does not exist, or
// starts a numbered one. This session keeps running.
func (m *Model) cmdNew(name string) tea.Cmd {
	if name != "" {
		if err := api.ValidSessionName(name); err != nil {
			m.prompt.notice = "new: " + command.Reason(err)
			return nil
		}
	}
	m.prompt = nil
	if name != "" && name == m.current() {
		return nil
	}
	return switchTo(SwitchMsg{Session: name, Create: name == "" || !m.exists(name)})
}

// cmdSwitch switches to the named session, or opens the session list.
func (m *Model) cmdSwitch(name string) tea.Cmd {
	switch {
	case name == "":
		m.prompt = nil
		return m.openSessions()
	case !m.exists(name):
		m.prompt.notice = "No session named " + name + "."
		return nil
	}
	m.prompt = nil
	if name == m.current() {
		return nil
	}
	return switchTo(SwitchMsg{Session: name})
}

// cmdRename renames this session to name, or asks for the new name.
func (m *Model) cmdRename(name string) tea.Cmd {
	if name == "" {
		m.prompt = &prompting{mode: modeName, input: m.current()}
		return nil
	}
	return m.renameTo(name)
}

// renameTo renames this session, unless the server would refuse the name.
func (m *Model) renameTo(name string) tea.Cmd {
	if err := api.ValidSessionName(name); err != nil {
		m.prompt.notice = "rename: " + command.Reason(err)
		return nil
	}
	m.prompt = nil
	if cur := m.current(); cur != "" && name != cur {
		m.send(&api.RenameSession{Session: cur, Name: name})
	}
	return nil
}

// cmdKill asks before it ends the named session, or this one.
func (m *Model) cmdKill(name string) tea.Cmd {
	if name == "" {
		name = m.current()
	}
	if name != m.current() && !m.exists(name) {
		m.prompt.notice = "No session named " + name + "."
		return nil
	}
	m.prompt = nil
	if name != "" {
		m.confirmKill(name)
	}
	return nil
}

// cmdTheme uses the named theme, or lists the themes to pick from.
func (m *Model) cmdTheme(name string) tea.Cmd {
	if name == "" {
		m.prompt = &prompting{mode: modeThemes}
		m.prompt.sel = max(slices.IndexFunc(theme.Builtin(), func(t theme.Theme) bool { return t.Name == m.base.Name }), 0)
		return nil
	}
	th, err := theme.Named(name)
	if err != nil {
		m.prompt.notice = "No theme named " + name + ". Type theme alone to list them."
		return nil
	}
	m.prompt = nil
	return useTheme(th.Name)
}

func useTheme(name string) tea.Cmd { return func() tea.Msg { return command.ThemeMsg{Name: name} } }
