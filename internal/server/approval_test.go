package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
	"github.com/emmyfong/snow/internal/tool"
	"github.com/emmyfong/snow/pkg/api"
)

// TestApprovalEndToEnd: in ask mode a write waits for the user; the
// pending approval holds the diff, survives a detach, and the answer sent
// as a ChatInput lets the tool run.
func TestApprovalEndToEnd(t *testing.T) {
	dir := t.TempDir()
	opts, _, _ := chatOptions(t,
		providertest.Reply{Events: []provider.Event{
			&provider.Start{Model: "model-1"},
			&provider.ToolUse{ID: "t1", Name: "write", Input: json.RawMessage(`{"path":"out.txt","content":"hello\n"}`)},
			&provider.Stop{Reason: provider.StopToolUse},
		}},
		reply("written"),
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
	say(c, id, "write it")

	waiting := c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Pending != nil && s.State == api.ChatStateApproval })
	if p := waiting.Pending; p.ID != "t1" || p.Tool != "write" || p.Diff == nil || p.Diff.New != "hello\n" {
		t.Fatalf("pending %+v", p)
	}
	if _, err := os.Stat(filepath.Join(dir, "out.txt")); err == nil {
		t.Fatal("the tool ran before the answer")
	}

	// A client that attaches again sees the same request.
	c.Send(&api.Detach{})
	c2 := connect(t, s, 100, 30)
	c2.Send(&api.Attach{Session: "work"})
	again := c2.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Pending != nil })
	if again.Pending.ID != "t1" || !again.Running {
		t.Fatalf("after attach: %+v", again)
	}

	c2.Send(&api.ChatInput{Pane: id, Agent: again.Agent, Kind: api.ChatInputAnswer, ID: "t1", Decision: api.ChatDecisionYes})
	done := c2.WaitChat(id, func(s *api.ChatSnapshot) bool { return lastReply(s) == "written" && idle(s) })
	if done.Pending != nil {
		t.Fatalf("pending left: %+v", done.Pending)
	}
	got, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	if err != nil || string(got) != "hello\n" {
		t.Fatalf("file %q, %v", got, err)
	}
	var cell *api.ChatTool
	for _, e := range done.Entries {
		if e.Tool != nil {
			cell = e.Tool
		}
	}
	if cell == nil || !cell.Done || cell.IsError {
		t.Fatalf("tool cell %+v", cell)
	}
}
