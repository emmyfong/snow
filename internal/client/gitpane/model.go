// Package gitpane draws a git pane: the branch and changed files of a
// folder, one file's diff, and a commit message box. Git runs on the
// server; the pane sends the user's actions as api.SidePaneAction and
// draws each api.GitState the server sends back.
package gitpane

import (
	"strings"

	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// mode is what the pane shows.
type mode int

const (
	modeList   mode = iota // the changes
	modeDiff               // one file's diff
	modeCommit             // the changes, with the commit message box
)

// entry is one row the user can select: a file's staged or its unstaged
// change, or a worktree. A file with both changes has two rows.
type entry struct {
	change api.GitChange
	staged bool
	wt     *api.GitWorktree // set for a worktree row
	cm     *api.GitCommit   // set for a history row
}

// The sections of the list, in their order.
const (
	secStaged = iota
	secChanges
	secWorktrees
	secHistory
)

// section is the part of the list e is in.
func (e entry) section() int {
	switch {
	case e.cm != nil:
		return secHistory
	case e.wt != nil:
		return secWorktrees
	case e.staged:
		return secStaged
	}
	return secChanges
}

// file reports whether e is a changed file, which the stage keys act on.
func (e entry) file() bool { return e.wt == nil && e.cm == nil }

// key names e across states, so the selection stays on it.
func (e entry) key() string {
	switch {
	case e.cm != nil:
		return "h\x00" + e.cm.Hash
	case e.wt != nil:
		return "w\x00" + e.wt.Path
	case e.staged:
		return "s\x00" + e.change.Path
	}
	return "u\x00" + e.change.Path
}

// committedPrefix starts the server's notice for a commit that worked.
const committedPrefix = "Committed "

// Model is one git pane.
type Model struct {
	pane          int
	th            theme.Theme
	st            api.GitState
	loaded        bool
	width, height int
	mode          mode
	// The selected row: by key, so it stays on the same row when the list
	// moves, and by index for when that row is gone.
	selKey   string
	selIndex int
	agents   []api.AgentInfo // the session's, for the worktrees' owners
	scroll   int             // diff rows scrolled past
	msg      []rune          // the commit message being typed
	cursor   int             // in msg, in runes
	// committing is set from sending a commit until its answer, so a
	// commit that worked clears the message and one that failed keeps it.
	// sentNotice is the notice shown when the commit went out: only a new
	// "Committed" notice tells that this commit worked, not a state that
	// came without one, such as after the server refused the action.
	committing bool
	sentNotice string
	note       string
}

// New returns the git pane for pane, waiting for its first state.
func New(pane int, th theme.Theme) *Model {
	return &Model{pane: pane, th: th.Resolve()}
}

// SetSize sets the pane's size in cells.
func (m *Model) SetSize(width, height int) { m.width, m.height = width, height }

// SetTheme sets the theme the pane draws with.
func (m *Model) SetTheme(th theme.Theme) { m.th = th.Resolve() }

// SetState shows st, the server's newest state of the pane. The diff view
// follows the state's open diff; the commit box stays open until a commit
// works or the user closes it.
func (m *Model) SetState(st api.GitState) {
	hadDiff := m.st.Diff != nil
	m.st, m.loaded = st, true
	if m.committing && !st.Loading {
		m.committing = false
		if st.Error == "" && st.Notice != m.sentNotice && strings.HasPrefix(st.Notice, committedPrefix) {
			m.msg, m.cursor = nil, 0
			m.mode = modeList
		}
	}
	switch {
	case m.mode == modeCommit:
	case st.Diff != nil:
		if !hadDiff || m.mode != modeDiff {
			m.scroll = 0
		}
		m.mode = modeDiff
	default:
		m.mode = modeList
	}
	m.keepSelection()
}

// Note returns a note for the status bar about the last key, once, or "".
func (m *Model) Note() string {
	n := m.note
	m.note = ""
	return n
}

// SetAgents gives the pane its session's agents, which own worktrees.
func (m *Model) SetAgents(agents []api.AgentInfo) { m.agents = agents }

// entries are the rows the user can select: staged changes first, then
// the others, each in git's order, then the worktrees when the repository
// has more than its main checkout.
func (m *Model) entries() []entry {
	var out []entry
	for _, c := range m.st.Changes {
		if c.Staged() {
			out = append(out, entry{change: c, staged: true})
		}
	}
	for _, c := range m.st.Changes {
		if c.Unstaged() {
			out = append(out, entry{change: c})
		}
	}
	if len(m.st.Worktrees) > 1 {
		for i := range m.st.Worktrees {
			out = append(out, entry{wt: &m.st.Worktrees[i]})
		}
	}
	for i := range m.st.History {
		out = append(out, entry{cm: &m.st.History[i]})
	}
	return out
}

// selected returns the selected row's index in entries, or -1 for none.
func (m *Model) selected(es []entry) int {
	for i, e := range es {
		if e.key() == m.selKey {
			return i
		}
	}
	return -1
}

// keepSelection moves the selection to a row that is there: the same
// change, else the row at the same place, else the last row.
func (m *Model) keepSelection() {
	es := m.entries()
	if i := m.selected(es); i >= 0 {
		m.selIndex = i
		return
	}
	if len(es) == 0 {
		m.selKey, m.selIndex = "", 0
		return
	}
	m.selectIndex(es, min(m.selIndex, len(es)-1))
}

func (m *Model) selectIndex(es []entry, i int) {
	i = min(max(i, 0), len(es)-1)
	m.selIndex = i
	m.selKey = es[i].key()
}

// staged reports whether any change is staged.
func (m *Model) staged() bool {
	for _, c := range m.st.Changes {
		if c.Staged() {
			return true
		}
	}
	return false
}

// readFailed reports whether git could not read the folder at all: an
// error and nothing else to show.
func (m *Model) readFailed() bool {
	return m.st.Error != "" && len(m.st.Changes) == 0 && m.st.Branch == "" && !m.st.Detached && !m.st.NotRepo
}

// action returns the side pane action for this pane.
func (m *Model) action(name string, args map[string]string) api.Message {
	return &api.SidePaneAction{Pane: m.pane, Action: name, Args: args}
}

// message returns the commit message as git should get it.
func (m *Model) message() string { return strings.TrimRight(string(m.msg), "\n") }
