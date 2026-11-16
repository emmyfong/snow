package server

import (
	"strings"
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

func TestRuntimeApprovalIdentity(t *testing.T) {
	options, _, _ := chatOptions(t)
	options.Chat.Runtime = runtimeOptions(t, "conversation-stop")
	server := New(options)
	t.Cleanup(server.Close)
	client := connect(t, server, 100, 30)
	pane := openChat(t, client)
	useTestRuntime(t, server, client, pane)
	chatInputOf(client, pane, api.ChatInputMessage, func(in *api.ChatInput) { in.Text = "original" })
	waiting := client.WaitChat(pane, func(s *api.ChatSnapshot) bool { return s.Pending != nil })
	agent, id := waiting.Agent, waiting.Pending.ID
	for _, in := range []api.ChatInput{
		{Pane: pane, Agent: "other-generation", Kind: api.ChatInputAnswer, ID: id, Decision: api.ChatDecisionYes},
		{Pane: pane, Agent: agent, Kind: api.ChatInputAnswer, ID: "stale-request", Decision: api.ChatDecisionYes},
		{Pane: pane, Agent: agent, Kind: api.ChatInputAnswer, ID: id, Decision: api.ChatDecisionAlways},
	} {
		client.Send(&in)
		if problem := nextError(t, client); problem.Code != api.CodeCommandFailed {
			t.Fatalf("wrong rejection: %+v", problem)
		}
	}
	client.Send(&api.ChatInput{Pane: pane, Agent: agent, Kind: api.ChatInputAnswer, ID: id, Decision: api.ChatDecisionNo, Text: "do not run"})
	client.WaitChat(pane, func(s *api.ChatSnapshot) bool { return s.Pending == nil })
	client.Send(&api.ChatInput{Pane: pane, Agent: agent, Kind: api.ChatInputAnswer, ID: id, Decision: api.ChatDecisionYes})
	if problem := nextError(t, client); problem.Code != api.CodeCommandFailed {
		t.Fatalf("duplicate approval not rejected: %+v", problem)
	}
	chatInputOf(client, pane, api.ChatInputInterrupt, nil)
	client.WaitChat(pane, func(s *api.ChatSnapshot) bool { return s.State == api.ChatStateStopped })
	client.Send(&api.ChatInput{Pane: pane, Agent: agent, Kind: api.ChatInputAnswer, ID: id, Decision: api.ChatDecisionYes})
	if problem := nextError(t, client); problem.Code != api.CodeCommandFailed {
		t.Fatalf("late approval not rejected: %+v", problem)
	}
	if !strings.Contains(client.ChatText(pane), "original") {
		t.Fatal("stop lost original intent")
	}
}
