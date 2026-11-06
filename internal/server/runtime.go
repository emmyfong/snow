package server

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/emmyfong/snow/internal/clauderuntime"
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
	found := true
	s.call(func(st *state) {
		p := st.paneOf(c, pane)
		if p == nil || p.chat == nil || p.chat.snap.Agent != agent {
			return
		}
		facts.conversation, facts.snapshot = p.chat.conv, p.chat.snap
		facts.folder, facts.profile = p.chat.folder, p.chat.profile
		if s.launch.shell != nil {
			facts.host, found = s.launch.shell(p.profile)
		}
	})
	if facts.conversation == nil {
		return runtimeFacts{}, errors.New("the chat changed before the connection request arrived")
	}
	if !found {
		// A zero host is this machine. Using it for a chat whose shell is
		// gone would move a WSL chat to another environment's login.
		return runtimeFacts{}, fmt.Errorf("%w: this chat's shell profile %q no longer exists", clauderuntime.ErrProfile, facts.profile)
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
		if login, err = s.newLogin(request.Connection); err != nil {
			s.refuse(c, api.CodeCommandFailed, err)
			return
		}
	}
	go s.runtimeOperation(c, request, facts, login)
}

// newLogin registers a sign-in for connection, refusing a second one.
func (s *Server) newLogin(connection string) (*runtimeLogin, error) {
	ctx, cancel := context.WithCancel(context.Background())
	login := &runtimeLogin{connection: connection, ctx: ctx, cancel: cancel}
	registered := false
	s.call(func(st *state) {
		if st.runtimeLogins[connection] != nil {
			return
		}
		if st.runtimeLogins == nil {
			st.runtimeLogins = map[string]*runtimeLogin{}
		}
		st.runtimeLogins[connection] = login
		registered = true
	})
	if !registered {
		cancel()
		return nil, errors.New("a sign-in attempt is already running for this connection")
	}
	return login, nil
}

// endLogin forgets a sign-in that ended.
func (s *Server) endLogin(login *runtimeLogin) {
	login.cancel()
	s.call(func(st *state) {
		if st.runtimeLogins[login.connection] == login {
			delete(st.runtimeLogins, login.connection)
		}
	})
}

func (s *Server) runtimeOperation(c *client, request *api.RuntimeRequest, facts runtimeFacts, login *runtimeLogin) {
	if login != nil {
		defer s.endLogin(login)
	}
	ctx, cancel := context.WithCancel(context.Background())
	// A sign-in waits for the user as long as they need.
	if request.Action != api.RuntimeLogin && (request.Action != api.RuntimeCreate || request.Auth != string(clauderuntime.Subscription)) {
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
		selected, status, message, err = s.createRuntime(ctx, c, request, facts)
	case api.RuntimeRemove:
		if err = s.removeRuntime(ctx, facts, request.Connection); err == nil {
			selected, message = "", "Connection removed. Saved chats stay."
		}
	case api.RuntimeUse:
		responseAgent, err = s.useRuntime(c, request.Pane, facts, request.Connection)
	case api.RuntimeSaveKey:
		err = s.changeRuntimeAuth(ctx, facts, request.Connection, func(connection clauderuntime.Connection) error {
			var saveErr error
			message, saveErr = s.saveRuntimeKey(connection, request.Key)
			return saveErr
		})
	case api.RuntimeLogout:
		err = s.changeRuntimeAuth(ctx, facts, request.Connection, func(connection clauderuntime.Connection) error {
			return s.logoutRuntime(ctx, facts, connection)
		})
	case api.RuntimeLogin:
		status, message, err = s.signIn(ctx, c, request, facts, login)
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
	response := &api.RuntimeConnections{Pane: pane, Agent: facts.snapshot.Agent, Selected: selected, Connections: []api.RuntimeConnection{}}
	if facts.host.Distro == "" {
		// A distribution has its own environment, which Snow cannot read here.
		response.Ignored = clauderuntime.IgnoredHostVars(os.Environ())
	}
	connections, unreadable, err := clauderuntime.ListConnections(s.opts.Chat.Runtime.Root, facts.host.Distro)
	if err != nil {
		response.State, response.Message = "error", err.Error()
		return response
	}
	for _, connection := range connections {
		response.Connections = append(response.Connections, api.RuntimeConnection{ID: connection.ID, Name: connection.Name,
			Auth: string(connection.Auth), Environment: runtimeEnvironment(connection.Distro), Status: api.RuntimeStatusUnknown})
	}
	for _, broken := range unreadable {
		response.Connections = append(response.Connections, api.RuntimeConnection{ID: broken.ID, Name: "Claude Code",
			Environment: "unknown", Status: api.RuntimeStatusUnknown, Reason: "Snow cannot read this connection's saved settings: " + broken.Err.Error()})
	}
	return response
}

func runtimeEnvironment(distro string) string {
	if distro == "" {
		return "This machine"
	}
	return "WSL " + distro
}
