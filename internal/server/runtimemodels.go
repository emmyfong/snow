package server

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/emmyfong/snow/internal/clauderuntime"
	"github.com/emmyfong/snow/pkg/api"
)

// runtimeModelList answers a model list request for a Claude Code chat with
// the models its connection's Claude Code offers, and reports whether the
// pane was one. The answer may need Claude to start, so it comes from its
// own goroutine.
func (s *Server) runtimeModelList(c *client, req *api.ModelListRequest) bool {
	var agent, connection string
	s.call(func(st *state) {
		if p := st.paneOf(c, req.Pane); p != nil && p.chat != nil && p.chat.snap.Runtime != nil {
			agent, connection = p.chat.snap.Agent, p.chat.snap.Runtime.Connection
		}
	})
	if agent == "" || s.opts.Chat.Runtime == nil {
		return false
	}
	go func() {
		entry := api.ProviderModels{Name: "claude-code", Title: "Claude Code", Connected: true, Models: []api.ModelInfo{}}
		models, err := s.runtimeModels(c, req.Pane, agent, connection, req.Force)
		if err != nil {
			entry.Err = err.Error()
		} else {
			entry.Models = models
		}
		s.reply(c, &api.ModelList{Pane: req.Pane, Providers: []api.ProviderModels{entry}})
	}()
	return true
}

// runtimeModels returns the models connection's Claude Code offers, from the
// cache unless force. It asks Claude only for its initialize handshake: no
// sign-in and no model request.
func (s *Server) runtimeModels(c *client, pane int, agent, id string, force bool) ([]api.ModelInfo, error) {
	var cached []api.ModelInfo
	s.call(func(st *state) { cached = st.runtimeModels[id] })
	if cached != nil && !force {
		return cached, nil
	}
	facts, err := s.runtimeFacts(c, pane, agent)
	if err != nil {
		return nil, err
	}
	connection, err := s.runtimeConnection(facts, id)
	if err != nil {
		return nil, err
	}
	options := s.opts.Chat.Runtime
	program, err := options.program(facts.host)
	if err != nil {
		return nil, err
	}
	profile, err := clauderuntime.ProfileFor(options.Root, connection)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	models, err := profile.Models(ctx, program, facts.folder)
	if err != nil {
		return nil, fmt.Errorf("ask Claude Code for its models: %w", err)
	}
	s.call(func(st *state) {
		if st.runtimeModels == nil {
			st.runtimeModels = map[string][]api.ModelInfo{}
		}
		st.runtimeModels[id] = models
	})
	return models, nil
}

// runtimeEfforts returns the effort levels a Claude Code model takes, from
// the models a connection reported. Snow learns them when /model opens.
func (s *Server) runtimeEfforts(model string) ([]string, error) {
	var levels []string
	found := false
	s.call(func(st *state) {
		for _, models := range st.runtimeModels {
			if i := slices.IndexFunc(models, func(m api.ModelInfo) bool { return clauderuntime.ModelPrefix+m.ID == model }); i >= 0 {
				levels, found = models[i].Efforts, true
				return
			}
		}
	})
	if !found {
		return nil, errors.New("open /model first, so Snow can read which effort levels Claude Code's models take")
	}
	return levels, nil
}
