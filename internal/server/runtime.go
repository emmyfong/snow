package server

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/emmyfong/snow/internal/clauderuntime"
	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/pkg/api"
)

type runtimeFacts struct {
	conversation    conversation
	snapshot        api.ChatSnapshot
	host            shell.Profile
	folder, profile string
}

func (s *Server) runtimeFacts(c *client, pane int, agent string) (runtimeFacts, error) {
	var facts runtimeFacts
	s.call(func(st *state) {
		p := st.paneOf(c, pane)
		if p == nil || p.chat == nil || p.chat.snap.Agent != agent {
			return
		}
		facts.conversation, facts.snapshot = p.chat.conv, p.chat.snap
		facts.folder, facts.profile = p.chat.folder, p.chat.profile
		if s.launch.shell != nil {
			facts.host, _ = s.launch.shell(p.profile)
		}
	})
	if facts.conversation == nil {
		return runtimeFacts{}, errors.New("the chat changed before the connection request arrived")
	}
	return facts, nil
}

func (s *Server) runtimeRequest(c *client, request *api.RuntimeRequest) {
	if err := request.Validate(); err != nil {
		s.refuse(c, api.CodeBadCommand, err)
		return
	}
	if s.opts.Chat == nil || s.opts.Chat.Runtime == nil {
		s.refuse(c, api.CodeCommandFailed, errors.New("runtime connections are not configured"))
		return
	}
	facts, err := s.runtimeFacts(c, request.Pane, request.Agent)
	if err != nil {
		s.refuse(c, api.CodeCommandFailed, err)
		return
	}
	var login *runtimeLogin
	if request.Action == api.RuntimeLogin {
		loginCtx, cancelLogin := context.WithCancel(context.Background())
		login = &runtimeLogin{connection: request.Connection, ctx: loginCtx, cancel: cancelLogin}
		registered := false
		s.call(func(st *state) {
			if st.runtimeLogins[request.Connection] != nil {
				return
			}
			if st.runtimeLogins == nil {
				st.runtimeLogins = map[string]*runtimeLogin{}
			}
			st.runtimeLogins[request.Connection] = login
			registered = true
		})
		if !registered {
			cancelLogin()
			s.refuse(c, api.CodeCommandFailed, errors.New("a sign-in attempt is already running for this connection"))
			return
		}
	}
	go s.runtimeOperation(c, request, facts, login)
}

