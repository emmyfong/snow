package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

func TestRuntimeSignInStaysInSnowAndDoesNotSaveTranscript(t *testing.T) {
	options, provider, _ := chatOptions(t)
	options.Chat.Runtime = runtimeOptions(t, "login")
	server := New(options)
	t.Cleanup(server.Close)
	client := connect(t, server, 100, 30)
	pane := openChat(t, client)
	chat := useTestRuntime(t, client, pane)
	connection, agent := chat.Runtime.Connection, chat.Agent
	client.Send(&api.RuntimeRequest{Pane: pane, Agent: agent, Action: api.RuntimeLogin, Connection: connection})
	response := runtimeReply(t, client)
	if response.State != "signing-in" || response.LoginPane == 0 {
		t.Fatalf("missing guided sign-in view: %+v", response)
	}
	client.WaitLayout(func(layout *api.Layout) bool {
		for _, p := range shownPanes(layout) {
			if p.ID == response.LoginPane {
				return p.Purpose == "sign-in"
			}
		}
		return false
	})
	client.WaitScreen("private-login-code")
	var saved []byte
	server.call(func(state *state) {
		var err error
		saved, err = json.Marshal(state.snapshot(state.sessions["work"]))
		if err != nil {
			t.Error(err)
		}
	})
	if strings.Contains(string(saved), "private-login-code") || strings.Contains(string(saved), "Claude sign-in") {
		t.Fatal("official login transcript or temporary window entered saved state")
	}
	client.Send(&api.ReadPaneRequest{Caller: pane, Pane: response.LoginPane})
	if refusal := nextError(t, client); !strings.Contains(refusal.Message, "only to the user") {
		t.Fatalf("agent could read sign-in view: %+v", refusal)
	}
	client.Send(&api.Input{Pane: response.LoginPane, Paste: "finish\r"})
	finished := runtimeReply(t, client)
	if finished.State == "error" || len(finished.Connections) != 1 || finished.Connections[0].Status != "signed-in" {
		t.Fatalf("sign-in completion did not refresh actual status: %+v", finished)
	}
	if len(provider.Requests()) != 0 {
		t.Fatal("sign-in submitted a model request")
	}
}

func TestRuntimeSignInCancelLeavesConnectionSignedOut(t *testing.T) {
	options, _, _ := chatOptions(t)
	options.Chat.Runtime = runtimeOptions(t, "login")
	server := New(options)
	t.Cleanup(server.Close)
	client := connect(t, server, 100, 30)
	pane := openChat(t, client)
	chat := useTestRuntime(t, client, pane)
	connection, agent := chat.Runtime.Connection, chat.Agent
	client.Send(&api.RuntimeRequest{Pane: pane, Agent: agent, Action: api.RuntimeLogin, Connection: connection})
	response := runtimeReply(t, client)
	if response.LoginPane == 0 {
		t.Fatalf("missing login view: %+v", response)
	}
	client.Send(&api.Command{Op: api.OpClose, Target: response.LoginPane})
	finished := runtimeReply(t, client)
	if finished.State == "error" || len(finished.Connections) != 1 || finished.Connections[0].Status != "signed-out" {
		t.Fatalf("cancelling fabricated a successful login: %+v", finished)
	}
	client.WaitChat(pane, func(snapshot *api.ChatSnapshot) bool { return !snapshot.Running })
}
