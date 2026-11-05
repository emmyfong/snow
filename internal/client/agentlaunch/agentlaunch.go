// Package agentlaunch is the form that starts a Claude Code agent in its
// own git worktree and window: the folder, the new branch, the agent
// profile, and a role. Keys and the mouse both fill it in.
package agentlaunch

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/pkg/api"
)

// field is the part of the form that takes the keys.
type field int

// Fields, in Tab order.
const (
	fieldFolder field = iota
	fieldBranch
	fieldProfile
	fieldRole
	fieldCount
)

// maxText bounds a text field so a stray paste cannot grow without bound.
const maxText = 1024

// Model is one open form.
type Model struct {
	folder, branch []rune
	profiles       []string
	selected       int // index in profiles
	roles          []api.RoleInfo
	role           int // 0 is no role, i+1 is roles[i]
	focus          field
	note           string // why the form cannot start an agent yet
}

// New opens the form on folder, the folder of the pane it starts from,
// with the agent profiles and the roles to pick from.
func New(folder string, profiles []string, roles []api.RoleInfo) *Model {
	m := &Model{folder: []rune(folder), profiles: profiles, roles: roles, focus: fieldBranch}
	if len(profiles) == 0 {
		m.focus = fieldFolder
	}
	return m
}

// Key handles one key press. Tab and Shift+Tab move between the fields,
// typing edits the folder or the branch, Up and Down pick the profile,
// Enter starts the agent, and Escape closes the form.
func (m *Model) Key(k tea.Key) picker.Result {
	switch {
	case k.Code == tea.KeyEscape:
		return picker.Cancelled
	case k.Code == tea.KeyEnter:
		return m.submit()
	case k.Code == tea.KeyTab && k.Mod.Contains(tea.ModShift):
		m.move(-1)
	case k.Code == tea.KeyTab:
		m.move(1)
	case k.Code == tea.KeyUp && m.focus == fieldProfile:
		m.selected = max(m.selected-1, 0)
	case k.Code == tea.KeyDown && m.focus == fieldProfile:
		m.selected = min(m.selected+1, max(len(m.profiles)-1, 0))
	case k.Code == tea.KeyUp && m.focus == fieldRole:
		m.role = max(m.role-1, 0)
	case k.Code == tea.KeyDown && m.focus == fieldRole:
		m.role = min(m.role+1, len(m.roles))
	case k.Code == tea.KeyUp && m.focus > fieldFolder:
		m.focus--
	case k.Code == tea.KeyDown && m.focus < fieldProfile:
		m.focus++
	case k.Code == tea.KeyBackspace:
		if t := m.text(); t != nil && len(*t) > 0 {
			*t = (*t)[:len(*t)-1]
		}
	case k.Text != "":
		m.Paste(k.Text)
	}
	m.note = ""
	return picker.Open
}

// move moves the focus step fields on, round the form. The role field is
// skipped when there are no roles.
func (m *Model) move(step int) {
	for {
		m.focus = (m.focus + fieldCount + field(step)) % fieldCount
		if m.focus != fieldRole || len(m.roles) > 0 {
			return
		}
	}
}

// Paste types text into the focused text field. Line breaks are dropped:
// a folder or a branch is one line.
func (m *Model) Paste(text string) {
	t := m.text()
	if t == nil {
		return
	}
	text = strings.NewReplacer("\r", "", "\n", "").Replace(text)
	*t = append(*t, []rune(text)...)
	if len(*t) > maxText {
		*t = (*t)[:maxText]
	}
}

// text returns the focused text field, or nil when the profile list has
// the focus.
func (m *Model) text() *[]rune {
	switch m.focus {
	case fieldFolder:
		return &m.folder
	case fieldBranch:
		return &m.branch
	}
	return nil
}

// submit accepts the form, or says what it still needs.
func (m *Model) submit() picker.Result {
	switch {
	case len(m.profiles) == 0:
		m.note = noProfile
		return picker.Open
	case strings.TrimSpace(string(m.folder)) == "":
		m.note = "Type the folder the agent works in."
		m.focus = fieldFolder
		return picker.Open
	}
	return picker.Accepted
}

// noProfile says how to get an agent profile.
const noProfile = "No Claude Code profile found. Press Ctrl+b a once, so Snow looks again, or get it: claude.com/claude-code"

// Command is the agent-new command the filled form sends, from pane
// target.
func (m *Model) Command(target int) *api.Command {
	args := map[string]string{
		api.ArgFolder:  strings.TrimSpace(string(m.folder)),
		api.ArgProfile: m.profiles[m.selected],
	}
	if b := strings.TrimSpace(string(m.branch)); b != "" {
		args[api.ArgBranch] = b
	}
	if m.role > 0 {
		args[api.ArgRole] = m.roles[m.role-1].Name
	}
	return &api.Command{Op: api.OpAgentNew, Target: target, Args: args}
}
