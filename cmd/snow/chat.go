package main

import (
	"fmt"
	"log/slog"

	"github.com/emmyfong/snow/internal/config"
	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/internal/server"
	"github.com/emmyfong/snow/internal/signin"
	"github.com/emmyfong/snow/internal/tool"
)

// chatOptions builds what the server's chat panes need: the conversation
// store, the providers with the saved keys, the built-in tools, and the
// model new chats start with. The default model is chosen once, here, so
// opening a chat never waits for the OS key store.
func chatOptions(cfg config.Config, log *slog.Logger) (*server.ChatOptions, error) {
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
	keys := credential.New(credDir, func(msg string) { log.Info(msg) })
	ps := signin.Providers()
	reg, err := signin.Registry(ps, keys)
	if err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}
	model := cfg.Chat.Model
	if model == "" {
		model = signin.DefaultModel(ps, keys)
	}
	hide := make([]string, len(ps))
	for i, p := range ps {
		hide[i] = credential.EnvVar(p.Name)
	}
	return &server.ChatOptions{
		Store: store, Providers: reg, Tools: tool.Builtin(), HideEnv: hide,
		Model: func() (string, string) { return model, cfg.Chat.Effort },
	}, nil
}
