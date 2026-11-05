package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/emmyfong/snow/internal/config"
	"github.com/emmyfong/snow/internal/server"
)

// roleOptions turns the [role.<name>] tables into the server's roles, in
// name order, and returns the folder a launch writes role instructions
// to: ~/.snow/roles.
func roleOptions(cfg config.Config) ([]server.Role, string, error) {
	roles := make([]server.Role, 0, len(cfg.Roles))
	for _, name := range slices.Sorted(maps.Keys(cfg.Roles)) {
		r := cfg.Roles[name]
		roles = append(roles, server.Role{Name: r.TitleOf(name), Instructions: r.Instructions, PermissionMode: r.PermissionMode})
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return roles, "", fmt.Errorf("find home folder: %w", err)
	}
	return roles, filepath.Join(home, ".snow", "roles"), nil
}
