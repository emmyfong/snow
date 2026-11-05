package gitpane

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/pkg/api"
)

// Key handles a key press in the focused pane and returns the action to
// send to the server, or nil.
//
// In the changes: Up, Down, j, k, PgUp, PgDn, Home, and End move; s stages
// the selected file and u unstages it; Space does whichever fits; a stages
// every change; Enter or d shows the file's diff; c opens the commit box;
// r reads the folder again. In a diff: the arrows, PgUp, and PgDn scroll;
// s and u stage and unstage the file; Esc, q, or d go back. In the commit
// box: Enter commits; Alt+Enter, Shift+Enter, and Ctrl+J add a line; Esc
// closes the box and keeps the message.
func (m *Model) Key(k tea.Key) api.Message {
	if !m.loaded {
		return nil
	}
	switch m.mode {
	case modeDiff:
		return m.diffKey(k)
	case modeCommit:
		return m.commitKey(k)
	}
	return m.listKey(k)
}

func (m *Model) listKey(k tea.Key) api.Message {
	es := m.entries()
	i := m.selected(es)
	switch {
	case k.Code == tea.KeyUp || k.Text == "k":
		m.move(es, i-1)
	case k.Code == tea.KeyDown || k.Text == "j":
		m.move(es, i+1)
	case k.Code == tea.KeyPgUp:
		m.move(es, i-max(m.listHeight()-1, 1))
	case k.Code == tea.KeyPgDown:
		m.move(es, i+max(m.listHeight()-1, 1))
	case k.Code == tea.KeyHome:
		m.move(es, 0)
	case k.Code == tea.KeyEnd:
		m.move(es, len(es)-1)
	case k.Text == "s":
		return m.stage(es, i)
	case k.Text == "u":
		return m.unstage(es, i)
	case k.Code == tea.KeySpace:
		if i >= 0 && es[i].file() && es[i].staged {
			return m.unstage(es, i)
		}
		return m.stage(es, i)
	case k.Text == "a":
		return m.stageAll()
	case k.Code == tea.KeyEnter || k.Text == "d":
		return m.openDiff(es, i)
	case k.Text == "c":
		m.openCommit()
	case k.Text == "r":
		return m.action(api.GitActRefresh, nil)
	}
	return nil
}

func (m *Model) move(es []entry, i int) {
	if len(es) > 0 {
		m.selectIndex(es, i)
	}
}

func (m *Model) stage(es []entry, i int) api.Message {
	switch {
	case i < 0 || !es[i].file():
		m.note = "No file to stage."
		return nil
	case es[i].staged && !es[i].change.Unstaged():
		m.note = es[i].change.Path + " is already staged."
		return nil
	}
	return m.action(api.GitActStage, map[string]string{api.ArgGitPath: es[i].change.Path})
}

func (m *Model) unstage(es []entry, i int) api.Message {
	if i < 0 || !es[i].file() || !es[i].change.Staged() {
		m.note = "Select a staged file to unstage."
		return nil
	}
	return m.action(api.GitActUnstage, map[string]string{api.ArgGitPath: es[i].change.Path})
}

func (m *Model) stageAll() api.Message {
	for _, c := range m.st.Changes {
		if c.Unstaged() {
			return m.action(api.GitActStageAll, nil)
		}
	}
	m.note = "Nothing to stage."
	return nil
}

func (m *Model) openDiff(es []entry, i int) api.Message {
	switch {
	case i < 0 || es[i].cm != nil:
		return nil
	case es[i].wt != nil:
		return m.openWorktree(*es[i].wt)
	}
	return m.action(api.GitActDiff, map[string]string{api.ArgGitPath: es[i].change.Path, api.ArgGitStaged: api.GitStagedArg(es[i].staged)})
}

func (m *Model) openCommit() {
	if m.st.NotRepo {
		m.note = "This folder is not in a git repository."
		return
	}
	m.mode = modeCommit
}

func (m *Model) diffKey(k tea.Key) api.Message {
	d := m.st.Diff
	switch {
	case k.Code == tea.KeyEscape || k.Text == "q" || k.Text == "d" || k.Code == tea.KeyBackspace:
		return m.action(api.GitActCloseDiff, nil)
	case k.Code == tea.KeyUp || k.Text == "k":
		m.Scroll(1)
	case k.Code == tea.KeyDown || k.Text == "j":
		m.Scroll(-1)
	case k.Code == tea.KeyPgUp:
		m.Scroll(max(m.diffHeight()-1, 1))
	case k.Code == tea.KeyPgDown:
		m.Scroll(-max(m.diffHeight()-1, 1))
	case k.Code == tea.KeyHome:
		m.scroll = 0
	case k.Code == tea.KeyEnd:
		m.scroll = m.maxScroll()
	case k.Text == "s" && d != nil && !d.Staged:
		return m.action(api.GitActStage, map[string]string{api.ArgGitPath: d.Path})
	case k.Text == "u" && d != nil && d.Staged:
		return m.action(api.GitActUnstage, map[string]string{api.ArgGitPath: d.Path})
	}
	return nil
}

func (m *Model) commitKey(k tea.Key) api.Message {
	switch {
	case k.Code == tea.KeyEscape:
		m.mode = modeList
	case k.Code == tea.KeyEnter && k.Mod&(tea.ModShift|tea.ModAlt) != 0, k.Code == 'j' && k.Mod == tea.ModCtrl:
		m.msg, m.cursor = keys.InsertAt(m.msg, m.cursor, "\n")
	case k.Code == tea.KeyEnter:
		return m.commit()
	default:
		m.msg, m.cursor, _ = keys.EditAt(m.msg, m.cursor, k)
	}
	return nil
}

// commit sends the commit, unless the message is blank, nothing is
// staged, or a commit already waits for its answer.
func (m *Model) commit() api.Message {
	msg := m.message()
	switch {
	case m.committing:
		return nil
	case strings.TrimSpace(msg) == "":
		m.note = "Type a commit message first."
		return nil
	case !m.staged():
		m.note = "Nothing is staged. Press Esc, then s on a file to stage it."
		return nil
	case len(msg) > api.MaxCommitMessage:
		m.note = "The commit message is too long."
		return nil
	}
	m.committing = true
	return m.action(api.GitActCommit, map[string]string{api.ArgGitMessage: msg})
}

// Paste adds pasted text to the commit message while the box is open.
func (m *Model) Paste(text string) {
	if m.mode == modeCommit {
		m.msg, m.cursor = keys.InsertAt(m.msg, m.cursor, strings.ReplaceAll(text, "\r\n", "\n"))
	}
}

// Scroll moves the diff by rows, up when rows is positive, or the
// selection in the changes.
func (m *Model) Scroll(rows int) {
	switch m.mode {
	case modeDiff:
		m.scroll = min(max(m.scroll-rows, 0), m.maxScroll())
	case modeList:
		es := m.entries()
		m.move(es, m.selected(es)-rows)
	}
}
