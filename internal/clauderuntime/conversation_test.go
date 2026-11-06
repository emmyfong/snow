package clauderuntime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/pkg/api"
)

type conversationHarness struct {
	c                *Conversation
	store            *convstore.Store
	events           chan api.ChatEvent
	starts           atomic.Int32
	last             atomic.Pointer[StartRequest]
	allowStopFailure bool
}

func newConversationHarness(t *testing.T, mode string) *conversationHarness {
	t.Helper()
	store, err := convstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	w, err := store.Create(convstore.Meta{Folder: t.TempDir(), Model: "claude-code/default",
		Runtime: &convstore.RuntimeMeta{Kind: "claude-code", Connection: "selected"}})
	if err != nil {
		t.Fatal(err)
	}
	h := &conversationHarness{store: store, events: make(chan api.ChatEvent, 4096)}
	launch := testLaunch(t, mode)
	h.c, err = NewConversation(ConversationOptions{Writer: w,
		Snapshot: api.ChatSnapshot{Pane: 1, Model: "claude-code/default", Mode: api.ChatModeAsk,
			Runtime: &api.RuntimeInfo{Kind: "claude-code", Connection: "selected", Auth: "subscription"}},
		Start: func(ctx context.Context, request StartRequest) (*Client, error) {
			h.starts.Add(1)
			h.last.Store(&request)
			return Connect(ctx, launch)
		}, Events: func(event api.ChatEvent) { h.events <- event },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := h.c.Close(); err != nil && !h.allowStopFailure {
			t.Error(err)
		}
	})
	return h
}

func TestConversationUnconfirmedTasksRemainVisible(t *testing.T) {
	h := newConversationHarness(t, "conversation-children-unconfirmed")
	h.allowStopFailure = true
	sendConversation(t, h.c, api.ChatInput{Kind: api.ChatInputMessage, Text: "original"})
	waitConversation(t, h.c, func(s api.ChatSnapshot) bool { return len(s.Runtime.Tasks) == 3 })
	sendConversation(t, h.c, api.ChatInput{Kind: api.ChatInputInterrupt})
	deadline := time.Now().Add(10 * time.Second)
	var unconfirmed bool
	for !unconfirmed && time.Now().Before(deadline) {
		select {
		case event := <-h.events:
			unconfirmed = event.Kind == api.ChatEventError && event.Error != nil && strings.Contains(event.Error.Message, "did not confirm")
		case <-time.After(10 * time.Millisecond):
		}
	}
	if !unconfirmed {
		t.Fatal("missing visible unconfirmed task failure")
	}
	if err := h.c.Close(); !errors.Is(err, ErrStop) {
		t.Fatalf("close hid unconfirmed owned work: %v", err)
	}
	state := h.c.Snapshot()
	if !state.Running || state.State != api.ChatStateStopping {
		t.Fatalf("unconfirmed work was labelled stopped: %+v", state)
	}
	if err := h.c.Close(); !errors.Is(err, ErrStop) {
		t.Fatalf("repeated close erased the failure: %v", err)
	}
}

