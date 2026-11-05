package server

import (
	"errors"
	"fmt"
	"hash/fnv"
	"path/filepath"
	"strings"

	"github.com/emmyfong/snow/internal/private"
	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/pkg/api"
)

// ErrUnknownRole means agent-new named a role the server does not have.
var ErrUnknownRole = errors.New("server: unknown role")

// Role is an agent role from config.toml. Instructions go to the agent
// after its own system prompt and enforce nothing. PermissionMode is the
// Claude Code permission mode the agent starts in, which Claude Code
// enforces; empty leaves Claude Code's default.
type Role struct {
	Name           string
	Instructions   string
	PermissionMode string
}

// readOnlyMode is the Claude Code permission mode that edits no files and
// runs no commands.
const readOnlyMode = "plan"

// info describes r for clients.
func (r Role) info() api.RoleInfo {
	return api.RoleInfo{
		Name: r.Name, PermissionMode: r.PermissionMode,
		HasInstructions: strings.TrimSpace(r.Instructions) != "",
		ReadOnly:        r.PermissionMode == readOnlyMode,
	}
}

// role returns the role called name.
func (s *Server) role(name string) (Role, error) {
	for _, r := range s.opts.Roles {
		if r.Name == name {
			return r, nil
		}
	}
	return Role{}, fmt.Errorf("%w: %s", ErrUnknownRole, name)
}

// roleArgs returns the Claude Code arguments that start an agent in role
// r: its permission mode, and its instructions from a file Snow writes,
// so no shell between Snow and claude reads their text. distro is the WSL
// distribution claude runs in, which reads the file at its Linux path. It
// writes to disk, so it never runs on the model goroutine.
func (s *Server) roleArgs(r Role, distro string) ([]string, error) {
	var args []string
	if r.PermissionMode != "" {
		args = append(args, "--permission-mode", r.PermissionMode)
	}
	if strings.TrimSpace(r.Instructions) == "" {
		return args, nil
	}
	dir := s.opts.RolesDir
	if dir == "" {
		return nil, errors.New("server: no folder for role instructions")
	}
	if err := private.Dir(dir); err != nil {
		return nil, fmt.Errorf("role %s: %w", r.Name, err)
	}
	path := filepath.Join(dir, roleFile(r.Name))
	if err := private.WriteAtomic(path, private.Bytes([]byte(r.Instructions))); err != nil {
		return nil, fmt.Errorf("role %s: %w", r.Name, err)
	}
	if distro != "" {
		path = shell.WSLPath(path)
	}
	return append(args, "--append-system-prompt-file", path), nil
}

// roleFile is the file name for a role's instructions: its name with every
// character a file name or cmd may not hold as a dash, then a hash of the
// exact name, so two names that differ only in those characters get two
// files.
func roleFile(name string) string {
	safe := strings.Map(func(c rune) rune {
		if c < ' ' || strings.ContainsRune(`/\:*?"<>|% !`, c) {
			return '-'
		}
		return c
	}, name)
	h := fnv.New32a()
	_, _ = h.Write([]byte(name)) // a hash.Hash never returns an error
	return fmt.Sprintf("%s-%08x.md", safe, h.Sum32())
}
