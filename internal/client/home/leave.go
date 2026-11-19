package home

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/confirm"
	"github.com/emmyfong/snow/pkg/api"
)

func (m *Model) askLeave() tea.Cmd {
	count := 0
	for _, session := range m.sessions {
		if session.State == api.StateRunning {
			count++
		}
	}
	question := "Leave Snow?"
	switch {
	case count == 1:
		question = "Leave Snow? Your 1 session keeps running."
	case count >= 2:
		question = fmt.Sprintf("Leave Snow? Your %d sessions keep running.", count)
	}
	m.confirm = confirm.New("Leave Snow", question, "Leave")
	m.leaving = true
	return nil
}
