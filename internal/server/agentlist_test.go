package server

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
	"github.com/emmyfong/snow/internal/tool"
	"github.com/emmyfong/snow/pkg/api"
)

// nextAgents waits until c's last agent list satisfies ok.
func nextAgents(t *testing.T, c *apitest.Client, ok func([]api.AgentInfo) bool) []api.AgentInfo {
	t.Helper()
	return c.WaitAgents(ok)
}

// agentOf returns the entry of pane in list, and whether it is there.
func agentOf(list []api.AgentInfo, pane int) (api.AgentInfo, bool) {
	for _, a := range list {
		if a.Pane == pane {
			return a, true
		}
	}
	return api.AgentInfo{}, false
}

// hasAgentState returns a check that pane is listed in state.
func hasAgentState(pane int, state string) func([]api.AgentInfo) bool {
	return func(l []api.AgentInfo) bool { a, ok := agentOf(l, pane); return ok && a.State == state }
}

// noAgent returns a check that pane is not listed.
func noAgent(pane int) func([]api.AgentInfo) bool {
	return func(l []api.AgentInfo) bool { _, ok := agentOf(l, pane); return !ok }
}

// agentStates reads agent lists until pane reaches last, and returns the
// states pane went through, each once.
func agentStates(t *testing.T, c *apitest.Client, pane int, last string) []string {
	t.Helper()
	var seen []string
	nextAgents(t, c, func(l []api.AgentInfo) bool {
		a, ok := agentOf(l, pane)
		if !ok {
			return false
		}
		if len(seen) == 0 || seen[len(seen)-1] != a.State {
			seen = append(seen, a.State)
		}
		return a.State == last && len(seen) > 1
	})
	return seen
}

// TestAgentListOnAttach: a client that connects gets the list at once, and
// a chat's turn shows as working, then idle.
func TestAgentListOnAttach(t *testing.T) {
	s, _, _ := newChatServer(t, reply("hi"))
	c := connect(t, s, 100, 30)
	nextAgents(t, c, func(l []api.AgentInfo) bool { return len(l) == 0 })
	id := openChat(t, c)
	nextAgents(t, c, hasAgentState(id, api.AgentIdle))

	late := connect(t, s, 80, 24)
	list := nextAgents(t, late, func([]api.AgentInfo) bool { return true })
	a, ok := agentOf(list, id)
	if !ok || a.Kind != api.AgentKindChat || a.Session != "work" || a.State != api.AgentIdle ||
		a.Model != chatModel || a.Folder == "" || a.Since.IsZero() || a.WindowName == "" {
		t.Fatalf("a new client's list %+v, want the idle chat %d", list, id)
	}

	say(c, id, "go")
	if got := agentStates(t, c, id, api.AgentIdle); strings.Join(got, ",") != "idle,working,idle" {
		t.Fatalf("states %v, want idle, working, idle", got)
	}
}

// TestRegistryStateFollowsEvents: a chat's turn start, approval request,
// answer, turn end, and failed turn each move its entry, and since moves
// only when the state does.
func TestRegistryStateFollowsEvents(t *testing.T) {
	dir := t.TempDir()
	_, _, c, id := chatInFolder(t, dir,
		providertest.Reply{Events: []provider.Event{
			&provider.Start{Model: "model-1"},
			&provider.ToolUse{ID: "t1", Name: "write", Input: json.RawMessage(`{"path":"out.txt","content":"hi\n"}`)},
			&provider.Stop{Reason: provider.StopToolUse},
		}},
		reply("written"),
		providertest.Reply{Err: errors.New("network down")},
	)
	start := nextAgents(t, c, hasAgentState(id, api.AgentIdle))
	idleSince := mustAgent(t, start, id).Since

	say(c, id, "write it")
	waiting := nextAgents(t, c, hasAgentState(id, api.AgentNeedsYou))
	if a := mustAgent(t, waiting, id); a.Waits != api.AgentWaitsApproval || !a.Since.After(idleSince) {
		t.Fatalf("waiting agent %+v, want it to wait for an approval since after %v", a, idleSince)
	}
	snap := c.Chat(id)
	c.Send(&api.ChatInput{Pane: id, Agent: snap.Agent, Kind: api.ChatInputAnswer, ID: "t1", Decision: api.ChatDecisionYes})
	if got := agentStates(t, c, id, api.AgentIdle); strings.Join(got, ",") != "needs-you,working,idle" {
		t.Fatalf("states after the answer %v, want needs-you, working, idle", got)
	}

	say(c, id, "again")
	if got := agentStates(t, c, id, api.AgentIdle); strings.Join(got, ",") != "idle,working,idle" {
		t.Fatalf("states of a failed turn %v, want idle, working, idle", got)
	}
}

// mustAgent returns pane's entry in list or fails the test.
func mustAgent(t *testing.T, list []api.AgentInfo, pane int) api.AgentInfo {
	t.Helper()
	a, ok := agentOf(list, pane)
	if !ok {
		t.Fatalf("pane %d not in %+v", pane, list)
	}
	return a
}

