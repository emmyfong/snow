package server

import (
	"github.com/emmyfong/snow/internal/agent"
	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/internal/tool"
)

// chatFolder returns the folder a chat opened from a pane running profile
// works in: dir if it is usable, else the home folder. WSL folders live in
// their distribution, out of reach of a host file stat.
func (s *Server) chatFolder(profile, dir string) tool.Folder {
	if distro, ok := s.profileDistro(profile); ok {
		if linux, ok := distroFolder(distro, dir); ok {
			return tool.Folder{Path: linux, Distro: distro}
		}
	}
	return tool.Folder{Path: usableDir(dir)}
}

// chatConfig is the agent config of a conversation in chat pane id. Its
// events reach the pane by its ID, and chatEvent drops the ones of a
// conversation the pane replaced.
func (s *Server) chatConfig(id int, profile, dir string) agent.Config {
	return s.chatConfigFor(func() int { return id }, profile, dir)
}

// chatConfigFor is chatConfig for a pane whose ID is not known yet, as with
// a restored session: pane returns it, or 0 while the pane does not exist,
// and events of that time are dropped.
func (s *Server) chatConfigFor(pane func() int, profile, dir string) agent.Config {
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
			id := pane()
			if id == 0 {
				return
			}
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
