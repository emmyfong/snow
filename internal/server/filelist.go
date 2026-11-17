package server

import (
	"context"
	"fmt"

	"github.com/emmyfong/snow/internal/files"
	"github.com/emmyfong/snow/pkg/api"
)

// fileList answers a FileListRequest with the files of the pane's chat
// folder, in walk order. It runs on c's reader goroutine: reading the folder
// takes disk time the model goroutine must not spend.
func (s *Server) fileList(c *client, req *api.FileListRequest) {
	if err := req.Validate(); err != nil {
		s.refuse(c, api.CodeBadCommand, err)
		return
	}
	var folder, profile string
	var found bool
	s.call(func(st *state) {
		if p := st.paneOf(c, req.Pane); p != nil && p.chat != nil {
			folder, profile, found = p.chat.folder, p.chat.profile, true
		}
	})
	if !found {
		s.refuse(c, api.CodeCommandFailed, fmt.Errorf("no chat in pane %d", req.Pane))
		return
	}
	distro, _ := s.distroOf(profile)
	root := files.OSPath(folder, distro)
	paths, cut, err := files.ListFiles(context.Background(), root, api.MaxFileList)
	if err != nil {
		s.reply(c, &api.FileList{Pane: req.Pane, Folder: folder, Err: err.Error()})
		return
	}
	s.reply(c, &api.FileList{Pane: req.Pane, Folder: folder, Files: paths, Cut: cut})
}
