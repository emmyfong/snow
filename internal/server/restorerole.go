package server

import "github.com/emmyfong/snow/internal/store"

// restoredRole is a saved agent's role, made ready off the model goroutine:
// what clients see, and the Claude Code arguments that apply it again.
type restoredRole struct {
	info Role
	args []string
}

// restoreRoles prepares the role of every saved agent pane in snap, keyed
// by the saved pane ID. A role gone from config.toml, or one that cannot
// apply, is logged and left out: the agent restores without a role, and
// the board shows none, so nothing claims a limit that is not there. It
// writes role files, so it never runs on the model goroutine.
func (s *Server) restoreRoles(snap *store.Snapshot) map[int]restoredRole {
	if snap == nil {
		return nil
	}
	var out map[int]restoredRole
	for _, w := range snap.Windows {
		for _, p := range w.Panes {
			if p.Role == "" {
				continue
			}
			r, err := s.role(p.Role)
			var args []string
			if err == nil {
				distro, _ := s.distroOf(p.Profile)
				args, err = s.roleArgs(r, distro)
			}
			if err != nil {
				s.log.Warn("saved agent restores without its role", "role", p.Role, "err", err)
				continue
			}
			if out == nil {
				out = map[int]restoredRole{}
			}
			out[p.ID] = restoredRole{info: r, args: args}
		}
	}
	return out
}
