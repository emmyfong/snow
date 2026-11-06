package server

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/emmyfong/snow/internal/clauderuntime"
	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/pkg/api"
)

// errRuntimeInUse refuses to remove a connection that an open chat uses.
var errRuntimeInUse = errors.New("a chat uses this connection. Switch that chat to another connection, or close it, first")

// createRuntime adds a connection only when it works: a Claude plan
// connection once its sign-in ends signed in, an API key connection with
// its key. A failed attempt removes what it made, so nothing half made
// stays in the list (#290). It returns the new connection's ID, or "".
func (s *Server) createRuntime(ctx context.Context, c *client, request *api.RuntimeRequest, facts runtimeFacts) (string, *api.RuntimeConnection, string, error) {
	options := s.opts.Chat.Runtime
	auth := clauderuntime.AuthMode(request.Auth)
	if err := options.policy(facts.host.Distro, auth); err != nil {
		return "", nil, "", err
	}
	connection, err := clauderuntime.CreateConnection(options.Root, facts.host.Distro, auth)
	if err != nil {
		return "", nil, "", err
	}
	var status *api.RuntimeConnection
	var message string
	if auth == clauderuntime.APIKey {
		message, err = s.saveRuntimeKey(connection, request.Key)
	} else {
		var login *runtimeLogin
		if login, err = s.newLogin(connection.ID); err == nil {
			status, message, err = s.signIn(ctx, c, request, facts, login)
			s.endLogin(login)
		}
	}
	if err == nil && (status == nil || status.Status == api.RuntimeStatusSignedIn) {
		return connection.ID, status, message, nil
	}
	// The sign-in may have ended with the context; removing still runs.
	removeCtx, cancel := context.WithTimeout(context.Background(), authTimeout)
	defer cancel()
	if removeErr := s.deleteConnection(removeCtx, facts, connection, false); removeErr != nil {
		return connection.ID, status, message, errors.Join(err, fmt.Errorf("remove the new connection: %w", removeErr))
	}
	if err != nil {
		return "", nil, "", err
	}
	return "", nil, message + " The connection was not added.", nil
}

// removeRuntime signs a connection out and deletes it, with its saved key.
// Saved chats that used it stay. A connection an open chat uses stays.
func (s *Server) removeRuntime(ctx context.Context, facts runtimeFacts, id string) error {
	inUse := false
	s.call(func(st *state) {
		for _, session := range st.sessions {
			for _, window := range session.windows {
				for _, pane := range window.panes {
					if pane.chat != nil && pane.chat.snap.Runtime != nil && pane.chat.snap.Runtime.Connection == id {
						inUse = true
					}
				}
			}
		}
	})
	if inUse {
		return errRuntimeInUse
	}
	err := s.changeRuntimeAuth(ctx, facts, id, func(connection clauderuntime.Connection) error {
		return s.deleteConnection(ctx, facts, connection, true)
	})
	if err == nil {
		s.call(func(st *state) { delete(st.runtimeModels, id) })
	}
	return err
}

// deleteConnection removes a connection with its sign-in or saved key.
// signOut first asks Claude to sign out, so a sign-in Claude keeps outside
// the folder, such as in the macOS keychain, goes too. A WSL connection's
// sign-in lives inside the distribution, so Snow removes it there first;
// if that fails, the connection stays for another try.
func (s *Server) deleteConnection(ctx context.Context, facts runtimeFacts, connection clauderuntime.Connection, signOut bool) error {
	options := s.opts.Chat.Runtime
	profile, err := clauderuntime.ProfileFor(options.Root, connection)
	if err != nil {
		return err
	}
	if connection.Auth == clauderuntime.APIKey && options.Keys != nil {
		if err := options.Keys.DeleteStored(profile.CredentialID()); err != nil && !errors.Is(err, credential.ErrNoKey) {
			return err
		}
	}
	if signOut && connection.Auth == clauderuntime.Subscription {
		// A Claude too old to sign out, or one that is gone, must not keep
		// the connection: deleting the folder removes its sign-in here.
		if err := s.logoutRuntime(ctx, facts, connection); err != nil {
			s.log.Warn("sign out a connection before removing it", "err", err)
		}
	}
	if connection.Distro != "" {
		program, err := options.program(facts.host)
		if err != nil {
			return err
		}
		launch, err := profile.RemoveLaunch(program)
		if err != nil {
			return err
		}
		code, err := launch.Run(ctx, io.Discard)
		if err != nil {
			return err
		}
		if code != 0 {
			return fmt.Errorf("remove the sign-in folder in WSL %s: exit status %d", connection.Distro, code)
		}
	}
	return clauderuntime.RemoveConnection(options.Root, connection.ID)
}

// saveRuntimeKey saves raw as an API key connection's key and says where.
func (s *Server) saveRuntimeKey(connection clauderuntime.Connection, raw string) (string, error) {
	options := s.opts.Chat.Runtime
	if connection.Auth != clauderuntime.APIKey || options.Keys == nil {
		return "", clauderuntime.ErrCredential
	}
	if err := options.policy(connection.Distro, connection.Auth); err != nil {
		return "", err
	}
	key, err := credential.NewKey(raw)
	if err != nil {
		return "", err
	}
	profile, err := clauderuntime.ProfileFor(options.Root, connection)
	if err != nil {
		return "", err
	}
	source, err := options.Keys.Set(profile.CredentialID(), key)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("API key saved in %s. API billing applies. Model availability is checked when you send.", source), nil
}
