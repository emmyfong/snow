package server

import (
	"fmt"

	"github.com/emmyfong/snow/internal/agent"
	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/tool"
)

// reopened is a saved conversation reopened for a chat pane.
type reopened struct {
	conv *agent.Conversation
	// loaded is the file as read after the writer claim, which is the one to
	// show: an earlier pane may have saved more since the first read.
	loaded *convstore.Conversation
	folder tool.Folder
	cfg    agent.Config
}

// reopenConversation reopens the saved conversation id for a pane running
// profile, as a resume and a restore both need. It waits for a closing pane
// to let go of the file, reads it, checks that its folder is one it can run
// in, and takes the writer claim. The events of the conversation go to the
// pane that pane returns. It never runs on the model goroutine: it waits for
// the disk.
func (s *Server) reopenConversation(pane func() int, profile, id string) (*reopened, error) {
	s.awaitRelease(id)
	loaded, err := s.opts.Chat.Store.Load(id)
	if err != nil {
		return nil, fmt.Errorf("load the conversation: %w", err)
	}
	folder := s.chatFolder(profile, loaded.Meta.Folder)
	if folder.Distro == "" {
		if err := checkFolder(loaded.Meta.Folder); err != nil {
			return nil, err
		}
	}
	cfg := s.chatConfigFor(pane, profile, loaded.Meta.Folder)
	cfg.Folder = folder
	conv, loaded, err := agent.ResumeLoaded(cfg, loaded)
	if err != nil {
		return nil, err
	}
	return &reopened{conv: conv, loaded: loaded, folder: folder, cfg: cfg}, nil
}
