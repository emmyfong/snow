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

// CreateConnection saves a new private connection. It imports no machine
// account and never falls back to API billing. A caller that adds one for
// the user removes it again with RemoveConnection when its first sign-in
// or key fails.
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

// RemoveConnection deletes a connection's folder: its metadata and, on
// this machine, its Claude sign-in. A WSL connection's sign-in lives in
// its distribution; RemoveLaunch deletes that first. Saved chats are kept
// elsewhere and stay.
func RemoveConnection(root, id string) error {
	if !filepath.IsAbs(root) || !validID(id) {
		return ErrProfile
	}
	if err := os.RemoveAll(filepath.Join(root, "connections", id)); err != nil {
		return fmt.Errorf("remove connection: %w", err)
	}
	return nil
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
// substitute for the current project's connection. Saved connections Snow
// cannot read come back apart, with the reason.
func ListConnections(root, distro string) ([]Connection, []Unreadable, error) {
	if !filepath.IsAbs(root) {
		return nil, nil, ErrProfile
	}
	dirs, err := os.ReadDir(filepath.Join(root, "connections"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("list connections: %w", err)
	}
	var connections []Connection
	var unreadable []Unreadable
	for _, dir := range dirs {
		if !dir.IsDir() || !validID(dir.Name()) {
			continue
		}
		c, err := LoadConnection(root, dir.Name())
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			// Its environment is unknown, so every environment lists it.
			unreadable = append(unreadable, Unreadable{ID: dir.Name(), Err: err})
			continue
		}
		if c.Distro == distro {
			connections = append(connections, c)
		}
	}
	return connections, unreadable, nil
}

// Unreadable is a saved connection Snow cannot read. One broken file must
// not hide the other connections.
type Unreadable struct {
	ID  string
	Err error
}
