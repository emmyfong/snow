package clauderuntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/emmyfong/snow/internal/shell"
)

// ErrSignIn means the selected connection has no active authentication. It does
// not authorize using a machine account or a different billing route.
var ErrSignIn = errors.New("connection needs sign-in")

// AuthStatus is authentication evidence reported by the selected Claude profile.
// It does not prove model availability, quota, or transport compatibility.
type AuthStatus struct {
	SignedIn bool
	Method   string
}

// CheckAuth reads local status using the same profile and route as chat. It
// never submits a model prompt or reads a standalone CLI configuration.
func (p Profile) CheckAuth(ctx context.Context, program shell.Profile, folder, key string) (AuthStatus, error) {
	launch, err := p.AuthLaunch(program, folder, key, "status")
	if err != nil {
		return AuthStatus{}, err
	}
	var output, diagnostics statusOutput
	code, err := launch.RunIO(ctx, &output, &diagnostics)
	if err != nil {
		return AuthStatus{}, err
	}
	if output.overflow {
		return AuthStatus{}, fmt.Errorf("%w: authentication status exceeds limit", ErrProtocol)
	}
	status, err := parseAuthStatus(output.Bytes(), code, p)
	if err != nil {
		if said := lastLine(diagnostics.String(), key); said != "" {
			return AuthStatus{}, fmt.Errorf("%w. Claude said: %s", err, said)
		}
	}
	return status, err
}

// keyLike matches text shaped like an Anthropic key, which a diagnostic
// line must never carry into a status message.
var keyLike = regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]+`)

// lastLine returns the last non-empty line of a status command's
// diagnostics, at most 200 characters, with key and anything key-shaped
// hidden. Status diagnostics hold no tool data, unlike a chat's.
func lastLine(text, key string) string {
	// wsl.exe writes its own errors, such as an unknown distribution, in
	// UTF-16; without the zero bytes the ASCII text reads plainly.
	text = strings.ReplaceAll(text, "\x00", "")
	lines := strings.Split(strings.TrimSpace(text), "\n")
	line := strings.TrimSpace(lines[len(lines)-1])
	if key != "" {
		line = strings.ReplaceAll(line, key, "[key hidden]")
	}
	line = keyLike.ReplaceAllString(line, "[key hidden]")
	if runes := []rune(line); len(runes) > 200 {
		line = string(runes[:200]) + "…"
	}
	return line
}

func parseAuthStatus(data []byte, code int, profile Profile) (AuthStatus, error) {
	var wire struct {
		LoggedIn        *bool  `json:"loggedIn"`
		AuthMethod      string `json:"authMethod"`
		ConfigDirectory string `json:"configDirectory"`
	}
	if (code != 0 && code != 1) || json.Unmarshal(data, &wire) != nil || wire.LoggedIn == nil ||
		!profile.configDirMatches(wire.ConfigDirectory) {
		return AuthStatus{}, fmt.Errorf("%w: cannot verify selected profile authentication", ErrProtocol)
	}
	if !*wire.LoggedIn {
		if code != 1 || wire.AuthMethod != "none" {
			return AuthStatus{}, fmt.Errorf("%w: inconsistent signed-out status", ErrProtocol)
		}
		return AuthStatus{Method: "none"}, nil
	}
	if code != 0 {
		return AuthStatus{}, fmt.Errorf("%w: inconsistent signed-in status", ErrProtocol)
	}
	if profile.Auth == Subscription && wire.AuthMethod != "claude.ai" || profile.Auth == APIKey && wire.AuthMethod != "api_key" {
		return AuthStatus{}, ErrCredential
	}
	return AuthStatus{SignedIn: true, Method: wire.AuthMethod}, nil
}

type statusOutput struct {
	bytes.Buffer
	overflow bool
}

// Write keeps data up to 64 KiB and notes when more arrived.
func (b *statusOutput) Write(data []byte) (int, error) {
	const limit = 64 << 10
	n := min(len(data), limit-b.Len())
	if n < len(data) {
		b.overflow = true
	}
	_, err := b.Buffer.Write(data[:n])
	return len(data), err
}

// Connect starts the structured worker and requires an initialization response
// before any prompt can be sent. Failed initialization closes the owned process.
func Connect(ctx context.Context, launch shell.Launch) (*Client, error) {
	stream, err := StartStream(launch)
	if err != nil {
		return nil, err
	}
	client := NewClient(stream)
	if _, err := client.Initialize(ctx); err != nil {
		return nil, errors.Join(err, client.Stop())
	}
	return client, nil
}
