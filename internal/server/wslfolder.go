package server

import (
	"path"
	"strings"

	"github.com/emmyfong/snow/internal/shell"
)

// distroFolder returns the Linux folder that dir names in the WSL distribution
// distro. ok is false when dir is neither a Linux path nor a WSL UNC path for
// distro. Distribution names compare without case.
func distroFolder(distro, dir string) (linux string, ok bool) {
	if path.IsAbs(dir) {
		return dir, true
	}
	uncDistro, linux, ok := shell.UNCToWSL(dir)
	if ok && strings.EqualFold(uncDistro, distro) {
		return linux, true
	}
	return "", false
}

// profileDistro returns the WSL distribution that profile runs. An empty
// profile means the default profile, which is the first in the profile set.
// ok is false when the profile set or distribution mapping is unavailable.
func (s *Server) profileDistro(profile string) (distro string, ok bool) {
	if s.launch.shell == nil || s.launch.distro == nil {
		return "", false
	}
	if profile == "" {
		p, found := s.launch.shell("")
		if !found {
			return "", false
		}
		profile = p.Name
	}
	return s.distroOf(profile)
}
