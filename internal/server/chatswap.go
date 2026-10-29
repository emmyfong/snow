package server

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/emmyfong/snow/internal/agent"
	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/pkg/api"
)

// errStopTurn is the refusal of /new and /resume while a turn runs.
var errStopTurn = errors.New("a turn is running. Press Esc to stop the turn first")

// paneFacts is what a swap needs from the model goroutine.
type paneFacts struct {
	found, running  bool
	conv            *agent.Conversation // the pane's conversation now
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
			found: true, running: p.chat.snap.Running, conv: p.chat.conv, profile: p.profile, dir: p.chat.folder,
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

// busy returns errStopTurn unless the pane's conversation is idle. It asks
// the agent: the pane's snapshot trails the agent by the time an event
// takes, so a message sent a moment ago would not show there yet.
func (f paneFacts) busy() error {
	if f.running || !f.conv.Idle() {
		return errStopTurn
	}
	return nil
}

// newChat puts a new conversation in pane, in the folder and with the model
// of the one it has now.
func (s *Server) newChat(c *client, pane int) error {
	f := s.swapFacts(c, pane, "")
	if !f.found {
		return fmt.Errorf("no chat in pane %d", pane)
	}
	if err := f.busy(); err != nil {
		return err
	}
	cfg := s.chatConfig(pane, f.profile, f.dir)
	conv, err := agent.Start(cfg, convstore.Meta{Model: f.model, Effort: f.effort})
	if err != nil {
		return fmt.Errorf("start chat: %w", err)
	}
	return s.adopt(c, pane, conv, newSnapshot(pane, conv, f.model, f.effort, conv.Mode()), cfg.Folder.Path)
}

// resumeChat puts the saved conversation id in pane. A conversation that
// another pane of the session shows moves focus there instead, so one
// conversation never has two writers.
func (s *Server) resumeChat(c *client, pane int, id string) error {
	f := s.swapFacts(c, pane, id)
	switch {
	case !f.found:
		return fmt.Errorf("no chat in pane %d", pane)
	case f.own:
		return nil
	case f.elsewhere != 0:
		if f.elsewhereIn != c.session {
			return fmt.Errorf("that conversation is open in session %q", f.elsewhereIn)
		}
		s.post(func(st *state) { st.command(c, &api.Command{Op: api.OpFocus, Target: f.elsewhere}) })
		return nil
	}
	if err := f.busy(); err != nil {
		return err
	}
	s.awaitRelease(id)
	loaded, err := s.opts.Chat.Store.Load(id)
	if err != nil {
		return fmt.Errorf("resume: %w", err)
	}
	folder := s.chatFolder(f.profile, loaded.Meta.Folder)
	if folder.Distro == "" {
		if err := checkFolder(loaded.Meta.Folder); err != nil {
			return err
		}
	}
	cfg := s.chatConfig(pane, f.profile, loaded.Meta.Folder)
	cfg.Folder = folder
	conv, loaded, err := agent.ResumeLoaded(cfg, loaded)
	if errors.Is(err, convstore.ErrInUse) {
		return errors.New("that conversation is already open")
	}
	if err != nil {
		return err
	}
	// A mode that stops all questions comes back only in the folder the user
	// chose it for: a file from another folder, or one copied in, starts
	// asking again.
	model, effort, mode := agent.StateOf(loaded)
	notice := ""
	if mode == agent.ModeAuto && loaded.Meta.Folder != f.dir {
		mode = agent.ModeAsk
		// The folder comes last: a narrow status bar cuts the end of a notice.
		notice = "Resumed with the approval mode ask. Type /mode auto to stop the questions. Saved in " + loaded.Meta.Folder + "."
		if err := conv.SetMode(mode); err != nil {
			return errors.Join(err, conv.Close())
		}
	}
	snap := replaySnapshot(pane, conv, loaded, model, effort, mode)
	if err := s.adopt(c, pane, conv, snap, folder.Path); err != nil {
		return err
	}
	if notice != "" {
		s.reply(c, &api.Error{Code: api.CodeNotice, Message: notice})
	}
	return nil
}

// checkFolder checks that dir, the folder a conversation was saved in, is a
// folder it can run in: an absolute path that exists. The chat folder
// falls back to the home folder for any other, which would put the tools
// somewhere the conversation never worked.
func checkFolder(dir string) error {
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("resume: the saved folder %q is not an absolute path", dir)
	}
	fi, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("resume: the folder %s is gone", dir)
	case err != nil:
		return fmt.Errorf("resume: cannot use the folder %s: %w", dir, err)
	case !fi.IsDir():
		return fmt.Errorf("resume: %s is not a folder", dir)
	}
	return nil
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
		before := p.agentNow()
		p.chat = &chatPane{conv: conv, snap: snap, folder: dir, profile: p.chat.profile}
		p.dir = dir
		// The session file names the pane's conversation.
		if sess := st.sessions[c.session]; sess != nil {
			st.save(sess)
		}
		for cl := range st.clients {
			if cl.session == c.session {
				delete(cl.chatLive, pane)
				st.syncChats(cl)
			}
		}
		// A resumed conversation can wait for an answer to its question.
		switch sess := st.sessions[c.session]; {
		case sess != nil && p.agentNow() != before:
			st.agentChanged(sess)
		default:
			st.syncAgents()
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
