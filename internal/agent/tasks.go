package agent

import (
	"fmt"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/tool"
)

// SetTasks saves the task list, then reports it. It runs on the turn's
// goroutine.
func (u callUser) SetTasks(tasks []tool.Task) error {
	c := u.c
	rec := &convstore.Tasks{At: c.now(), Tasks: make([]convstore.TaskItem, len(tasks))}
	ev := &TasksChanged{Tasks: make([]Task, len(tasks))}
	for i, t := range tasks {
		rec.Tasks[i] = convstore.TaskItem{Text: t.Text, State: t.State}
		ev.Tasks[i] = Task{Text: t.Text, State: t.State}
	}
	if err := c.w.Append(rec); err != nil {
		return fmt.Errorf("the task list could not be saved: %w", err)
	}
	c.emit(ev)
	return nil
}

// TasksOf returns the task list a loaded conversation ends with: the last
// one saved in the file, or none.
func TasksOf(loaded *convstore.Conversation) []Task {
	for i := len(loaded.Records) - 1; i >= 0; i-- {
		if r, ok := loaded.Records[i].(*convstore.Tasks); ok {
			return tasksOf(r)
		}
	}
	return nil
}

func tasksOf(r *convstore.Tasks) []Task {
	out := make([]Task, len(r.Tasks))
	for i, t := range r.Tasks {
		out[i] = Task{Text: t.Text, State: t.State}
	}
	return out
}
