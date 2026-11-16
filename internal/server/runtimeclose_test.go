package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/pkg/api"
)

func TestRuntimeChatCloseWaitsForStop(t *testing.T) {
	for _, op := range []string{api.OpClose, api.OpCloseWindow} {
		t.Run(op, func(t *testing.T) {
			gate := filepath.Join(t.TempDir(), "stop")
			t.Setenv("SNOW_TEST_STOP_GATE", gate)
			options, _, store := chatOptions(t)
			options.Chat.Runtime = runtimeOptions(t, "conversation-children-gated")
			server := New(options)
			t.Cleanup(server.Close)
			client := connect(t, server, 100, 30)
			pane := openChat(t, client)
			started := useTestRuntime(t, server, client, pane)
			chatInputOf(client, pane, api.ChatInputMessage, func(in *api.ChatInput) { in.Text = "original" })
			client.WaitChat(pane, func(s *api.ChatSnapshot) bool { return len(s.Runtime.Tasks) == 3 })
			target := pane
			if op == api.OpCloseWindow {
				server.call(func(st *state) { sess, _ := st.findChat(pane); w, _ := sess.findPane(pane); target = w.index })
			}
			client.Send(&api.Command{Op: op, Target: target})
			eventually(t, "runtime to receive stop", func() bool { _, err := os.Stat(gate + ".entered"); return err == nil })
			present := false
			server.call(func(st *state) { _, p := st.findChat(pane); present = p != nil })
			// Release the real subprocess even when the assertion fails.
			if err := os.WriteFile(gate, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if !present {
				t.Fatal("runtime chat disappeared before its owned tasks acknowledged stop")
			}
			eventually(t, "pane to close after acknowledgement", func() bool {
				server.call(func(st *state) { _, p := st.findChat(pane); present = p != nil })
				return !present
			})
			loaded, err := store.Load(started.Conversation)
			if err != nil {
				t.Fatal(err)
			}
			var savedText, savedSession bool
			for _, record := range loaded.Records {
				event, ok := record.(*convstore.RuntimeEvent)
				if !ok {
					continue
				}
				var display api.ChatEvent
				if err := json.Unmarshal(event.Display, &display); err != nil {
					t.Fatal(err)
				}
				savedText = savedText || event.User == "original"
				savedSession = savedSession || event.Session == "backend-session"
			}
			if !savedText || !savedSession {
				t.Fatalf("closed conversation lost user text or session: text=%v session=%v records=%+v", savedText, savedSession, loaded.Records)
			}
		})
	}
}

func TestRuntimeChatCloseFailureStaysVisible(t *testing.T) {
	options, _, _ := chatOptions(t)
	options.Chat.Runtime = runtimeOptions(t, "conversation-children-unconfirmed")
	server := New(options)
	t.Cleanup(server.Close)
	client := connect(t, server, 100, 30)
	pane := openChat(t, client)
	useTestRuntime(t, server, client, pane)
	chatInputOf(client, pane, api.ChatInputMessage, func(in *api.ChatInput) { in.Text = "original" })
	client.WaitChat(pane, func(s *api.ChatSnapshot) bool { return len(s.Runtime.Tasks) == 3 })
	client.Send(&api.Command{Op: api.OpClose, Target: pane})
	failed := client.WaitChat(pane, func(s *api.ChatSnapshot) bool {
		for _, entry := range s.Entries {
			if entry.Kind == api.ChatEntryError {
				return true
			}
		}
		return false
	})
	if failed.State != api.ChatStateStopping || !failed.Running {
		t.Fatalf("unconfirmed stop labelled finished: %+v", failed)
	}
	present := false
	server.call(func(st *state) { _, p := st.findChat(pane); present = p != nil })
	if !present {
		t.Fatal("failed runtime stop lost its pane")
	}
}

func TestStopPreservesUnrelatedRuntimeWork(t *testing.T) {
	options, _, _ := chatOptions(t)
	options.Chat.Runtime = runtimeOptions(t, "conversation-children")
	server := New(options)
	t.Cleanup(server.Close)
	client := connect(t, server, 100, 30)
	first := openChat(t, client)
	useTestRuntime(t, server, client, first)
	chatInputOf(client, first, api.ChatInputMessage, func(in *api.ChatInput) { in.Text = "first" })
	client.WaitChat(first, func(s *api.ChatSnapshot) bool { return len(s.Runtime.Tasks) == 3 })
	client.Send(&api.Command{Op: api.OpNewWindow, Args: map[string]string{api.ArgKind: api.PaneChat}})
	layout := nextLayout(t, client, func(l *api.Layout) bool { return chatIn(l) != 0 && chatIn(l) != first })
	second := chatIn(layout)
	client.WaitChat(second, func(s *api.ChatSnapshot) bool { return s.Agent != "" })
	useTestRuntime(t, server, client, second)
	chatInputOf(client, second, api.ChatInputMessage, func(in *api.ChatInput) { in.Text = "independent" })
	client.WaitChat(second, func(s *api.ChatSnapshot) bool { return len(s.Runtime.Tasks) == 3 })
	chatInputOf(client, first, api.ChatInputInterrupt, nil)
	client.WaitChat(first, func(s *api.ChatSnapshot) bool { return s.State == api.ChatStateStopped })
	server.call(func(st *state) {
		_, p := st.findChat(second)
		if p == nil || !p.chat.snap.Running {
			t.Error("stopping one runtime ended independent work")
		}
		if p != nil {
			for _, task := range p.chat.snap.Runtime.Tasks {
				if task.State != "running" {
					t.Errorf("independent task changed: %+v", task)
				}
			}
		}
	})
}

func TestRuntimeWindowCloseKeepsNewPane(t *testing.T) {
	gate := filepath.Join(t.TempDir(), "stop")
	t.Setenv("SNOW_TEST_STOP_GATE", gate)
	options, _, _ := chatOptions(t)
	options.Chat.Runtime = runtimeOptions(t, "conversation-children-gated")
	server := New(options)
	t.Cleanup(server.Close)
	client := connect(t, server, 100, 30)
	pane := openChat(t, client)
	useTestRuntime(t, server, client, pane)
	chatInputOf(client, pane, api.ChatInputMessage, func(in *api.ChatInput) { in.Text = "original" })
	client.WaitChat(pane, func(s *api.ChatSnapshot) bool { return len(s.Runtime.Tasks) == 3 })
	window := 0
	server.call(func(st *state) { sess, _ := st.findChat(pane); w, _ := sess.findPane(pane); window = w.index })
	client.Send(&api.Command{Op: api.OpCloseWindow, Target: window})
	eventually(t, "runtime stop to start", func() bool { _, err := os.Stat(gate + ".entered"); return err == nil })
	client.Send(&api.Command{Op: api.OpSplit, Target: pane, Args: map[string]string{api.ArgKind: api.PaneChat, api.ArgDir: api.DirRight}})
	layout := nextLayout(t, client, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	added := 0
	for _, p := range shownPanes(layout) {
		if p.ID != pane {
			added = p.ID
		}
	}
	if err := os.WriteFile(gate, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	eventually(t, "original pane to close", func() bool {
		gone := false
		server.call(func(st *state) { _, p := st.findChat(pane); gone = p == nil })
		return gone
	})
	server.call(func(st *state) {
		_, p := st.findChat(added)
		if p == nil || p.chat.closing {
			t.Error("new pane was closed with the old window")
		}
	})
}

func TestRuntimeWindowCloseKeepsSameCountReplacement(t *testing.T) {
	gate := filepath.Join(t.TempDir(), "stop")
	t.Setenv("SNOW_TEST_STOP_GATE", gate)
	options, _, _ := chatOptions(t)
	options.Chat.Runtime = runtimeOptions(t, "conversation-children-gated")
	server := New(options)
	t.Cleanup(server.Close)
	client := connect(t, server, 100, 30)
	first := openChat(t, client)
	useTestRuntime(t, server, client, first)
	chatInputOf(client, first, api.ChatInputMessage, func(in *api.ChatInput) { in.Text = "first" })
	client.WaitChat(first, func(s *api.ChatSnapshot) bool { return len(s.Runtime.Tasks) == 3 })
	client.Send(&api.Command{Op: api.OpSplit, Target: first, Args: map[string]string{api.ArgKind: api.PaneChat, api.ArgDir: api.DirRight}})
	layout := nextLayout(t, client, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	var second int
	for _, p := range shownPanes(layout) {
		if p.ID != first {
			second = p.ID
		}
	}
	useTestRuntime(t, server, client, second)
	chatInputOf(client, second, api.ChatInputMessage, func(in *api.ChatInput) { in.Text = "second" })
	client.WaitChat(second, func(s *api.ChatSnapshot) bool { return len(s.Runtime.Tasks) == 3 })
	window := 0
	server.call(func(st *state) { sess, _ := st.findChat(first); w, _ := sess.findPane(first); window = w.index })
	client.Send(&api.Command{Op: api.OpCloseWindow, Target: window})
	eventually(t, "both runtime stops to start", func() bool { _, err := os.Stat(gate + ".entered"); return err == nil })
	client.Send(&api.Command{Op: api.OpLayout, Target: first, Args: map[string]string{api.ArgName: "focus"}})
	_ = nextLayout(t, client, func(l *api.Layout) bool { return len(shownPanes(l)) == 1 })
	client.Send(&api.Command{Op: api.OpSplit, Target: first, Args: map[string]string{api.ArgKind: api.PaneChat, api.ArgDir: api.DirRight}})
	layout = nextLayout(t, client, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	var replacement int
	for _, p := range shownPanes(layout) {
		if p.ID != first {
			replacement = p.ID
		}
	}
	if replacement == 0 {
		t.Fatal("split did not create a replacement pane")
	}
	// The target window now has two panes, matching its original count, but
	// one original chat moved to another window and a new pane took its place.
	if err := os.WriteFile(gate, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	eventually(t, "original runtimes to close", func() bool {
		removed := true
		server.call(func(st *state) {
			_, firstPane := st.sessions["work"].findPane(first)
			_, secondPane := st.sessions["work"].findPane(second)
			_, replacementPane := st.sessions["work"].findPane(replacement)
			removed = firstPane == nil && secondPane == nil && replacementPane != nil
		})
		return removed
	})
}

func TestRuntimeWindowCloseReconcilesChangedPane(t *testing.T) {
	for _, replace := range []bool{false, true} {
		name := "removed"
		if replace {
			name = "new conversation"
		}
		t.Run(name, func(t *testing.T) {
			gate := filepath.Join(t.TempDir(), "stop")
			t.Setenv("SNOW_TEST_STOP_GATE", gate)
			options, _, _ := chatOptions(t)
			options.Chat.Runtime = runtimeOptions(t, "conversation-children-gated")
			server := New(options)
			t.Cleanup(server.Close)
			t.Cleanup(func() {
				if err := os.WriteFile(gate, nil, 0o600); err != nil {
					t.Error(err)
				}
			})
			client := connect(t, server, 100, 30)
			runtimePane := openChat(t, client)
			useTestRuntime(t, server, client, runtimePane)
			chatInputOf(client, runtimePane, api.ChatInputMessage, func(in *api.ChatInput) { in.Text = "original" })
			client.WaitChat(runtimePane, func(s *api.ChatSnapshot) bool { return len(s.Runtime.Tasks) == 3 })
			client.Send(&api.Command{Op: api.OpSplit, Target: runtimePane, Args: map[string]string{api.ArgKind: api.PaneChat, api.ArgDir: api.DirRight}})
			layout := nextLayout(t, client, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
			other := 0
			for _, p := range shownPanes(layout) {
				if p.ID != runtimePane {
					other = p.ID
				}
			}
			original := client.WaitChat(other, func(s *api.ChatSnapshot) bool { return s.Agent != "" }).Agent
			window := 0
			server.call(func(st *state) {
				sess, _ := st.findChat(runtimePane)
				w, _ := sess.findPane(runtimePane)
				window = w.index
			})
			client.Send(&api.Command{Op: api.OpCloseWindow, Target: window})
			eventually(t, "runtime stop to start", func() bool { _, err := os.Stat(gate + ".entered"); return err == nil })
			replacement := ""
			if replace {
				chatInputOf(client, other, api.ChatInputNew, nil)
				replacement = client.WaitChat(other, func(s *api.ChatSnapshot) bool { return s.Agent != original }).Agent
			} else {
				client.Send(&api.Command{Op: api.OpClose, Target: other})
				_ = nextLayout(t, client, func(l *api.Layout) bool { return len(shownPanes(l)) == 1 })
			}
			if err := os.WriteFile(gate, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			eventually(t, "remaining original runtime to close", func() bool {
				removed := false
				server.call(func(st *state) { _, p := st.findChat(runtimePane); removed = p == nil })
				return removed
			})
			server.call(func(st *state) {
				if replace {
					_, p := st.findChat(other)
					if p == nil || p.chat.snap.Agent != replacement {
						t.Error("window close removed the replacement conversation")
					}
				}
			})
		})
	}
}
