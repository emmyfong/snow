package gitpane

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/ui"
)

// Action is a git action and the key shared by its button and menu item.
type Action struct {
	Label    string
	Shortcut string
	Key      tea.Key
}

type actionID int

const (
	actionNone actionID = iota
	actionStage
	actionUnstage
	actionDiff
	actionOpen
	actionCommit
	actionToggle
	actionStageAll
	actionRefresh
	actionRemove
)

type actionRule int

const (
	whenAlways actionRule = iota
	whenUnstaged
	whenStaged
	whenFile
	whenWorktree
	whenRemovable
)

type actionSpec struct {
	Action
	id     actionID
	rule   actionRule
	button bool
	alias  string
}

// The action list owns the key, labels, and selection rules for every list
// action. Key handling ignores display rules so invalid actions keep their
// existing explanatory notes.
var listActions = []actionSpec{
	{Action: Action{"Stage", "s", tea.Key{Code: 's', Text: "s"}}, id: actionStage, rule: whenUnstaged, button: true},
	{Action: Action{"Unstage", "u", tea.Key{Code: 'u', Text: "u"}}, id: actionUnstage, rule: whenStaged, button: true},
	{Action: Action{"Diff", "Enter, d", tea.Key{Code: tea.KeyEnter}}, id: actionDiff, rule: whenFile, button: true, alias: "d"},
	{Action: Action{"Open", "Enter", tea.Key{Code: tea.KeyEnter}}, id: actionOpen, rule: whenWorktree, button: true},
	{Action: Action{"Commit...", "c", tea.Key{Code: 'c', Text: "c"}}, id: actionCommit, button: true},
	{Action: Action{"Stage or unstage", "Space", tea.Key{Code: tea.KeySpace, Text: " "}}, id: actionToggle, rule: whenFile},
	{Action: Action{"Stage all", "a", tea.Key{Code: 'a', Text: "a"}}, id: actionStageAll},
	{Action: Action{"Refresh", "r", tea.Key{Code: 'r', Text: "r"}}, id: actionRefresh, button: true},
	{Action: Action{"Remove worktree", "x", tea.Key{Code: 'x', Text: "x"}}, id: actionRemove, rule: whenRemovable},
}

func (m *Model) actionAllowed(rule actionRule) bool {
	es := m.entries()
	i := m.selected(es)
	if rule == whenAlways {
		return true
	}
	if i < 0 {
		return false
	}
	e := es[i]
	switch rule {
	case whenUnstaged:
		return e.file() && !e.staged
	case whenStaged:
		return e.file() && e.staged
	case whenFile:
		return e.file()
	case whenWorktree:
		return e.wt != nil
	case whenRemovable:
		return m.CanRemoveSelectedWorktree()
	}
	return false
}

// MenuActions returns the actions available for the pane's current mode and
// selection, in display order. Each shortcut describes the key it sends.
func (m *Model) MenuActions() []Action {
	if m.mode != modeList || m.removePath != "" {
		return nil
	}
	var out []Action
	for _, a := range listActions {
		if m.actionAllowed(a.rule) {
			out = append(out, a.Action)
		}
	}
	return out
}

// MenuKey resolves the key name carried by a menu item from MenuActions.
// The current action list also rejects menu items left from another mode.
func (m *Model) MenuKey(name string) (tea.Key, bool) {
	for _, a := range m.MenuActions() {
		if a.Key.String() == name {
			return a.Key, true
		}
	}
	return tea.Key{}, false
}

func listAction(k tea.Key) actionID {
	for _, a := range listActions {
		if a.Key.Code == tea.KeyEnter && k.Code == tea.KeyEnter || a.Key.Code == tea.KeySpace && k.Code == tea.KeySpace || a.Key.Text != "" && k.Text == a.Key.Text || a.alias != "" && k.Text == a.alias {
			return a.id
		}
	}
	return actionNone
}

func (m *Model) listButtons() (ui.ButtonRowOptions, []Action) {
	o := ui.ButtonRowOptions{Width: m.width}
	var actions []Action
	var right Action
	for _, a := range listActions {
		if !a.button || !m.actionAllowed(a.rule) {
			continue
		}
		if a.id == actionCommit {
			o.Right = a.Label
			right = a.Action
			continue
		}
		if a.id == actionRefresh && len(m.entries()) > 0 {
			continue
		}
		o.Choices = append(o.Choices, a.Label)
		actions = append(actions, a.Action)
	}
	if o.Right != "" {
		actions = append(actions, right)
	}
	return o, actions
}
