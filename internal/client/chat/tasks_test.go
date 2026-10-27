package chat

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

var parserTasks = []api.ChatTask{
	{Text: "Read the parser and its tests", State: api.ChatTaskDone},
	{Text: "Find why nested lists lose their indent", State: api.ChatTaskDone},
	{Text: "Fix the indent in parseList", State: api.ChatTaskInProgress},
	{Text: "Add a regression test", State: api.ChatTaskPending},
	{Text: "Run the whole suite", State: api.ChatTaskPending},
}

// working returns a chat whose running turn set the parser task list.
func working(t *testing.T, w, h int) *Model {
	t.Helper()
	m, _ := newChat(w, h)
	m.Event(ev(api.ChatEvent{Kind: api.ChatEventTurnStarted, Turn: 1, Text: "Fix the nested list indent"}))
	m.Event(ev(api.ChatEvent{Kind: api.ChatEventToolCall, Tool: &api.ChatTool{ID: "t1", Name: "tasks", Done: true, Result: "The task list is saved: 2 of 5 done."}}))
	m.Event(ev(api.ChatEvent{Kind: api.ChatEventTasks, Tasks: parserTasks}))
	m.Event(ev(api.ChatEvent{Kind: api.ChatEventText, Text: "Fixing the indent now."}))
	return m
}

// TestTasksRow: the compact row counts what is done and names the task
// in progress; Ctrl+T and a click open and close the list.
func TestTasksRow(t *testing.T) {
	m := working(t, 100, 30)
	if s := screen(m); !strings.Contains(s, "Tasks 2/5 done") || !strings.Contains(s, "[>] Fix the indent in parseList") {
		t.Fatalf("compact row:\n%s", s)
	}
	if strings.Contains(screen(m), "Add a regression test") {
		t.Fatal("the closed list shows every task")
	}
	if in := m.Key(key('t', tea.ModCtrl, "")); in != nil {
		t.Fatalf("Ctrl+T sent %+v", in)
	}
	if !strings.Contains(screen(m), "[ ] Add a regression test") || !strings.Contains(screen(m), "[x] Read the parser") {
		t.Fatalf("Ctrl+T did not open the list:\n%s", screen(m))
	}
	y := rowOf(t, m, "Tasks 2/5 done")
	if in := m.Click(2, y); in != nil {
		t.Fatalf("click sent %+v", in)
	}
	if strings.Contains(screen(m), "Add a regression test") {
		t.Fatalf("a click did not close the list:\n%s", screen(m))
	}
	m.Event(ev(api.ChatEvent{Kind: api.ChatEventTasks}))
	if strings.Contains(screen(m), "Tasks") {
		t.Fatalf("an empty list still shows:\n%s", screen(m))
	}
	m.Key(key('t', tea.ModCtrl, ""))
	if m.tasksOpen {
		t.Fatal("Ctrl+T opened an empty list")
	}
}

// TestTasksRowCapped: a long list shows what fits and counts the rest.
func TestTasksRowCapped(t *testing.T) {
	m := working(t, 100, 24)
	var many []api.ChatTask
	for i := range 30 {
		many = append(many, api.ChatTask{Text: fmt.Sprintf("step %d", i+1), State: api.ChatTaskPending})
	}
	m.Event(ev(api.ChatEvent{Kind: api.ChatEventTasks, Tasks: many}))
	m.toggleTasks()
	if s := screen(m); !strings.Contains(s, "... 24 more") || len(m.View()) != 24 {
		t.Fatalf("rows %d:\n%s", len(m.View()), s)
	}
}

func TestTasksGolden(t *testing.T) {
	for _, open := range []bool{false, true} {
		for _, th := range []struct {
			name  string
			theme theme.Theme
		}{{"Dark", theme.Default()}, {"Light", lightTheme()}} {
			for _, size := range [][2]int{{80, 24}, {120, 40}} {
				name := "Compact"
				if open {
					name = "Open"
				}
				t.Run(fmt.Sprintf("%s%s%dx%d", name, th.name, size[0], size[1]), func(t *testing.T) {
					m := working(t, size[0], size[1])
					m.SetTheme(th.theme)
					if open {
						m.toggleTasks()
					}
					golden.RequireEqual(t, strings.Join(m.View(), "\n"))
				})
			}
		}
	}
}
