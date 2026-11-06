package server

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/emmyfong/snow/internal/clauderuntime"
	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/pkg/api"
)

// errAuthChanging refuses a runtime start while its connection's sign-in
// changes. Chats open at the change are paused; this covers one that starts
// during it, such as a chat restored at that moment.
var errAuthChanging = errors.New("this connection's sign-in is changing. Send again when it finishes")

func (s *Server) changeRuntimeAuth(ctx context.Context, facts runtimeFacts, id string, change func(clauderuntime.Connection) error) error {
	connection, err := s.runtimeConnection(facts, id)
	if err != nil {
		return err
	}
	var chats []*claudeConversation
	busy := false
	s.call(func(st *state) {
		busy = st.runtimeAuth[id]
		if busy {
			return
		}
		if st.runtimeAuth == nil {
			st.runtimeAuth = map[string]bool{}
		}
		st.runtimeAuth[id] = true
		for _, session := range st.sessions {
			for _, window := range session.windows {
				for _, pane := range window.panes {
					if pane.chat == nil || pane.chat.snap.Runtime == nil || pane.chat.snap.Runtime.Connection != id {
						continue
					}
					if chat, ok := pane.chat.conv.(*claudeConversation); ok {
						chats = append(chats, chat)
					}
				}
			}
		}
	})
	if busy {
		return errors.New("connection authentication is already being updated")
	}
	defer s.call(func(st *state) {
		delete(st.runtimeAuth, id)
		// Whatever the change did, the next start proves readiness again.
		delete(st.runtimeReady, id)
	})
	var paused []*claudeConversation
	defer func() {
		resumeCtx, cancel := context.WithTimeout(context.Background(), authTimeout)
		defer cancel()
		for _, chat := range paused {
			if err := chat.runtime.ResumeAfterAuth(resumeCtx); err != nil && !errors.Is(err, clauderuntime.ErrClosed) {
				s.log.Warn("resume connection controls after authentication", "err", err)
			}
		}
	}()
	for _, chat := range chats {
		// Include an ambiguous/cancelled pause in cleanup: it can have reached
		// the controller even when this caller's deadline expired afterward.
		paused = append(paused, chat)
		if err := chat.runtime.PauseForAuth(ctx); err != nil && !errors.Is(err, clauderuntime.ErrClosed) {
			return err
		}
	}
	return change(connection)
}

func (s *Server) logoutRuntime(ctx context.Context, facts runtimeFacts, connection clauderuntime.Connection) error {
	options := s.opts.Chat.Runtime
	profile, err := clauderuntime.ProfileFor(options.Root, connection)
	if err != nil {
		return err
	}
	if connection.Auth == clauderuntime.APIKey {
		if options.Keys == nil {
			return clauderuntime.ErrSignIn
		}
		err := options.Keys.DeleteStored(profile.CredentialID())
		if errors.Is(err, credential.ErrNoKey) {
			return nil
		}
		return err
	}
	program, err := options.program(facts.host)
	if err != nil {
		return err
	}
	launch, err := profile.AuthLaunch(program, facts.folder, "", "logout")
	if err != nil {
		return err
	}
	code, err := launch.Run(ctx, io.Discard)
	if err != nil {
		return err
	}
	if code != 0 {
		return clauderuntime.ErrControl
	}
	return nil
}

// connectionStatus asks the connection's own Claude for its sign-in state.
// It never sends a model request. A failure to find out leaves the status
// unknown, with the reason, and is not an error of the whole operation.
func (s *Server) connectionStatus(ctx context.Context, facts runtimeFacts, connection clauderuntime.Connection) (api.RuntimeConnection, error) {
	info := api.RuntimeConnection{ID: connection.ID, Name: connection.Name, Auth: string(connection.Auth),
		Environment: runtimeEnvironment(connection.Distro), Status: api.RuntimeStatusUnknown}
	if err := s.opts.Chat.Runtime.policy(connection.Distro, connection.Auth); err != nil {
		if errors.Is(err, clauderuntime.ErrPolicy) {
			info.Status = api.RuntimeStatusBlocked
		}
		info.Reason = err.Error()
		return info, nil
	}
	profile, key, err := s.opts.Chat.Runtime.profile(connection)
	if errors.Is(err, clauderuntime.ErrSignIn) {
		info.Status = api.RuntimeStatusSignedOut
		return info, nil
	}
	if err != nil {
		info.Reason = err.Error()
		return info, nil
	}
	if connection.Auth == clauderuntime.APIKey {
		info.Status = api.RuntimeStatusKeySaved
	} else {
		program, err := s.opts.Chat.Runtime.program(facts.host)
		if err != nil {
			info.Reason = err.Error()
			return info, nil
		}
		statusCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		status, err := profile.CheckAuth(statusCtx, program, facts.folder, key)
		if err != nil {
			info.Reason = err.Error()
			return info, nil
		}
		info.Status = api.RuntimeStatusSignedOut
		if status.SignedIn {
			info.Status = api.RuntimeStatusSignedIn
		}
	}
	if info.Status != api.RuntimeStatusSignedOut {
		s.call(func(st *state) { info.Ready = st.runtimeReady[connection.ID] })
	}
	return info, nil
}
