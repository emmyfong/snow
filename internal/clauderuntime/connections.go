package clauderuntime

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/emmyfong/snow/internal/private"
)

// Connection binds an opaque identity to one auth route and environment. It
// never contains a credential, CLI token, or path to the standalone profile.
type Connection struct {
	Version int      `json:"version"`
	ID      string   `json:"id"`
	Auth    AuthMode `json:"auth"`
	Distro  string   `json:"distro,omitempty"`
	Name    string   `json:"name"`
}

// CreateConnection saves a new private connection. A missing runtime or failed
// login leaves this profile available for the user to finish, without importing
// a machine account or falling back to API billing.
func CreateConnection(root, distro string, auth AuthMode) (Connection, error) {
	c := Connection{Version: 1, ID: strings.ToLower(rand.Text()), Auth: auth, Distro: distro, Name: "Claude Code"}
	p, err := ProfileFor(root, c)
	if err != nil {
		return Connection{}, err
	}
	if err := p.Prepare(); err != nil {
		return Connection{}, err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return Connection{}, fmt.Errorf("encode connection: %w", err)
	}
	dir := filepath.Join(root, "connections", c.ID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Connection{}, fmt.Errorf("create connection folder: %w", err)
	}
	path := filepath.Join(dir, "connection.json")
	if err := private.WriteAtomic(path, private.Bytes(data)); err != nil {
		return Connection{}, fmt.Errorf("save connection: %w", err)
	}
	return c, nil
}

// LoadConnection reads a connection by opaque identity. Metadata cannot redirect
// its configuration directory or change which identity the caller selected.
func LoadConnection(root, id string) (Connection, error) {
	if !filepath.IsAbs(root) || !validID(id) {
		return Connection{}, ErrProfile
	}
	path := filepath.Join(root, "connections", id, "connection.json")
	f, err := os.Open(path)
	if err != nil {
		return Connection{}, fmt.Errorf("open connection: %w", err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return Connection{}, fmt.Errorf("read connection size: %w", err)
	}
	if info.Size() > 16<<10 {
		return Connection{}, ErrProfile
	}
	var c Connection
	if json.NewDecoder(f).Decode(&c) != nil || c.ID != id || c.Version != 1 ||
		(c.Auth != Subscription && c.Auth != APIKey) {
		return Connection{}, ErrProfile
	}
	return c, nil
}

// ListConnections returns only connections in the selected environment. An
// account from another WSL distribution or the host is never offered as a
// substitute for the current project's connection.
func ListConnections(root, distro string) ([]Connection, error) {
	if !filepath.IsAbs(root) {
		return nil, ErrProfile
	}
	dirs, err := os.ReadDir(filepath.Join(root, "connections"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	var connections []Connection
	for _, dir := range dirs {
		if !dir.IsDir() || !validID(dir.Name()) {
			continue
		}
		c, err := LoadConnection(root, dir.Name())
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if c.Distro == distro {
			connections = append(connections, c)
		}
	}
	return connections, nil
}
