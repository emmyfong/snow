package server

import (
	"errors"
	"fmt"
	"time"

	"github.com/emmyfong/snow/internal/agent"
	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/internal/tool"
	"github.com/emmyfong/snow/pkg/api"
)

// Errors about chat panes.
var (
	// ErrNoChat means a chat pane was asked for, but the server was started
	// without Options.Chat.
	ErrNoChat = errors.New("server: chat panes are not set up")
	// ErrNotTerminal means a request that needs a terminal, such as a
	// scrollback page, named a chat pane.
	ErrNotTerminal = errors.New("server: not a terminal pane")
)

// ChatOptions is what chat panes need. cmd/snow builds it once.
type ChatOptions struct {
	Store     *convstore.Store
	Providers *provider.Registry
	Tools     *tool.Registry
	// DefaultModel is the model a new chat starts with, as "provider/model",
	// and its effort. Empty Model means the client asks the user to choose
	// before the first message.
	DefaultModel, DefaultEffort string
	// ProviderStatuses returns the providers the model picker shows, in
	// order, with their titles and whether a key is saved. It may read the
	// OS key store, so callers run it off the state goroutine. Nil lists
	// the registry's providers as connected.
	ProviderStatuses func() []api.ProviderModels
	// SaveDefault saves model and effort as the default for new chats. It
	// runs off the state goroutine. Nil means a default cannot be saved.
	SaveDefault func(model, effort string) error
	// Auth lists the providers and saves or removes their keys. Its calls
	// may read the OS key store, so the server runs them off the state
	// goroutine. Nil refuses auth requests.
	Auth Auth
	// Runtime enables Snow-owned external runtime connections alongside the
	// direct-API loop. All runtime I/O stays off the state goroutine.
	Runtime *RuntimeOptions
	// ModelTTL is how long a fetched model list stays fresh. Zero means
	// DefaultModelTTL.
	ModelTTL time.Duration
	// Now returns the current time. Nil means time.Now.
	Now    func() time.Time
	System string // the system prompt
	// HideEnv names the variables the run tool's commands do not get: the
	// ones that hold provider keys.
	HideEnv []string
	// Shell, when set, is the profile every chat's run tool uses. Unset,
	// a chat uses the profile of the pane it opened from.
	Shell *shell.Profile

	// start starts a new conversation. Nil means agent.Start. Tests set it
	// to hold a start while they watch the model goroutine.
	start func(agent.Config, convstore.Meta) (*agent.Conversation, error)
}

// providerStatuses is the provider list the model picker shows.
func (o *ChatOptions) providerStatuses() []api.ProviderModels {
	if o.ProviderStatuses != nil {
		return o.ProviderStatuses()
	}
	if o.Providers == nil {
		return nil
	}
	names := o.Providers.Names()
	out := make([]api.ProviderModels, len(names))
	for i, name := range names {
		out[i] = api.ProviderModels{Name: name, Title: name, Connected: true}
	}
	return out
}

// modelTTL is how long a fetched model list stays fresh.
func (o *ChatOptions) modelTTL() time.Duration {
	if o.ModelTTL > 0 {
		return o.ModelTTL
	}
	return DefaultModelTTL
}

// now is the current time.
func (o *ChatOptions) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// chatPane is the conversation a chat pane shows. The model goroutine owns
// snap; conv is safe from any goroutine.
type chatPane struct {
	conv    conversation
	closing bool // close waits off the owner goroutine; reject further submissions
	snap    api.ChatSnapshot
	// folder and profile are where the conversation works, as its file
	// records them: the history list shows the conversations of the same
	// folder and profile.
	folder, profile string
	// kept is the conversation the saved session named before a restore could
	// not resume it and gave this pane an empty one. The session keeps naming
	// it until this chat has a message.
	kept string
}

// savedID is the conversation the session file names for the pane.
func (c *chatPane) savedID() string {
	if c.kept != "" && len(c.snap.Entries) == 0 {
		return c.kept
	}
	return c.conv.ID()
}

// startChat starts a conversation in dir for a new chat pane id, opened
// from a pane running profile, with model and effort for its next turn.
// The conversation reports to the model goroutine in order.
func (s *Server) startChat(id int, profile, dir, model, effort string) (*pane, error) {
	o := s.opts.Chat
	if o == nil {
		return nil, ErrNoChat
	}
	cfg := s.chatConfig(id, profile, dir)
	folder, sh := cfg.Folder, cfg.Shell
	conv, err := s.startConversation(cfg, convstore.Meta{Model: model, Effort: effort})
	if err != nil {
		return nil, fmt.Errorf("start chat: %w", err)
	}
	p := &pane{id: id, profile: profile, dir: folder.Path, chat: &chatPane{conv: conv, folder: folder.Path, profile: sh.Name}}
	p.chat.snap = newSnapshot(id, conv, model, effort, conv.Mode())
	return p, nil
}

// newSnapshot is the snapshot of chat pane id showing conv, with no
// transcript and no turn running.
func newSnapshot(id int, conv conversation, model, effort string, mode agent.Mode) api.ChatSnapshot {
	return api.ChatSnapshot{
		Pane: id, Agent: string(conv.Agent()), Conversation: conv.ID(),
		Model: model, Effort: effort, Mode: string(mode), State: api.ChatStateIdle,
	}
}

// send runs f in the model goroutine, after the functions sent before it.
// Unlike post it waits for the model goroutine to take f, so it keeps
// order; it must not run on the model goroutine.
func (s *Server) send(f func(*state)) {
	select {
	case s.requests <- f:
	case <-s.quit:
	}
}
