package agent

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
	"github.com/emmyfong/snow/internal/tool"
)

// tasksCall is a reply that sets the task list to tasks.
func tasksCall(id string, tasks ...Task) providertest.Reply {
	list := make([]map[string]string, len(tasks))
	for i, t := range tasks {
		list[i] = map[string]string{"text": t.Text, "state": t.State}
	}
	in, _ := json.Marshal(map[string]any{"tasks": list})
	return providertest.Reply{Events: []provider.Event{
		&provider.ToolUse{ID: id, Name: "tasks", Input: in}, &provider.Stop{Reason: provider.StopToolUse},
	}}
}

func tasksTool(t *testing.T) tool.Tool {
	t.Helper()
	tl, ok := tool.Builtin().Get("tasks")
	if !ok {
		t.Fatal("tasks is not built in")
	}
	return tl
}

var (
	firstList = []Task{{"read code", "in_progress"}, {"fix parser", "pending"}}
	lastList  = []Task{{"read code", "done"}, {"fix parser", "in_progress"}}
)

// TestTasksRecordSaved: each call saves one task list record and reports
// the list, with no approval in the ask mode.
func TestTasksRecordSaved(t *testing.T) {
	h := newHarness(t, []tool.Tool{tasksTool(t)}, tasksCall("t1", firstList...), tasksCall("t2", lastList...), text("ok"))
	_ = h.conv.Send("fix the parser")
	if got := next[*TasksChanged](t, h); !reflect.DeepEqual(got.Tasks, firstList) {
		t.Fatalf("first list %+v", got.Tasks)
	}
	if got := next[*TasksChanged](t, h); !reflect.DeepEqual(got.Tasks, lastList) {
		t.Fatalf("second list %+v", got.Tasks)
	}
	if e := next[*TurnEnded](t, h); e.Reason != EndDone {
		t.Fatalf("turn ended %+v", e)
	}
	var saved []*convstore.Tasks
	for _, r := range h.records(t) {
		if r, ok := r.(*convstore.Tasks); ok {
			saved = append(saved, r)
		}
	}
	if len(saved) != 2 || len(saved[1].Tasks) != 2 || saved[1].Tasks[0].State != "done" {
		t.Fatalf("saved %+v", saved)
	}
}

// TestTasksReload: a reloaded conversation reports the last list.
func TestTasksReload(t *testing.T) {
	h := newHarness(t, []tool.Tool{tasksTool(t)}, tasksCall("t1", firstList...), tasksCall("t2", lastList...), text("ok"))
	_ = h.conv.Send("fix the parser")
	next[*TurnEnded](t, h)
	loaded := closeAndLoad(t, h)
	if got := TasksOf(loaded); !reflect.DeepEqual(got, lastList) {
		t.Fatalf("TasksOf = %+v, want %+v", got, lastList)
	}
	if got := TasksOf(&convstore.Conversation{}); got != nil {
		t.Fatalf("TasksOf with no list = %+v", got)
	}
}