// TestAgentListFollowsHooks: a Claude Code pane is listed from its start,
// takes each state its hooks report, and a repeated state sends nothing.
func TestAgentListFollowsHooks(t *testing.T) {
	s, _, home, claude, _ := claudePane(t)
	first := nextAgents(t, home, func(l []api.AgentInfo) bool { _, ok := agentOf(l, claude); return ok })
	if a := mustAgent(t, first, claude); a.Kind != api.AgentKindClaudeCode || a.State != "" || a.Profile != "Claude Code" {
		t.Fatalf("new Claude Code pane %+v, want kind claude-code and no state yet", a)
	}
	t0 := time.Now()
	var since time.Time
	for i, state := range []string{api.AgentWorking, api.AgentNeedsYou, api.AgentIdle} {
		// Windows' monotonic clock ticks about every 15.6 ms, so two
		// changes closer than that can get the same Since.
		time.Sleep(20 * time.Millisecond)
		hook(t, s, claude, state, t0.Add(time.Duration(i)*time.Millisecond))
		a := mustAgent(t, nextAgents(t, home, hasAgentState(claude, state)), claude)
		if !a.Since.After(since) {
			t.Fatalf("%s since %v, want after %v", state, a.Since, since)
		}
		since = a.Since
	}
	hook(t, s, claude, api.AgentIdle, t0.Add(time.Second))
	expectNoAgentList(t, home)
}

// expectNoAgentList fails when c gets an agent list soon.
func expectNoAgentList(t *testing.T, c *apitest.Client) {
	t.Helper()
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		if m, _ := c.Next(50 * time.Millisecond); m != nil {
			if l, ok := m.(*api.AgentList); ok {
				t.Fatalf("unexpected agent list %+v", l.Agents)
			}
		}
	}
}

// TestRegistryEntryLeavesWithPane: closing an agent's pane removes its
// entry, and so does ending its session.
func TestRegistryEntryLeavesWithPane(t *testing.T) {
	s, c, home, claude, _ := claudePane(t)
	nextAgents(t, home, func(l []api.AgentInfo) bool { _, ok := agentOf(l, claude); return ok })
	c.Send(&api.Command{Op: api.OpClose, Target: claude})
	nextAgents(t, home, noAgent(claude))

	other := connect(t, s, 80, 24)
	other.Send(&api.Attach{Session: "other", Create: true})
	pane := nextLayout(t, other, func(*api.Layout) bool { return true }).Windows[0].Panes[0].ID
	hook(t, s, pane, api.AgentWorking, time.Now())
	nextAgents(t, home, hasAgentState(pane, api.AgentWorking))
	other.Send(&api.KillSession{Session: "other"})
	nextAgents(t, home, noAgent(pane))
}

// TestAgentListPlainShellNotListed: a shell pane is no agent until a hook
// reports from it, as when the user starts claude by hand, and stops being
// one when the hook clears its state.
func TestAgentListPlainShellNotListed(t *testing.T) {
	s := newTestServer(t)
	_, first := attachNoPrompt(t, s)
	home := connect(t, s, 80, 24)
	if list := nextAgents(t, home, func([]api.AgentInfo) bool { return true }); len(list) != 0 {
		t.Fatalf("list %+v for a session with only a shell, want none", list)
	}
	now := time.Now()
	hook(t, s, first, api.AgentWorking, now)
	if a := mustAgent(t, nextAgents(t, home, hasAgentState(first, api.AgentWorking)), first); a.Kind != api.AgentKindClaudeCode {
		t.Fatalf("hooked shell pane %+v, want kind claude-code", a)
	}
	hook(t, s, first, "", now.Add(time.Millisecond))
	nextAgents(t, home, noAgent(first))
}

// TestAgentListTwoConversations: two chat panes are two entries.
func TestAgentListTwoConversations(t *testing.T) {
	s, _, _ := newChatServer(t)
	c := connect(t, s, 100, 30)
	first := openChat(t, c)
	second := openSecondChat(t, c, first)
	nextAgents(t, c, func(l []api.AgentInfo) bool {
		a, okA := agentOf(l, first)
		b, okB := agentOf(l, second)
		return len(l) == 2 && okA && okB && a.Kind == api.AgentKindChat && b.Kind == api.AgentKindChat
	})
}

// TestAgentListFewSends: streamed text changes no entry, so a turn sends
// a list only when the state changes.
func TestAgentListFewSends(t *testing.T) {
	deltas := make([]string, 12)
	for i := range deltas {
		deltas[i] = "word "
	}
	s, _, _ := newChatServer(t, reply(deltas...))
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	nextAgents(t, c, hasAgentState(id, api.AgentIdle))
	say(c, id, "go")
	lists := 0
	deadline := time.Now().Add(apitest.Wait)
	for time.Now().Before(deadline) {
		m, _ := c.Next(time.Second)
		if _, ok := m.(*api.AgentList); ok {
			lists++
		}
		if snap := c.Chat(id); snap != nil && lastReply(snap) == strings.Repeat("word ", 12) && idle(snap) {
			break
		}
	}
	c.Drain(200 * time.Millisecond)
	if lists > 3 {
		t.Fatalf("%d agent lists for one turn of 12 text deltas, want one per state change", lists)
	}
}

// TestAgentListChatWaitsQuestion: a chat that asks the user a question
// says so in its entry.
func TestAgentListChatWaitsQuestion(t *testing.T) {
	opts, _, _ := chatOptions(t, askReply("q1"))
	opts.Chat.Tools = tool.Builtin()
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	say(c, id, "pick")
	a := mustAgent(t, nextAgents(t, c, hasAgentState(id, api.AgentNeedsYou)), id)
	if a.Waits != api.AgentWaitsQuestion {
		t.Fatalf("asking chat %+v, want it to wait for a question", a)
	}
}
