package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/emmyfong/snow/internal/clauderuntime"
	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/internal/transport"
)

// RuntimeOptions configures Snow-owned Claude connections. Keys are read only
// through saved-profile operations; inherited provider keys never select a route.
type RuntimeOptions struct {
	Root string
	Keys *credential.Store
	// Program optionally resolves the installed external executable. Tests use
	// a real subprocess protocol fixture; normal builds use shell.FindClaude.
	Program func(shell.Profile) (shell.Profile, error)
	// PolicyDir optionally replaces the managed settings folder of an
	// environment. Tests use it; normal builds read the system folder.
	PolicyDir func(distro string) string
}

// policy refuses a route that the environment's managed policy rules out.
func (o *RuntimeOptions) policy(distro string, auth clauderuntime.AuthMode) error {
	dir := clauderuntime.PolicyDir(distro)
	if o.PolicyDir != nil {
		dir = o.PolicyDir(distro)
	}
	p, err := clauderuntime.ReadPolicy(dir)
	if err != nil {
		return err
	}
	return p.Check(auth)
}

func (o *RuntimeOptions) program(host shell.Profile) (shell.Profile, error) {
	if o.Program != nil {
		return o.Program(host)
	}
	program, found := shell.FindClaude(host)
	if !found {
		return shell.Profile{}, fmt.Errorf("%w: install Claude Code in the selected environment", shell.ErrProgramNotFound)
	}
	return program, nil
}

func (o *RuntimeOptions) profile(connection clauderuntime.Connection) (clauderuntime.Profile, string, error) {
	profile, err := clauderuntime.ProfileFor(o.Root, connection)
	if err != nil {
		return clauderuntime.Profile{}, "", err
	}
	if connection.Auth != clauderuntime.APIKey {
		return profile, "", nil
	}
	if o.Keys == nil {
		return clauderuntime.Profile{}, "", clauderuntime.ErrSignIn
	}
	key, _, err := o.Keys.GetStored(profile.CredentialID())
	if err != nil {
		if errors.Is(err, credential.ErrNoKey) {
			return clauderuntime.Profile{}, "", fmt.Errorf("%w: save an API key for this connection", clauderuntime.ErrSignIn)
		}
		return clauderuntime.Profile{}, "", fmt.Errorf("read connection key: %w", err)
	}
	return profile, key.Reveal(), nil
}

func (s *Server) startRuntime(ctx context.Context, connection clauderuntime.Connection, host shell.Profile, folder, session string, pane int) (*clauderuntime.Client, error) {
	options := s.opts.Chat.Runtime
	if connection.Distro != host.Distro {
		return nil, fmt.Errorf("%w: connection belongs to a different runtime environment", clauderuntime.ErrProfile)
	}
	program, err := options.program(host)
	if err != nil {
		return nil, err
	}
	if err := options.policy(connection.Distro, connection.Auth); err != nil {
		return nil, err
	}
	profile, key, err := options.profile(connection)
	if err != nil {
		return nil, err
	}
	status, err := profile.CheckAuth(ctx, program, folder, key)
	if err != nil {
		return nil, err
	}
	if !status.SignedIn {
		return nil, clauderuntime.ErrSignIn
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("find Snow for runtime tools: %w", err)
	}
	env := map[string]string{transport.EnvPane: strconv.Itoa(pane), transport.EnvSocket: s.opts.Socket}
	launch, err := profile.ChatLaunch(program, folder, key, clauderuntime.RunOptions{Resume: session,
		MCP: map[string]clauderuntime.MCPServer{"snow": {Command: executable, Args: []string{"mcp"}, Env: env}},
	})
	if err != nil {
		return nil, err
	}
	launch.Hide = append(launch.Hide, s.opts.Chat.HideEnv...)
	client, err := clauderuntime.Connect(ctx, launch)
	if err == nil {
		s.send(func(st *state) {
			if st.runtimeReady == nil {
				st.runtimeReady = map[string]bool{}
			}
			st.runtimeReady[connection.ID] = true
		})
	}
	return client, err
}