func (s *Server) runtimeOperation(c *client, request *api.RuntimeRequest, facts runtimeFacts, login *runtimeLogin) {
	if login != nil {
		defer login.cancel()
		defer s.call(func(st *state) {
			if st.runtimeLogins[request.Connection] == login {
				delete(st.runtimeLogins, request.Connection)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	if request.Action != api.RuntimeLogin {
		cancel()
		ctx, cancel = context.WithTimeout(context.Background(), authTimeout)
	}
	defer cancel()
	selected := request.Connection
	if selected == "" && facts.snapshot.Runtime != nil {
		selected = facts.snapshot.Runtime.Connection
	}
	var message string
	var err error
	responseAgent := facts.snapshot.Agent
	var status *api.RuntimeConnection
	switch request.Action {
	case api.RuntimeList:
	case api.RuntimeCreate:
		if err = s.opts.Chat.Runtime.policy(facts.host.Distro, clauderuntime.AuthMode(request.Auth)); err == nil {
			var connection clauderuntime.Connection
			connection, err = clauderuntime.CreateConnection(s.opts.Chat.Runtime.Root, facts.host.Distro, clauderuntime.AuthMode(request.Auth))
			selected = connection.ID
		}
	case api.RuntimeUse:
		responseAgent, err = s.useRuntime(c, request.Pane, facts, request.Connection)
	case api.RuntimeSaveKey:
		err = s.changeRuntimeAuth(ctx, facts, request.Connection, func(connection clauderuntime.Connection) error {
			if connection.Auth != clauderuntime.APIKey || s.opts.Chat.Runtime.Keys == nil {
				return clauderuntime.ErrCredential
			}
			if err := s.opts.Chat.Runtime.policy(connection.Distro, connection.Auth); err != nil {
				return err
			}
			var key credential.Key
			key, err = credential.NewKey(request.Key)
			if err != nil {
				return err
			}
			profile, profileErr := clauderuntime.ProfileFor(s.opts.Chat.Runtime.Root, connection)
			if profileErr != nil {
				return profileErr
			}
			source, saveErr := s.opts.Chat.Runtime.Keys.Set(profile.CredentialID(), key)
			message = fmt.Sprintf("API key saved in %s. API billing applies. Model availability is checked when you send.", source)
			return saveErr
		})
	case api.RuntimeLogout:
		err = s.changeRuntimeAuth(ctx, facts, request.Connection, func(connection clauderuntime.Connection) error {
			return s.logoutRuntime(ctx, facts, connection)
		})
	case api.RuntimeLogin:
		err = s.changeRuntimeAuth(ctx, facts, request.Connection, func(connection clauderuntime.Connection) error {
			return s.loginRuntime(c, request, facts, connection, login)
		})
		if err == nil || errors.Is(err, context.Canceled) {
			status, message, err = s.runtimeLoginStatus(ctx, facts, request.Connection)
		}
	case api.RuntimeCancelLogin:
		if _, connectionErr := s.runtimeConnection(facts, request.Connection); connectionErr != nil {
			err = connectionErr
		} else {
			err = s.cancelRuntimeLogin(request.Connection)
		}
	case api.RuntimeStatus:
		var connection clauderuntime.Connection
		connection, err = s.runtimeConnection(facts, request.Connection)
		if err == nil {
			var info api.RuntimeConnection
			info, err = s.connectionStatus(ctx, facts, connection)
			status = &info
		}
	default:
		err = fmt.Errorf("%w: this connection operation is not available", clauderuntime.ErrControl)
	}
	response := s.runtimeConnectionList(facts, request.Pane, selected)
	response.RequestID = request.RequestID
	if responseAgent != "" {
		response.Agent = responseAgent
	}
	if status != nil {
		for i := range response.Connections {
			if response.Connections[i].ID == status.ID {
				response.Connections[i] = *status
			}
		}
	}
	if err != nil {
		response.State, response.Message = "error", err.Error()
	} else if response.State != "error" {
		response.State, response.Message = "updated", message
	}
	s.reply(c, response)
}

func (s *Server) runtimeConnection(facts runtimeFacts, id string) (clauderuntime.Connection, error) {
	connection, err := clauderuntime.LoadConnection(s.opts.Chat.Runtime.Root, id)
	if err != nil {
		return clauderuntime.Connection{}, err
	}
	if connection.Distro != facts.host.Distro {
		return clauderuntime.Connection{}, fmt.Errorf("%w: connection belongs to another environment", clauderuntime.ErrProfile)
	}
	return connection, nil
}

func (s *Server) runtimeConnectionList(facts runtimeFacts, pane int, selected string) *api.RuntimeConnections {
	response := &api.RuntimeConnections{Pane: pane, Agent: facts.snapshot.Agent, Selected: selected, Connections: []api.RuntimeConnection{},
		Ignored: clauderuntime.IgnoredHostVars(os.Environ())}
	connections, err := clauderuntime.ListConnections(s.opts.Chat.Runtime.Root, facts.host.Distro)
	if err != nil {
		response.State, response.Message = "error", err.Error()
		return response
	}
	for _, connection := range connections {
		response.Connections = append(response.Connections, api.RuntimeConnection{ID: connection.ID, Name: connection.Name,
			Auth: string(connection.Auth), Environment: runtimeEnvironment(connection.Distro), Status: "unknown"})
	}
	return response
}

func runtimeEnvironment(distro string) string {
	if distro == "" {
		return "This machine"
	}
	return "WSL " + distro
}