func waitConversation(t *testing.T, c *Conversation, match func(api.ChatSnapshot) bool) api.ChatSnapshot {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		snapshot := c.Snapshot()
		if match(snapshot) {
			return snapshot
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("controller did not reach expected state; last snapshot: %+v", c.Snapshot())
	return api.ChatSnapshot{}
}

func sendConversation(t *testing.T, c *Conversation, input api.ChatInput) {
	t.Helper()
	if err := c.Input(input); err != nil {
		t.Fatal(err)
	}
}

func TestConversationQuestionsApprovalsAndQueuedTurns(t *testing.T) {
	h := newConversationHarness(t, "conversation")
	sendConversation(t, h.c, api.ChatInput{Kind: api.ChatInputMessage, Text: "original"})
	first := waitConversation(t, h.c, func(s api.ChatSnapshot) bool { return s.Question != nil })
	sendConversation(t, h.c, api.ChatInput{Kind: api.ChatInputMessage, Text: "queued"})
	sendConversation(t, h.c, api.ChatInput{Kind: api.ChatInputReply, ID: first.Question.ID, Picked: []string{"A", "B"}})
	second := waitConversation(t, h.c, func(s api.ChatSnapshot) bool { return s.Question != nil && s.Question.ID != first.Question.ID })
	sendConversation(t, h.c, api.ChatInput{Kind: api.ChatInputReply, ID: second.Question.ID, Text: "raw\nfree answer"})
	approval := waitConversation(t, h.c, func(s api.ChatSnapshot) bool { return s.Pending != nil })
	if err := h.c.Input(api.ChatInput{Kind: api.ChatInputAnswer, ID: approval.Pending.ID, Decision: api.ChatDecisionAlways}); !errors.Is(err, ErrControl) {
		t.Fatalf("unsupported Always grant accepted: %v", err)
	}
	sendConversation(t, h.c, api.ChatInput{Kind: api.ChatInputAnswer, ID: approval.Pending.ID, Decision: api.ChatDecisionYes})
	final := waitConversation(t, h.c, func(s api.ChatSnapshot) bool { return s.Turn == 2 && !s.Running && s.State == api.ChatStateIdle })
	if final.Cost != 0.2 || final.Usage.Scope != "main-agent" || final.Usage.CacheRead != 8 || h.starts.Load() != 1 {
		t.Fatalf("incorrect cost, usage or repeated process initialization: %+v, starts %d", final, h.starts.Load())
	}
	if err := h.c.Close(); err != nil {
		t.Fatal(err)
	}
	loaded, err := h.store.Load(h.c.ID())
	if err != nil {
		t.Fatal(err)
	}
	snapshot := api.ChatSnapshot{Pane: 1}
	for _, record := range loaded.Records {
		if record, ok := record.(*convstore.RuntimeEvent); ok {
			var event api.ChatEvent
			if json.Unmarshal(record.Display, &event) != nil {
				t.Fatal("invalid saved display event")
			}
			snapshot.Apply(&event)
		}
	}
	var texts []string
	for _, entry := range snapshot.Entries {
		if entry.Kind == api.ChatEntryAssistant {
			texts = append(texts, entry.Text)
		}
	}
	if strings.Join(texts, "/") != "hello world/hello world" || snapshot.Runtime.Session != "backend-session" {
		t.Fatalf("duplicated text or lost session identity: %q, %+v", texts, snapshot.Runtime)
	}
}

func TestConversationStopKeepsQueuePausedAndInvalidatesRequests(t *testing.T) {
	h := newConversationHarness(t, "conversation-stop")
	sendConversation(t, h.c, api.ChatInput{Kind: api.ChatInputMessage, Text: "original"})
	pending := waitConversation(t, h.c, func(s api.ChatSnapshot) bool { return s.Pending != nil })
	sendConversation(t, h.c, api.ChatInput{Kind: api.ChatInputMessage, Text: "paused draft"})
	sendConversation(t, h.c, api.ChatInput{Kind: api.ChatInputInterrupt})
	stopped := waitConversation(t, h.c, func(s api.ChatSnapshot) bool { return s.State == api.ChatStateStopped })
	if stopped.Running || stopped.Pending != nil || !stopped.QueuePaused || len(stopped.Queue) != 1 || stopped.Queue[0] != "paused draft" {
		t.Fatalf("stop discarded drafts or retained permission: %+v", stopped)
	}
	if err := h.c.Input(api.ChatInput{Kind: api.ChatInputAnswer, ID: pending.Pending.ID, Decision: api.ChatDecisionYes}); !errors.Is(err, ErrRequest) {
		t.Fatalf("stale permission accepted: %v", err)
	}
	if h.starts.Load() != 1 {
		t.Fatal("stopping automatically restarted queued work")
	}
	sendConversation(t, h.c, api.ChatInput{Kind: api.ChatInputResumeQueue})
	waitConversation(t, h.c, func(s api.ChatSnapshot) bool { return s.Pending != nil && s.Turn == 2 })
	if h.starts.Load() != 2 {
		t.Fatal("explicitly released queue did not start a new runtime")
	}
}

func TestConversationStopsOnlySelectedChildTree(t *testing.T) {
	h := newConversationHarness(t, "conversation-children")
	sendConversation(t, h.c, api.ChatInput{Kind: api.ChatInputMessage, Text: "original"})
	waitConversation(t, h.c, func(s api.ChatSnapshot) bool { return len(s.Runtime.Tasks) == 3 })
	if err := h.c.Input(api.ChatInput{Kind: api.ChatInputStopTask, ID: "child", Run: "stale-run"}); !errors.Is(err, ErrRequest) {
		t.Fatalf("stale task generation accepted: %v", err)
	}
	sendConversation(t, h.c, api.ChatInput{Kind: api.ChatInputStopTask, ID: "child", Run: "run-1"})
	state := waitConversation(t, h.c, func(s api.ChatSnapshot) bool {
		for _, task := range s.Runtime.Tasks {
			if task.ID == "child" {
				return terminalTask(task.State)
			}
		}
		return false
	})
	if !state.Running {
		t.Fatal("child stop ended its parent")
	}
	for _, task := range state.Runtime.Tasks {
		if task.ID == "sibling" && task.State != "running" || task.ID == "grandchild" && !terminalTask(task.State) {
			t.Fatalf("child stop affected the wrong ownership set: %+v", state.Runtime.Tasks)
		}
	}
}

func TestConversationAuthenticationFence(t *testing.T) {
	h := newConversationHarness(t, "conversation-stop")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sendConversation(t, h.c, api.ChatInput{Kind: api.ChatInputMessage, Text: "original"})
	waitConversation(t, h.c, func(s api.ChatSnapshot) bool { return s.Pending != nil })
	if err := h.c.PauseForAuth(ctx); !errors.Is(err, ErrControl) {
		t.Fatalf("authentication changed during active work: %v", err)
	}
	if !h.c.Snapshot().Running {
		t.Fatal("rejected authentication change stopped the run")
	}
	sendConversation(t, h.c, api.ChatInput{Kind: api.ChatInputMessage, Text: "keep paused"})
	sendConversation(t, h.c, api.ChatInput{Kind: api.ChatInputInterrupt})
	waitConversation(t, h.c, func(s api.ChatSnapshot) bool { return s.State == api.ChatStateStopped })
	if err := h.c.PauseForAuth(ctx); err != nil {
		t.Fatal(err)
	}
	if err := h.c.Input(api.ChatInput{Kind: api.ChatInputMessage, Text: "must not start"}); !errors.Is(err, ErrControl) {
		t.Fatalf("new submission bypassed authentication fence: %v", err)
	}
	if err := h.c.ResumeAfterAuth(ctx); err != nil {
		t.Fatal(err)
	}
	state := h.c.Snapshot()
	if state.Running || !state.QueuePaused || len(state.Queue) != 1 || h.starts.Load() != 1 {
		t.Fatalf("authentication resumed queued model work: %+v", state)
	}
}

// TestConversationModelSwitch: a Claude Code chat takes only Claude Code's
// models, never while a run is active. A chosen model and effort reach the
// next start, and a live Claude restarts on the same session to take them.
func TestConversationModelSwitch(t *testing.T) {
	h := newConversationHarness(t, "conversation-fast")
	if err := h.c.Input(api.ChatInput{Kind: api.ChatInputModel, Model: "anthropic/claude-opus-5-5"}); err == nil {
		t.Fatal("accepted a model of another provider")
	}
	sendConversation(t, h.c, api.ChatInput{Kind: api.ChatInputModel, Model: "claude-code/haiku"})
	sendConversation(t, h.c, api.ChatInput{Kind: api.ChatInputMessage, Text: "first"})
	waitConversation(t, h.c, func(s api.ChatSnapshot) bool { return s.Turn == 1 && !s.Running })
	if got := h.last.Load(); got == nil || got.Model != "claude-code/haiku" {
		t.Fatalf("first start: %+v, want haiku", got)
	}
	sendConversation(t, h.c, api.ChatInput{Kind: api.ChatInputModel, Model: "claude-code/opus", Effort: "high"})
	if s := h.c.Snapshot(); s.Model != "claude-code/opus" || s.Effort != "high" {
		t.Fatalf("snapshot after switch: %s %s", s.Model, s.Effort)
	}
	sendConversation(t, h.c, api.ChatInput{Kind: api.ChatInputMessage, Text: "second"})
	waitConversation(t, h.c, func(s api.ChatSnapshot) bool { return s.Turn == 2 && !s.Running })
	got := h.last.Load()
	if s := h.c.Snapshot(); s.Model != "claude-code/opus" || s.Effort != "high" {
		t.Fatalf("the run replaced the chosen model: %s %s", s.Model, s.Effort)
	}
	if h.starts.Load() != 2 || got.Model != "claude-code/opus" || got.Effort != "high" || got.Session != "backend-session" {
		t.Fatalf("after switch: %d starts, last %+v; want a restart with opus, high, on the same session", h.starts.Load(), got)
	}
}
