package server

import (
	"fmt"
	"sync/atomic"

	"github.com/emmyfong/snow/internal/agent"
	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/store"
	"github.com/emmyfong/snow/pkg/api"
)

// prestarted is a conversation started on a connection's reader goroutine
// for the chat pane that the next attach or command opens. Starting one
// creates its file and syncs the folder, which the model goroutine must
// never wait for (ADR 0007).
type prestarted struct {
	// profile and dir are the pane arguments it was started for. A pane
	// opened with other arguments does not take it.
	profile, dir string
	chat         *restoredChat // nil when err is set
	err          error         // why the conversation could not start
}

// startConversation starts a conversation, as agent.Start does.
func (s *Server) startConversation(cfg agent.Config, meta convstore.Meta) (*agent.Conversation, error) {
	if start := s.opts.Chat.start; start != nil {
		return start(cfg, meta)
	}
	return agent.Start(cfg, meta)
}

// prestart starts a conversation for a chat pane opened from a pane running
// profile in dir, with model and effort for its next turn.
func (s *Server) prestart(profile, dir, model, effort string) *prestarted {
	pane := new(atomic.Int64)
	cfg := s.chatConfigFor(func() int { return int(pane.Load()) }, profile, dir)
	conv, err := s.startConversation(cfg, convstore.Meta{Model: model, Effort: effort})
	if err != nil {
		return &prestarted{profile: profile, dir: dir, err: err}
	}
	return &prestarted{profile: profile, dir: dir, chat: &restoredChat{
		conv: conv, snap: newSnapshot(0, conv, model, effort, conv.Mode()),
		folder: cfg.Folder.Path, profile: cfg.Shell.Name, pane: pane,
	}}
}

// prestartSession starts the first conversation of the session m creates.
// It returns nil when m creates nothing: the session runs or is saved, or
// the server has no chats.
func (s *Server) prestartSession(m *api.Attach, snap *store.Snapshot) *prestarted {
	if !m.Create || snap != nil || s.opts.Chat == nil || m.Validate() != nil {
		return nil
	}
	var exists bool
	var model, effort string
	s.call(func(st *state) {
		_, running := st.sessions[m.Session]
		_, saved := st.saved[m.Session]
		exists = running || saved
		model, effort = st.defaultModel, st.defaultEffort
	})
	if exists {
		return nil
	}
	return s.prestart("", m.Dir, model, effort)
}

// opensChat reports whether cmd opens a chat pane: a new window or a split
// of kind chat. An agent split opens its own kind of pane.
func opensChat(cmd *api.Command) bool {
	return cmd.Args[api.ArgKind] == api.PaneChat && cmd.Args[api.ArgAgent] == "" &&
		(cmd.Op == api.OpNewWindow || cmd.Op == api.OpSplit)
}

// prestartCommand starts the conversation of the chat pane that cmd from c
// opens, with the profile and folder the command will use. It returns nil
// when cmd opens no chat or cannot run.
func (s *Server) prestartCommand(c *client, cmd *api.Command) *prestarted {
	if s.opts.Chat == nil || !opensChat(cmd) || cmd.Validate() != nil {
		return nil
	}
	var ok bool
	var profile, dir, model, effort string
	s.call(func(st *state) {
		sess, found := st.sessions[c.session]
		if !found || c.dropped {
			return
		}
		from := 0
		if cmd.Op == api.OpSplit {
			if _, p := sess.findPane(cmd.Target); p == nil {
				return
			}
			from = cmd.Target
		}
		profile, dir = st.newPaneArgs(sess, cmd, from)
		model, effort = st.defaultModel, st.defaultEffort
		ok = true
	})
	if !ok {
		return nil
	}
	return s.prestart(profile, dir, model, effort)
}

// takePrestarted returns chat pane id from the prestarted conversation, if
// one was started for profile and dir. ok is false when there is none, and
// the caller starts the conversation itself.
func (st *state) takePrestarted(id int, profile, dir string) (p *pane, ok bool, err error) {
	pre := st.prestarted
	if pre == nil || pre.profile != profile || pre.dir != dir {
		return nil, false, nil
	}
	st.prestarted = nil
	if pre.err != nil {
		return nil, true, fmt.Errorf("start chat: %w", pre.err)
	}
	return pre.chat.paneFor(id, profile), true, nil
}

// withPrestarted runs f on the model goroutine with pre offered to the chat
// pane f opens, then closes pre's conversation if no pane took it.
func (s *Server) withPrestarted(pre *prestarted, f func(st *state)) {
	s.call(func(st *state) {
		st.prestarted = pre
		f(st)
		st.prestarted = nil
	})
	if pre != nil && pre.chat != nil {
		s.closeUnused(map[int]*restoredChat{0: pre.chat})
	}
}
