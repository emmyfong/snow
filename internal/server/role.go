package server

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/emmyfong/snow/internal/private"
	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/pkg/api"
)

var (
	// ErrUnknownRole means agent-new named a role the server does not have.
	ErrUnknownRole = errors.New("server: unknown role")
	// ErrRoleMode means a role asks for a permission mode Snow refuses.
	ErrRoleMode = errors.New("server: role permission mode refused")
	// ErrRoleFolder means the role instructions folder has no path inside
	// WSL, such as a network folder, so claude there cannot read it.
	ErrRoleFolder = errors.New("server: role folder cannot be read from WSL")
)

// Role is an agent role from config.toml. Name is its table name, which
// identifies it; Title is what clients show. Instructions go to the agent
// after its own system prompt and enforce nothing. PermissionMode is the
// Claude Code permission mode the agent starts in, which Claude Code
// enforces; empty leaves Claude Code's default.
type Role struct {
	Name           string
	Title          string
	Instructions   string
	PermissionMode string
}

const (
	// readOnlyMode is the Claude Code permission mode that edits no files
	// and runs no commands.
	readOnlyMode = "plan"
	// bypassMode turns off every Claude Code permission check. config
	// refuses it in a role; the server refuses it too.
	bypassMode = "bypassPermissions"
)

// title is what clients show for r.
func (r Role) title() string {
	if r.Title != "" {
		return r.Title
	}
	return r.Name
}

// info describes r for clients.
func (r Role) info() api.RoleInfo {
	return api.RoleInfo{
		Name: r.Name, Title: r.title(), PermissionMode: r.PermissionMode,
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
	switch r.PermissionMode {
	case "":
	case bypassMode:
		return nil, fmt.Errorf("%w: %s turns off every permission check", ErrRoleMode, r.Name)
	default:
		args = append(args, "--permission-mode", r.PermissionMode)
	}
	if strings.TrimSpace(r.Instructions) == "" {
		return args, nil
	}
	dir := s.opts.RolesDir
	if dir == "" {
		return nil, errors.New("server: no folder for role instructions")
	}
	path, err := writeRoleFile(dir, r)
	if err != nil {
		return nil, fmt.Errorf("role %s: %w", r.Name, err)
	}
	if distro != "" {
		wsl := shell.WSLPath(path)
		if wsl == path {
			return nil, fmt.Errorf("role %s: %w: %s", r.Name, ErrRoleFolder, path)
		}
		path = wsl
	}
	return append(args, "--append-system-prompt-file", path), nil
}

// writeRoleFile writes r's instructions to a file in dir and returns its
// path. The name holds a hash of the role and its text, so a file never
// changes once written: a launch that finds it uses it, and two launches
// that write it at once write the same bytes, each through its own temp
// file.
func writeRoleFile(dir string, r Role) (string, error) {
	path := filepath.Join(dir, roleFile(r))
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if err := private.Dir(dir); err != nil {
		return "", err
	}
	// CreateTemp makes the file with mode 0600 and a name no other launch
	// uses.
	f, err := os.CreateTemp(dir, ".role-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmp := f.Name()
	_, err = f.WriteString(r.Instructions)
	if err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err == nil {
		err = private.Rename(tmp, path)
	}
	if err != nil {
		return "", errors.Join(err, private.Remove(tmp))
	}
	return path, nil
}

// roleFile is the file name for r's instructions: its name with every
// character a file name or cmd may not hold as a dash, then a hash of the
// exact name and the instructions.
func roleFile(r Role) string {
	safe := strings.Map(func(c rune) rune {
		if c < ' ' || strings.ContainsRune(`/\:*?"<>|% !`, c) {
			return '-'
		}
		return c
	}, r.Name)
	sum := sha256.Sum256([]byte(r.Name + "\x00" + r.Instructions))
	return fmt.Sprintf("%s-%x.md", safe, sum[:8])
}
