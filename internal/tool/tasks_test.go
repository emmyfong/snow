package tool

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestTasksRejectsBadState(t *testing.T) {
	many := make([]map[string]string, tasksMax+1)
	for i := range many {
		many[i] = map[string]string{"text": "x", "state": TaskPending}
	}
	tooMany, _ := json.Marshal(map[string]any{"tasks": many})
	tests := []struct {
		name, in string
	}{
		{"unknown state", `{"tasks":[{"text":"a","state":"doing"}]}`},
		{"no state", `{"tasks":[{"text":"a"}]}`},
		{"blank text", `{"tasks":[{"text":"  ","state":"done"}]}`},
		{"long text", `{"tasks":[{"text":"` + strings.Repeat("x", tasksMaxText+1) + `","state":"done"}]}`},
		{"two lines", `{"tasks":[{"text":"a\nb","state":"done"}]}`},
		{"too many", string(tooMany)},
		{"no list", `{}`},
		{"unknown field", `{"tasks":[{"text":"a","state":"done","owner":"me"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &fakeUser{}
			r := Builtin().Call(context.Background(), Env{User: u}, "tasks", json.RawMessage(tt.in), nil)
			if !r.IsError {
				t.Fatalf("result = %+v, want an error", r)
			}
			if len(u.tasks) != 0 {
				t.Fatalf("set %v; a bad list must not reach the user", u.tasks)
			}
		})
	}
}

func TestTasksSet(t *testing.T) {
	u := &fakeUser{}
	in := `{"tasks":[{"text":" read code ","state":"done"},{"text":"fix parser","state":"in_progress"},{"text":"add tests","state":"pending"}]}`
	r := Builtin().Call(context.Background(), Env{User: u}, "tasks", json.RawMessage(in), nil)
	if r.IsError || r.Content != "The task list is saved: 1 of 3 done." {
		t.Fatalf("result %+v", r)
	}
	want := []Task{{"read code", TaskDone}, {"fix parser", TaskInProgress}, {"add tests", TaskPending}}
	if len(u.tasks) != 1 || len(u.tasks[0]) != 3 || u.tasks[0][0] != want[0] || u.tasks[0][2] != want[2] {
		t.Fatalf("set %+v, want %+v", u.tasks, want)
	}
	if r := Builtin().Call(context.Background(), Env{User: u}, "tasks", json.RawMessage(`{"tasks":[]}`), nil); r.Content != "The task list is cleared." {
		t.Fatalf("clear: %+v", r)
	}
	if r := run(t, Env{}, "tasks", `{"tasks":[]}`); !r.IsError || r.Content != ErrNoUser.Error() {
		t.Fatalf("no user: %+v", r)
	}
	saveErr := errors.New("disk full")
	if r := run(t, Env{User: &fakeUser{err: saveErr}}, "tasks", `{"tasks":[]}`); !r.IsError || r.Content != saveErr.Error() {
		t.Fatalf("save error: %+v", r)
	}
}
