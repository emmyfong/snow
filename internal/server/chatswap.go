package server

import (
	"errors"
	"fmt"
	"os"

	"github.com/emmyfong/snow/internal/agent"
	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/pkg/api"
)

// errStopTurn is the refusal of /new and /resume while a turn runs.
var errStopTurn = errors.New("a turn is running. Press Esc to stop the turn first")

// paneFacts is what a swap needs from the model goroutine.
type paneFacts struct {
	found, running  bool
	profile, dir    string
	model, effort   string
	elsewhere       int    // pane that shows the wanted conversation, 0 if none
	elsewhereIn     string // its session
	own             bool   // the wanted conversation is this pane's
	oldConversation string
}

// swapFacts reads the facts for a swap of pane's conversation. want is the
// conversation /resume wants, or empty.
func (s *Server) swapFacts(c *client, pane int, want string) (f paneFacts) {
	s.call(func(st *state) {
		p := st.paneOf(c, pane)
		if p == nil || p.chat == nil {
			return
		}
		f = paneFacts{
			found: true, running: p.chat.snap.Running, profile: p.profile, dir: p.chat.folder,
			model: p.chat.snap.Model, effort: p.chat.snap.Effort, oldConversation: p.chat.snap.Conversation,
		}
		if want == "" {
			return
		}
		for _, sess := range st.sessions {
			for _, w := range sess.windows {
				for id, q := range w.panes {
					if q.chat != nil && q.chat.snap.Conversation == want {
						f.elsewhere, f.elsewhereIn, f.own = id, sess.name, id == pane
					}
				}
			}
		}
	})
	return f
}

// newChat puts a new conversation in pane, in the folder and with the model
// of the one it has now.
func (s *Server) newChat(c *client, pane int) error {
	f := s.swapFacts(c, pane, "")
	switch {
	case !f.found:
		return fmt.Errorf("no chat in pane %d", pane)
	case f.running:
		return errStopTurn
	}
	cfg := s.chatConfig(pane, f.profile, f.dir)
	conv, err := agent.Start(cfg, convstore.Meta{Model: f.model, Effort: f.effort})
	if err != nil {
		return fmt.Errorf("start chat: %w", err)
	}
	snap := api.ChatSnapshot{
		Pane: pane, Agent: string(conv.Agent()), Conversation: conv.ID(),
		Model: f.model, Effort: f.effort, Mode: string(conv.Mode()), State: api.ChatStateIdle,
	}
	return s.adopt(c, pane, conv, snap, cfg.Folder.Path)
}

// resumeChat puts the saved conversation id in pane. A conversation that
// another pane of the session shows moves focus there instead, so one
// conversation never has two writers.
func (s *Server) resumeChat(c *client, pane int, id string) error {
	f := s.swapFacts(c, pane, id)
	switch {
	case !f.found:
		return fmt.Errorf("no chat in pane %d", pane)
	case f.running:
		return errStopTurn
	case f.own:
		return nil
	case f.elsewhere != 0:
		if f.elsewhereIn != c.session {
			return fmt.Errorf("that conversation is open in session %q", f.elsewhereIn)
		}
		s.post(func(st *state) { st.command(c, &api.Command{Op: api.OpFocus, Target: f.elsewhere}) })
		return nil
	}
	o := s.opts.Chat
	loaded, err := o.Store.Load(id)
	if err != nil {
		return fmt.Errorf("resume: %w", err)
	}
	folder := s.chatFolder(f.profile, loaded.Meta.Folder)
	if _, err := os.Stat(folder.Path); err != nil && folder.Distro == "" {
		return fmt.Errorf("resume: the folder %s is gone", loaded.Meta.Folder)
	}
	cfg := s.chatConfig(pane, f.profile, loaded.Meta.Folder)
	cfg.Folder = folder
	conv, err := agent.ResumeLoaded(cfg, loaded)
	if errors.Is(err, convstore.ErrInUse) {
		return errors.New("that conversation is open in another Snow")
	}
	if err != nil {
		return err
	}
	return s.adopt(c, pane, conv, replaySnapshot(pane, conv, loaded), folder.Path)
}

// adopt makes conv the conversation of pane and sends every client of the
// session its snapshot. The old conversation closes off the model
// goroutine; if the pane is gone or started a turn meanwhile, conv closes
// instead.
func (s *Server) adopt(c *client, pane int, conv *agent.Conversation, snap api.ChatSnapshot, dir string) error {
	var old *agent.Conversation
	var err error
	s.call(func(st *state) {
		p := st.paneOf(c, pane)
		switch {
		case p == nil || p.chat == nil:
			err = fmt.Errorf("no chat in pane %d", pane)
			return
		case p.chat.snap.Running:
			err = errStopTurn
			return
		}
		old = p.chat.conv
		p.chat = &chatPane{conv: conv, snap: snap, folder: dir, profile: p.chat.profile}
		p.dir = dir
		for cl := range st.clients {
			if cl.session == c.session {
				delete(cl.chatLive, pane)
				st.syncChats(cl)
			}
		}
	})
	if err != nil {
		return errors.Join(err, conv.Close())
	}
	if old == nil {
		return conv.Close()
	}
	if err := old.Close(); err != nil && !errors.Is(err, agent.ErrClosed) {
		s.log.Warn("close replaced conversation", "pane", pane, "err", err)
	}
	return nil
}
