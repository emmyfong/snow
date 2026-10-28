package server

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
	"github.com/emmyfong/snow/pkg/api"
)

// writeReply is a reply that asks to write a file with tool call id.
func writeReply(id string) providertest.Reply {
	return providertest.Reply{Events: []provider.Event{
		&provider.Start{Model: "model-1"},
		&provider.ToolUse{ID: id, Name: "write", Input: json.RawMessage(`{"path":"` + id + `.txt","content":"hi\n"}`)},
		&provider.Stop{Reason: provider.StopToolUse},
	}}
}

// twoWaitingChats opens two chats in session "work" and has each ask for
// an approval, the first before the second. It returns the client, the
// two pane IDs, and the agent list once both wait.
func twoWaitingChats(t *testing.T) (*apitest.Client, [2]int, []api.AgentInfo) {
	t.Helper()
	_, _, c, first := chatInFolder(t, t.TempDir(), writeReply("t1"), writeReply("t2"), reply("one"), reply("two"))
	second := openSecondChat(t, c, first)
	say(c, first, "write one")
	nextAgents(t, c, hasAgentState(first, api.AgentNeedsYou))
	say(c, second, "write two")
	list := nextAgents(t, c, func(l []api.AgentInfo) bool {
		return hasAgentState(first, api.AgentNeedsYou)(l) && hasAgentState(second, api.AgentNeedsYou)(l)
	})
	return c, [2]int{first, second}, list
}

// TestInboxListsAllAgents: two agents that each wait for an approval are
// two needs-you entries, and since says which asked first.
func TestInboxListsAllAgents(t *testing.T) {
	_, ids, list := twoWaitingChats(t)
	a, b := mustAgent(t, list, ids[0]), mustAgent(t, list, ids[1])
	for _, x := range []api.AgentInfo{a, b} {
		if x.State != api.AgentNeedsYou || x.Waits != api.AgentWaitsApproval {
			t.Fatalf("agent %+v, want it waiting for an approval", x)
		}
	}
	if !a.Since.Before(b.Since) {
		t.Fatalf("first since %v, second %v; want the first to wait longer", a.Since, b.Since)
	}
}

// TestInboxItemLeavesOnAnswer: answering one agent takes it out of the
// needs-you entries, back to working; the other still waits.
func TestInboxItemLeavesOnAnswer(t *testing.T) {
	c, ids, _ := twoWaitingChats(t)
	snap := c.Chat(ids[0])
	c.Send(&api.ChatInput{Pane: ids[0], Agent: snap.Agent, Kind: api.ChatInputAnswer, ID: "t1", Decision: api.ChatDecisionYes})
	if got := agentStates(t, c, ids[0], api.AgentIdle); strings.Join(got, ",") != "needs-you,working,idle" {
		t.Fatalf("states after the answer %v, want needs-you, working, idle", got)
	}
	if a := mustAgent(t, c.Agents(), ids[1]); a.State != api.AgentNeedsYou {
		t.Fatalf("the other agent %+v, want it still waiting", a)
	}
}

// TestInboxSurvivesRestart: a conversation Snow stopped while a question
// waited, as a server restart does, is listed as needing the user again
// once a chat opens it.
func TestInboxSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	_, store, c, id := chatInFolder(t, dir)
	at := time.Now()
	in := json.RawMessage(`{"question":"Which store?","options":[{"label":"A"},{"label":"B"}]}`)
	past := saveConversation(t, store, dir,
		&convstore.User{At: at, Text: "pick a store"},
		&convstore.Assistant{At: at, Stop: convstore.StopToolUse, Blocks: []convstore.Block{{Kind: convstore.BlockToolUse, ID: "q1", Name: "ask", Input: in}}},
		&convstore.Question{At: at, ToolUseID: "q1", Text: "Which store?", Options: []convstore.QuestionOption{{Label: "A"}, {Label: "B"}}},
	)
	nextAgents(t, c, hasAgentState(id, api.AgentIdle))
	chatInputOf(c, id, api.ChatInputResume, func(in *api.ChatInput) { in.Conversation = past })
	a := mustAgent(t, nextAgents(t, c, hasAgentState(id, api.AgentNeedsYou)), id)
	if a.Waits != api.AgentWaitsQuestion {
		t.Fatalf("resumed agent %+v, want it waiting for its question", a)
	}
}

// TestAttachFocusesPane: a client that attaches to answer an agent in
// another session gets that agent's window shown and its pane focused.
func TestAttachFocusesPane(t *testing.T) {
	s := newTestServer(t)
	c, first := attachNoPrompt(t, s)
	c.Send(&api.Command{Op: api.OpNewWindow, Args: map[string]string{}})
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(l.Windows) == 2 })
	if l.Active == 0 {
		t.Fatalf("new window not shown: %+v", l)
	}

	other := connect(t, s, 81, 24)
	other.Send(&api.Attach{Session: "work", Pane: first})
	got := nextLayout(t, other, func(*api.Layout) bool { return true })
	if got.Active != 0 {
		t.Fatalf("shown window %d, want 0, the one with pane %d", got.Active, first)
	}
	if p := shownPanes(got); len(p) != 1 || p[0].ID != first || !p[0].Focused {
		t.Fatalf("panes %+v, want pane %d focused", p, first)
	}

	other.Send(&api.Detach{})
	other.Send(&api.Attach{Session: "work", Pane: 999})
	if got := nextLayout(t, other, func(*api.Layout) bool { return true }); got.Active != 0 {
		t.Fatalf("a pane the session lacks moved the shown window to %d", got.Active)
	}
}
