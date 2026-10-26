package server

import (
	"encoding/json"
	"testing"

	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
	"github.com/emmyfong/snow/internal/tool"
	"github.com/emmyfong/snow/pkg/api"
)

// TestTasksInSnapshot: a client that attaches after the task list
// changed gets the list in the snapshot.
func TestTasksInSnapshot(t *testing.T) {
	dir := t.TempDir()
	in := json.RawMessage(`{"tasks":[{"text":"read code","state":"done"},{"text":"fix parser","state":"in_progress"}]}`)
	opts, _, _ := chatOptions(t,
		providertest.Reply{Events: []provider.Event{
			&provider.Start{Model: "model-1"},
			&provider.ToolUse{ID: "t1", Name: "tasks", Input: in},
			&provider.Stop{Reason: provider.StopToolUse},
		}},
		reply("on it"),
	)
	opts.Chat.Tools = tool.Builtin()
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 100, 30)
	c.Send(&api.Attach{Session: "work", Create: true, Dir: dir})
	l := nextLayout(t, c, func(*api.Layout) bool { return true })
	c.Send(&api.Command{Op: api.OpSplit, Target: shownPanes(l)[0].ID, Args: map[string]string{
		api.ArgDir: api.DirRight, api.ArgKind: api.PaneChat, api.ArgFolder: dir,
	}})
	id := chatIn(nextLayout(t, c, func(l *api.Layout) bool { return chatIn(l) != 0 }))
	c.WaitChat(id, func(*api.ChatSnapshot) bool { return true })
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
