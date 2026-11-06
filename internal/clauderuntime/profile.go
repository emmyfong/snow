// Package clauderuntime runs Claude Code's structured chat protocol in an
// explicitly selected Snow connection. It does not implement model requests.
package clauderuntime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/emmyfong/snow/internal/shell"
)

// AuthMode selects one billing route. A connection never falls back to another.
type AuthMode string

// Supported connection authentication routes.
const (
	Subscription AuthMode = "subscription"
	APIKey       AuthMode = "api-key"
)

var (
	// ErrProfile means the connection identity, location or route is invalid.
	ErrProfile = errors.New("invalid Claude connection")
	// ErrCredential means the selected route has no appropriate explicit key.
	ErrCredential = errors.New("runtime connection credential does not match its route")
)

// Profile identifies a Snow-owned Claude configuration directory. ConfigDir is
// interpreted inside the selected runtime environment, including its WSL distro.
// It must never name the user's standalone Claude directory.
type Profile struct {
	ID        string
	Auth      AuthMode
	ConfigDir string
	// Distro is the WSL distribution the connection runs in; empty for the
	// machine Snow runs on. A WSL ConfigDir is under that user's home, and
	// only the distribution knows the home, so ConfigDir is shown, not used.
	Distro string
}

// NewProfile returns the connection's native configuration location under root.
// It does not create directories or inspect standalone Claude credentials.
func NewProfile(root, id string, auth AuthMode) (Profile, error) {
	if !filepath.IsAbs(root) || !validID(id) || auth != Subscription && auth != APIKey {
		return Profile{}, ErrProfile
	}
	return Profile{ID: id, Auth: auth, ConfigDir: filepath.Join(root, "connections", id, "claude")}, nil
}

// ProfileFor returns c's profile: a native folder under root, or for a WSL
// connection a folder inside its distribution.
func ProfileFor(root string, c Connection) (Profile, error) {
	p, err := NewProfile(root, c.ID, c.Auth)
	if err != nil || c.Distro == "" {
		return p, err
	}
	if strings.ContainsAny(c.Distro, "\x00\r\n") {
		return Profile{}, ErrProfile
	}
	p.Distro, p.ConfigDir = c.Distro, "~/"+wslConfigBase+"/"+c.ID+"/claude"
	return p, nil
}

func validID(id string) bool {
	if len(id) < 1 || len(id) > 64 {
		return false
	}
	for _, ch := range id {
		if ch != '-' && (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') {
			return false
		}
	}
	return true
}

// Prepare creates the private native connection directory. WSL connections must
// prepare their directory inside that distribution instead.
func (p Profile) Prepare() error {
	if p.Distro != "" {
		// The launch script creates it, owner-only, inside the distribution.
		return nil
	}
	if !validID(p.ID) || !filepath.IsAbs(p.ConfigDir) || p.Auth != Subscription && p.Auth != APIKey {
		return ErrProfile
	}
	if err := os.MkdirAll(p.ConfigDir, 0o700); err != nil {
		return fmt.Errorf("create Claude connection directory: %w", err)
	}
	return nil
}

// CredentialID scopes the saved API key to this connection, rather than the
// shared Anthropic provider. Read it with credential.Store.GetStored.
func (p Profile) CredentialID() string { return "claude-connection-" + p.ID }

// Launch applies a native profile to an already resolved program. inherited is
// the environment the process would inherit. The caller supplies a key only for
// the API-key route. No process-wide environment is changed. WSL callers must
// apply equivalent filtering inside the distro, before exec, as well. Runtime
// command construction must also select setting sources; environment filtering
// alone does not isolate workspace credential helpers.
func (p Profile) Launch(l shell.Launch, inherited []string, key string) (shell.Launch, error) {
	l.Hide = slices.Clone(l.Hide)
	if !validID(p.ID) || p.ConfigDir == "" || strings.ContainsAny(p.ConfigDir, "\x00\r\n") {
		return shell.Launch{}, ErrProfile
	}
	if p.Auth != Subscription && p.Auth != APIKey {
		return shell.Launch{}, ErrProfile
	}
	if p.Auth == Subscription && key != "" || p.Auth == APIKey && (key == "" || strings.ContainsAny(key, " \t\r\n\x00")) {
		return shell.Launch{}, ErrCredential
	}
	// Launch.Env bypasses Hide. Filter it too, so an earlier setup stage cannot
	// restore an inherited route or helper after applying this profile.
	var env []string
	for _, entry := range l.Env {
		name, _, _ := strings.Cut(entry, "=")
		if !connectionVar(name) {
			env = append(env, entry)
		}
	}
	for _, entry := range inherited {
		name, _, _ := strings.Cut(entry, "=")
		if connectionVar(name) {
			l.Hide = append(l.Hide, name)
		}
	}
	l.Env = append(env, "CLAUDE_CONFIG_DIR="+p.ConfigDir)
	if p.Auth == APIKey {
		l.Env = append(l.Env, "ANTHROPIC_API_KEY="+key)
	}
	return l, nil
}

func connectionVar(name string) bool {
	name = strings.ToUpper(name)
	if name == gitBashVar {
		return false
	}
	return name == "CLAUDECODE" || strings.HasPrefix(name, "CLAUDE_") || strings.HasPrefix(name, "ANTHROPIC_") ||
		name == bedrockKey
}

// gitBashVar tells Claude Code on Windows where Git Bash is. It chooses no
// route or account, and Claude may not start without it.
const gitBashVar = "CLAUDE_CODE_GIT_BASH_PATH"

// bedrockKey is the Amazon Bedrock gateway's key. The gateway switches start
// with CLAUDE_, but this key does not.
const bedrockKey = "AWS_BEARER_TOKEN_BEDROCK"

// hostAuthVars are inherited variables that would choose a route, a key, or
// a gateway in a standalone Claude. Snow keeps them from every connection
// and names the ones it finds, so the user knows they have no effect.
var hostAuthVars = []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN", "ANTHROPIC_PROFILE",
	"ANTHROPIC_BASE_URL", "ANTHROPIC_CUSTOM_HEADERS", "CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX",
	"CLAUDE_CODE_USE_FOUNDRY", "ANTHROPIC_BEDROCK_BASE_URL", "ANTHROPIC_VERTEX_BASE_URL", "ANTHROPIC_FOUNDRY_API_KEY",
	bedrockKey, "CLAUDE_CONFIG_DIR"}

// IgnoredHostVars returns the names in hostAuthVars that environ sets.
func IgnoredHostVars(environ []string) []string {
	var found []string
	for _, entry := range environ {
		name, value, _ := strings.Cut(entry, "=")
		if value != "" && slices.ContainsFunc(hostAuthVars, func(v string) bool { return strings.EqualFold(v, name) }) {
			found = append(found, strings.ToUpper(name))
		}
	}
	slices.Sort(found)
	return slices.Compact(found)
}
