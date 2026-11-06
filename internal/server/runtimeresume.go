package server

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/emmyfong/snow/internal/clauderuntime"
	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/pkg/api"
)

func (s *Server) reopenRuntimeConversation(pane func() int, profile string, loaded *convstore.Conversation) (*reopened, error) {
	if s.opts.Chat.Runtime == nil || loaded.Meta.Runtime.Kind != "claude-code" {
		return nil, fmt.Errorf("%w: runtime connection is unavailable", clauderuntime.ErrControl)
	}
	cfg := s.chatConfigFor(pane, profile, loaded.Meta.Folder)
	if cfg.Folder.Distro == "" {
		if err := checkFolder(loaded.Meta.Folder); err != nil {
			return nil, err
		}
	}
	facts := runtimeFacts{host: cfg.Shell, folder: loaded.Meta.Folder, profile: cfg.Shell.Name}
	connection, err := s.runtimeConnection(facts, loaded.Meta.Runtime.Connection)
	if err != nil {
		return nil, err
	}
	w, err := s.opts.Chat.Store.Writer(loaded.Meta.ID)
	if err != nil {
		return nil, err
	}
	loaded, err = s.opts.Chat.Store.Load(loaded.Meta.ID)
	if err != nil {
		return nil, errors.Join(err, w.Close())
	}
	snapshot := api.ChatSnapshot{Pane: pane(), Conversation: w.ID(), Model: loaded.Meta.Model, Mode: api.ChatModeAsk,
		State: api.ChatStateIdle, Runtime: &api.RuntimeInfo{Kind: "claude-code", Connection: connection.ID,
			Auth: string(connection.Auth), Environment: runtimeEnvironment(connection.Distro)}}
	for _, record := range loaded.Records {
		if record, ok := record.(*convstore.RuntimeEvent); ok {
			var event api.ChatEvent
			if json.Unmarshal(record.Display, &event) != nil {
				return nil, errors.Join(fmt.Errorf("%w: unreadable saved runtime event", clauderuntime.ErrProtocol), w.Close())
			}
			event.Pane = snapshot.Pane
			fitRuntimeEvent(&event)
			snapshot.Apply(&event)
		}
	}
	// A new process must obtain fresh permission identities. An unfinished run
	// is refused by the controller; crash recovery belongs to #264.
	snapshot.Pending, snapshot.Question = nil, nil
	conv, err := s.runtimeConversation(pane, facts, connection, w, snapshot)
	if err != nil {
		return nil, errors.Join(err, w.Close())
	}
	snapshot.Agent = string(conv.Agent())
	return &reopened{conv: conv, runtimeSnapshot: &snapshot, loaded: loaded, folder: cfg.Folder, cfg: cfg}, nil
}
