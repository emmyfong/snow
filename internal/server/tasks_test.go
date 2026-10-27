package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
	"github.com/emmyfong/snow/pkg/api"
)

// TestTasksInSnapshot: a client that attaches after the task list
// changed gets the list in the snapshot.
func TestTasksInSnapshot(t *testing.T) {
	in := json.RawMessage(`{"tasks":[{"text":"read code","state":"done"},{"text":"fix parser","state":"in_progress"}]}`)
	s, _, c, id := chatInFolder(t, t.TempDir(),
		providertest.Reply{Events: []provider.Event{
			&provider.Start{Model: "model-1"},
			&provider.ToolUse{ID: "t1", Name: "tasks", Input: in},
			&provider.Stop{Reason: provider.StopToolUse},
		}},
		reply("on it"),
	)
	say(c, id, "fix the parser")
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return lastReply(s) == "on it" && idle(s) })

	c.Send(&api.Detach{})
	c2 := connect(t, s, 100, 30)
	c2.Send(&api.Attach{Session: "work"})
	snap := c2.WaitChat(id, func(s *api.ChatSnapshot) bool { return len(s.Tasks) > 0 })
	want := []api.ChatTask{{Text: "read code", State: api.ChatTaskDone}, {Text: "fix parser", State: api.ChatTaskInProgress}}
	if len(snap.Tasks) != 2 || snap.Tasks[0] != want[0] || snap.Tasks[1] != want[1] {
		t.Fatalf("tasks %+v, want %+v", snap.Tasks, want)
	}
}

// TestResumeShowsTasks: a resumed conversation shows the last task list
// saved in its file.
func TestResumeShowsTasks(t *testing.T) {
	dir := t.TempDir()
	_, store, c, id := chatInFolder(t, dir)
	at := time.Now()
	past := saveConversation(t, store, dir, append(pastExchange(),
		&convstore.Tasks{At: at, Tasks: []convstore.TaskItem{{Text: "a", State: "in_progress"}}},
		&convstore.Tasks{At: at, Tasks: []convstore.TaskItem{{Text: "a", State: "done"}, {Text: "b", State: "pending"}}},
	)...)
	chatInputOf(c, id, api.ChatInputResume, func(in *api.ChatInput) { in.Conversation = past })
	got := c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Conversation == past })
	want := []api.ChatTask{{Text: "a", State: api.ChatTaskDone}, {Text: "b", State: api.ChatTaskPending}}
	if len(got.Tasks) != 2 || got.Tasks[0] != want[0] || got.Tasks[1] != want[1] {
		t.Fatalf("tasks %+v, want %+v", got.Tasks, want)
	}
}
