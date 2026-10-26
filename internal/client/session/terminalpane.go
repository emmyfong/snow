package session

import "github.com/emmyfong/snow/pkg/api"

// paneState is what the screen knows of one pane: its rows and its cursor.
type paneState struct {
	lines  []string
	cursor api.Cursor
	mouse  bool // the program asked for mouse events
}

func (m *Model) applyUpdate(u *api.PaneUpdate) {
	st, ok := m.panes[u.Pane]
	if !ok {
		return
	}
	for _, l := range u.Lines {
		if l.Row < 0 {
			continue
		}
		if l.Row >= len(st.lines) {
			st.lines = resize(st.lines, l.Row+1)
		}
		st.lines[l.Row] = l.Text
	}
	if u.Cursor != nil {
		st.cursor = *u.Cursor
	}
	st.mouse = u.Mouse
}

func resize(lines []string, rows int) []string {
	if rows < 0 {
		rows = 0
	}
	if rows <= len(lines) {
		return lines[:rows]
	}
	return append(lines, make([]string, rows-len(lines))...)
}
