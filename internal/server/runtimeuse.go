package server

import (
	"context"
	"errors"
	"time"

	"github.com/emmyfong/snow/internal/clauderuntime"
	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/pkg/api"
)

func (s *Server) useRuntime(c *client, pane int, facts runtimeFacts, id string) (string, error) {
	connection, err := s.runtimeConnection(facts, id)
	if err != nil {
		return "", err
	}
	if err := s.opts.Chat.Runtime.policy(connection.Distro, connection.Auth); err != nil {
		return "", err
	}
	if facts.snapshot.Runtime != nil && facts.snapshot.Runtime.Connection == id {
		return facts.snapshot.Agent, nil
	}
	if facts.snapshot.Running || !facts.conversation.Idle() {
		return "", errStopTurn
	}
	if facts.snapshot.Turn > 0 {
		return "", errors.New("open a new chat to use a different connection. This conversation stays saved")
	}
	conv, snapshot, err := s.createRuntimeConversation(pane, facts, connection)
	if err != nil {
		return "", err
	}
	if err := s.adoptExpected(c, pane, facts.conversation, conv, snapshot, facts.folder); err != nil {
		return "", err
	}
	return snapshot.Agent, nil
}

func (s *Server) createRuntimeConversation(pane int, facts runtimeFacts, connection clauderuntime.Connection) (*claudeConversation, api.ChatSnapshot, error) {
	w, err := s.opts.Chat.Store.Create(convstore.Meta{Folder: facts.folder, Profile: facts.profile,
		Created: time.Now(), Model: "claude-code/default", Runtime: &convstore.RuntimeMeta{Kind: "claude-code", Connection: connection.ID}})
	if err != nil {
		return nil, api.ChatSnapshot{}, err
	}
	snapshot := api.ChatSnapshot{Pane: pane, Conversation: w.ID(), Model: "claude-code/default", Mode: api.ChatModeAsk,
		State: api.ChatStateIdle, Runtime: &api.RuntimeInfo{Kind: "claude-code", Connection: connection.ID,
			Auth: string(connection.Auth), Environment: runtimeEnvironment(connection.Distro)}}
	runtime, err := s.runtimeConversation(func() int { return pane }, facts, connection, w, snapshot)
	if err != nil {
		return nil, api.ChatSnapshot{}, errors.Join(err, w.Close())
	}
	snapshot.Agent = runtime.runtime.Agent()
	return runtime, snapshot, nil
}

func (s *Server) runtimeConversation(pane func() int, facts runtimeFacts, connection clauderuntime.Connection, writer *convstore.Writer, snapshot api.ChatSnapshot) (*claudeConversation, error) {
	runtime, err := clauderuntime.NewConversation(clauderuntime.ConversationOptions{Writer: writer, Snapshot: snapshot,
		Start: func(ctx context.Context, session string) (*clauderuntime.Client, error) {
			return s.startRuntime(ctx, connection, facts.host, facts.folder, session, pane())
		}, Events: func(event api.ChatEvent) {
			event.Pane = pane()
			if event.Pane == 0 {
				return
			}
			fitRuntimeEvent(&event)
			s.send(func(st *state) { st.chatEvent(&event) })
		},
	})
	if err != nil {
		return nil, err
	}
	return &claudeConversation{runtime: runtime}, nil
}

func fitRuntimeEvent(event *api.ChatEvent) {
	event.Text = capText(event.Text, api.MaxChatMessage)
	if event.Tool != nil {
		event.Tool.Result = capText(event.Tool.Result, api.MaxToolResult)
	}
	fitEvent(event)
}
