package tool

import (
	"context"
	"errors"
)

// User is the person a tool waits for or reports to. The agent supplies
// it in Env; the tools reach the user only through it.
type User interface {
	// Ask shows q and waits, with no timeout, for the answer. It returns
	// an error when ctx ends first or the question cannot be shown.
	Ask(ctx context.Context, q Question) (Reply, error)
	// SetTasks replaces the task list the user sees. It returns an error
	// when the list could not be saved.
	SetTasks(tasks []Task) error
}

// ErrNoUser is the cause of an error from a tool that needs a User when
// Env has none.
var ErrNoUser = errors.New("no user to ask in this conversation")

// Question is what the ask tool shows the user.
type Question struct {
	Text    string
	Options []Option
	Multi   bool // more than one option may be picked
}

// Option is one answer the model offers. The user may always type
// another answer instead.
type Option struct {
	Label       string
	Description string
	Preview     string // longer text shown while the option is highlighted
}

// Reply is the user's answer to a Question: the labels picked, in the
// order of the options, or the text typed instead.
type Reply struct {
	Picked []string
	Other  string
}

// Task is one item of the agent's task list.
type Task struct {
	Text  string
	State string // one of the Task* values
}

// The states of a Task.
const (
	TaskPending    = "pending"
	TaskInProgress = "in_progress"
	TaskDone       = "done"
)
