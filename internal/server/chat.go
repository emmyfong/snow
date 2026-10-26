package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
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
	conv *agent.Conversation
	snap api.ChatSnapshot
	// folder and profile are where the conversation works, as its file
	// records them: the history list shows the conversations of the same
	// folder and profile.
	folder, profile string
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
	conv, err := agent.Start(cfg, convstore.Meta{Model: model, Effort: effort})
	if err != nil {
		return nil, fmt.Errorf("start chat: %w", err)
	}
	p := &pane{id: id, profile: profile, dir: folder.Path, chat: &chatPane{conv: conv, folder: folder.Path, profile: sh.Name}}
	p.chat.snap = newSnapshot(id, conv, model, effort, conv.Mode())
	return p, nil
}

// newSnapshot is the snapshot of chat pane id showing conv, with no
// transcript and no turn running.
func newSnapshot(id int, conv *agent.Conversation, model, effort string, mode agent.Mode) api.ChatSnapshot {
	return api.ChatSnapshot{
		Pane: id, Agent: string(conv.Agent()), Conversation: conv.ID(),
		Model: model, Effort: effort, Mode: string(mode), State: api.ChatStateIdle,
	}
}

// chatFolder returns the folder a chat opened from a pane running profile
// works in: dir if it is usable, else the home folder. A WSL profile's
// folder lives in its distribution, out of reach of a stat.
func (s *Server) chatFolder(profile, dir string) tool.Folder {
	if distro, ok := s.distroOf(profile); ok && path.IsAbs(dir) {
		return tool.Folder{Path: dir, Distro: distro}
	}
	return tool.Folder{Path: usableDir(dir)}
}

// chatConfig is the agent config of a conversation in chat pane id. Its
// events reach the pane by its ID, and chatEvent drops the ones of a
// conversation the pane replaced.
func (s *Server) chatConfig(id int, profile, dir string) agent.Config {
	o := s.opts.Chat
	sh := shell.Profile{}
	if o.Shell != nil {
		sh = *o.Shell
	} else if s.launch.shell != nil {
		sh, _ = s.launch.shell(profile)
	}
	return agent.Config{
		Store: o.Store, Providers: o.Providers, Tools: o.Tools, Folder: s.chatFolder(profile, dir), Shell: sh,
		HideEnv: o.HideEnv, System: o.System,
		Events: func(e agent.Event) {
			if ce := chatEvent(id, e); ce != nil {
				s.send(func(st *state) { st.chatEvent(ce) })
			}
		},
	}
}

// distroOf returns the WSL distribution profile runs, if it is a WSL
// profile.
func (s *Server) distroOf(profile string) (string, bool) {
	if s.launch.distro == nil {
		return "", false
	}
	return s.launch.distro(profile)
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

// findChat returns the session and chat pane with ID id, or nils.
func (st *state) findChat(id int) (*session, *pane) {
	for _, sess := range st.sessions {
		if _, p := sess.findPane(id); p != nil && p.chat != nil {
			return sess, p
		}
	}
	return nil, nil
}

// chatEvent folds e into its pane's snapshot and sends it to the clients
// of the pane's session. An event for a pane that is gone is dropped.
func (st *state) chatEvent(e *api.ChatEvent) {
	sess, p := st.findChat(e.Pane)
	if p == nil {
		return
	}
	if e.Agent != p.chat.snap.Agent {
		return // an event of a conversation this pane replaced
	}
	p.chat.snap.Apply(e)
	for c := range st.clients {
		if c.session != sess.name || !c.chatLive[e.Pane] {
			continue
		}
		// A client that falls behind skips events and later gets a
		// snapshot instead, so it never loses text and never stalls the
		// model goroutine.
		if len(c.out) >= cap(c.out)/2 {
			delete(c.chatLive, e.Pane)
			continue
		}
		st.enqueue(c, e)
	}
}

// syncChats sends c a snapshot of each chat pane of its session it is not
// in step with: a new pane, a new attach, or a client that fell behind and
// now has room again.
func (st *state) syncChats(c *client) {
	sess, ok := st.sessions[c.session]
	if !ok || c.dropped || len(c.out) >= cap(c.out)/2 {
		return
	}
	for _, w := range sess.windows {
		for id, p := range w.panes {
			if p.chat == nil || c.chatLive[id] {
				continue
			}
			c.chatLive[id] = true
			st.enqueue(c, fitSnapshot(p.chat.snap))
		}
	}
}

// snapshotBudget is how large an encoded snapshot may be: a frame, less
// room for growth between the check and the write.
const snapshotBudget = api.MaxFrameSize * 3 / 4

// fitSnapshot returns a copy of snap whose encoding fits snapshotBudget,
// leaving out its oldest entries if needed.
func fitSnapshot(snap api.ChatSnapshot) *api.ChatSnapshot {
	out := snap
	out.Entries = nil
	base, err := json.Marshal(out)
	if err != nil {
		return &out
	}
	size, keep := len(base), len(snap.Entries)
	for keep > 0 {
		b, err := json.Marshal(snap.Entries[keep-1])
		if err != nil || size+len(b)+1 > snapshotBudget {
			break
		}
		size += len(b) + 1
		keep--
	}
	out.Earlier = snap.Earlier + keep
	out.Entries = append([]api.ChatEntry(nil), snap.Entries[keep:]...)
	return &out
}
