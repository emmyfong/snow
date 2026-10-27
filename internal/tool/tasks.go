package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Limits of the tasks tool, so the list fits above the chat input.
const (
	tasksMax     = 50
	tasksMaxText = 200
)

// ErrBadTasks is the cause of an error for a task list the chat cannot
// show.
var ErrBadTasks = errors.New("bad task list")

type tasksTool struct{}

func init() { register(tasksTool{}) }

func (tasksTool) Def() Def {
	task := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": prop("string", "what to do, in a few words"),
			"state": map[string]any{
				"type": "string", "enum": []string{TaskPending, TaskInProgress, TaskDone},
				"description": "pending, in_progress, or done",
			},
		},
		"required":             []string{"text", "state"},
		"additionalProperties": false,
	}
	return Def{
		Name: "tasks",
		Description: "Set your task list, which the user sees above the chat input. " +
			"Send the whole list each time, in order. Use it for work of three steps or more: " +
			"mark a task in_progress when you start it and done when it is done. An empty list clears it.",
		Schema: schema(map[string]any{
			"tasks": map[string]any{"type": "array", "items": task, "maxItems": tasksMax, "description": "the whole list"},
		}, "tasks"),
	}
}

func (tasksTool) Run(_ context.Context, env Env, raw json.RawMessage) (string, error) {
	var in struct {
		Tasks *[]Task `json:"tasks"`
	}
	if err := input(raw, &in); err != nil {
		return "", err
	}
	if in.Tasks == nil {
		// A missing list is a mistake; only an empty one clears.
		return "", fmt.Errorf("%w: no tasks; send [] to clear the list", ErrBadTasks)
	}
	tasks := *in.Tasks
	for i := range tasks {
		tasks[i].Text = strings.TrimSpace(tasks[i].Text)
	}
	if err := checkTasks(tasks); err != nil {
		return "", err
	}
	if env.User == nil {
		return "", ErrNoUser
	}
	if err := env.User.SetTasks(tasks); err != nil {
		return "", err
	}
	if len(tasks) == 0 {
		return "The task list is cleared.", nil
	}
	done := 0
	for _, t := range tasks {
		if t.State == TaskDone {
			done++
		}
	}
	return fmt.Sprintf("The task list is saved: %d of %d done.", done, len(tasks)), nil
}

// checkTasks returns an error wrapping ErrBadTasks when tasks breaks a
// limit of the tasks tool.
func checkTasks(tasks []Task) error {
	if len(tasks) > tasksMax {
		return fmt.Errorf("%w: %d tasks; the limit is %d", ErrBadTasks, len(tasks), tasksMax)
	}
	for i, t := range tasks {
		switch {
		case t.Text == "":
			return fmt.Errorf("%w: task %d has no text", ErrBadTasks, i+1)
		case utf8.RuneCountInString(t.Text) > tasksMaxText:
			return fmt.Errorf("%w: task %d is over %d characters", ErrBadTasks, i+1, tasksMaxText)
		case strings.ContainsAny(t.Text, "\r\n"):
			return fmt.Errorf("%w: task %d is not one line", ErrBadTasks, i+1)
		case t.State != TaskPending && t.State != TaskInProgress && t.State != TaskDone:
			return fmt.Errorf("%w: task %d has state %q; use %s, %s, or %s", ErrBadTasks, i+1, t.State, TaskPending, TaskInProgress, TaskDone)
		}
	}
	return nil
}
