package server

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/emmyfong/snow/internal/agent"
	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/store"
	"github.com/emmyfong/snow/pkg/api"
)

// restoredChat is the conversation of a saved chat pane, resumed before its
// session restores: resuming reads the conversation file, which the model
// goroutine must never wait for.
type restoredChat struct {
	conv *agent.Conversation
	// snap is the pane's snapshot; Pane is set when the pane gets its ID.
	snap            api.ChatSnapshot
	folder, profile string
	// pane is the ID that the conversation's events go to. It is 0 until the
	// pane exists, and the events of that time are dropped, as replaySnapshot
	// replays the state they would have shown.
	pane   *atomic.Int64
	notice string // says why the chat is empty, when its conversation could not be resumed
	used   bool   // a pane took conv; owned by the model goroutine
}

// resumeChats resumes the conversation of every saved chat pane in snap,
// keyed by the saved pane ID. A server without chats resumes none, and its
// restore opens the pane as a terminal.
func (s *Server) resumeChats(snap *store.Snapshot) map[int]*restoredChat {
	if snap == nil || s.opts.Chat == nil {
		return nil
	}
	out := map[int]*restoredChat{}
	for _, w := range snap.Windows {
		for _, p := range w.Panes {
			if p.Kind != store.KindChat {
				continue
			}
			if rc := s.resumeSavedChat(p); rc != nil {
				out[p.ID] = rc
			}
		}
	}
	return out
}

// resumeSavedChat resumes the conversation sp names. A conversation that is
// missing, unreadable, gone from its folder, or open elsewhere gives an empty
// new conversation in the pane's folder and a notice, so the rest of the
// session still restores and the file stays as it is. It returns nil when
// even an empty conversation cannot start.
func (s *Server) resumeSavedChat(sp store.Pane) *restoredChat {
	o := s.opts.Chat
	pane := new(atomic.Int64)
	cfgFor := func(dir string) agent.Config {
		return s.chatConfigFor(func() int { return int(pane.Load()) }, sp.Profile, dir)
	}
	loaded, err := o.Store.Load(sp.Conversation)
	if err == nil {
		folder := s.chatFolder(sp.Profile, loaded.Meta.Folder)
		if folder.Distro == "" {
			err = checkFolder(loaded.Meta.Folder)
		}
		if err == nil {
			s.awaitRelease(sp.Conversation)
			cfg := cfgFor(loaded.Meta.Folder)
			cfg.Folder = folder
			var conv *agent.Conversation
			if conv, loaded, err = agent.ResumeLoaded(cfg, loaded); err == nil {
				model, effort, mode := agent.StateOf(loaded)
				return &restoredChat{
					conv: conv, snap: replaySnapshot(0, conv, loaded, model, effort, mode),
					folder: folder.Path, profile: cfg.Shell.Name, pane: pane,
				}
			}
			if errors.Is(err, convstore.ErrInUse) {
				err = errors.New("it is open in another pane")
			}
		}
	}
	notice := fmt.Sprintf("Could not resume the conversation in %s: %v. This chat is empty. The file stays as it is.", s.conversationFile(sp.Conversation), err)
	return s.freshChat(sp, pane, cfgFor(sp.Folder), notice)
}

// conversationFile names the file of conversation id for a notice.
func (s *Server) conversationFile(id string) string {
	if p, err := s.opts.Chat.Store.Path(id); err == nil {
		return p
	}
	return fmt.Sprintf("the conversation %q", id)
}

// freshChat starts an empty conversation for a saved pane whose own could not
// resume. It returns nil if that fails too.
func (s *Server) freshChat(sp store.Pane, pane *atomic.Int64, cfg agent.Config, notice string) *restoredChat {
	var model, effort string
	s.call(func(st *state) { model, effort = st.defaultModel, st.defaultEffort })
	conv, err := agent.Start(cfg, convstore.Meta{Model: model, Effort: effort})
	if err != nil {
		s.log.Warn("restore chat failed", "conversation", sp.Conversation, "err", err)
		return nil
	}
	return &restoredChat{
		conv: conv, snap: newSnapshot(0, conv, model, effort, conv.Mode()),
		folder: cfg.Folder.Path, profile: cfg.Shell.Name, pane: pane, notice: plain(notice),
	}
}

// paneFor builds chat pane id from rc, which the pane then owns.
func (rc *restoredChat) paneFor(id int, profile string) *pane {
	rc.used = true
	rc.pane.Store(int64(id))
	rc.snap.Pane = id
	return &pane{id: id, profile: profile, dir: rc.folder, chat: &chatPane{conv: rc.conv, snap: rc.snap, folder: rc.folder, profile: rc.profile}}
}

// closeUnused closes the conversations that no pane took, for an attach
// that restored nothing: it keeps their files free. It waits, so it runs off
// the model goroutine, after the attach.
func closeUnused(chats map[int]*restoredChat) {
	for _, rc := range chats {
		if !rc.used {
			_ = rc.conv.Close()
		}
	}
}
