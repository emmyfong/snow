package main

import (
	"fmt"

	"github.com/emmyfong/snow/internal/config"
	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/internal/server"
	"github.com/emmyfong/snow/internal/signin"
	"github.com/emmyfong/snow/internal/tool"
	"github.com/emmyfong/snow/pkg/api"
)

// chatOptions builds what the server's chat panes need: the conversation
// store, the providers with the saved keys, the built-in tools, the model
// new chats start with, how the model picker learns which providers have a
// key and saves a new default, and how /login and /logout read and change
// their keys.
func chatOptions(cfg config.Config) (*server.ChatOptions, error) {
	dir, err := convstore.DefaultDir()
	if err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}
	store, err := convstore.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}
	credDir, err := credential.DefaultDir()
	if err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}
	path, err := config.DefaultPath()
	if err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}
	ps, err := providers(cfg, path)
	if err != nil {
		return nil, err
	}
	auth := signin.NewAuth(ps, credDir)
	keys := auth.Keys()
	reg, err := signin.Registry(ps, keys)
	if err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}
	model := cfg.Chat.Model
	hide := make([]string, len(ps))
	for i, p := range ps {
		hide[i] = credential.EnvVar(p.Name)
	}
	return &server.ChatOptions{
		Store: store, Providers: reg, Tools: tool.Builtin(), HideEnv: hide,
		DefaultModel: model, DefaultEffort: cfg.Chat.Effort,
		Auth: auth,
		ProviderStatuses: func() []api.ProviderModels {
			statuses := signin.Statuses(ps, keys)
			out := make([]api.ProviderModels, len(statuses))
			for i, s := range statuses {
				out[i] = api.ProviderModels{Name: s.Provider.Name, Title: s.Provider.Title, Connected: s.Connected()}
			}
			return out
		},
		SaveDefault: func(model, effort string) error { return config.SetChatModel(path, model, effort) },
	}, nil
}
