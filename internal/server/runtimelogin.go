package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/emmyfong/snow/internal/clauderuntime"
	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

type runtimeLogin struct {
	ctx                 context.Context
	cancel              context.CancelFunc
	connection, session string
	pane                int
	terminal            *term.Pane
}

func (s *Server) loginRuntime(c *client, request *api.RuntimeRequest, facts runtimeFacts, connection clauderuntime.Connection, login *runtimeLogin) error {
	if err := login.ctx.Err(); err != nil {
		return err
	}
	if connection.Auth != clauderuntime.Subscription {
		return clauderuntime.ErrCredential
	}
	if err := s.opts.Chat.Runtime.policy(connection.Distro, connection.Auth); err != nil {
		return err
	}
	program, err := s.opts.Chat.Runtime.program(facts.host)
	if err != nil {
		return err
	}
	profile, _, err := s.opts.Chat.Runtime.profile(connection)
	if err != nil {
		return err
	}
	launch, err := profile.AuthLaunch(program, facts.folder, "", "login")
	if err != nil {
		return err
	}
	launch.Hide = append(launch.Hide, s.opts.Chat.HideEnv...)
	if err := login.ctx.Err(); err != nil {
		return err
	}
	var id, cols, rows, originWindow int
	var sessionName string
	s.call(func(st *state) {
		p := st.paneOf(c, request.Pane)
		if p == nil || p.chat == nil || p.chat.snap.Agent != facts.snapshot.Agent {
			return
		}
		sess := st.sessions[c.session]
		w, _ := sess.findPane(p.id)
		id, sessionName, originWindow = st.nextPane, sess.name, w.index
		st.nextPane++
		cols, rows = sess.area()
	})
	if id == 0 {
		return errors.New("the chat changed before sign-in started")
	}
	preamble := fmt.Sprintf("Claude Code sign-in (%s). This sign-in is only for Snow.\r\n"+
		"Sign in in the browser. If the browser cannot return here, it shows a code:\r\n"+
		"paste the code below and press Enter. Close this window to cancel.\r\n\r\n", runtimeEnvironment(connection.Distro))
	terminal, err := term.Start(launchSpec(launch), cols, rows, term.Options{Scrollback: 200, Preamble: preamble})
	if err != nil {
		return err
	}
	adopted := false
	s.call(func(st *state) {
		if login.ctx.Err() != nil {
			return
		}
		sess := st.sessions[sessionName]
		if sess == nil {
			return
		}
		if _, origin := sess.findPane(request.Pane); origin == nil || origin.chat == nil || origin.chat.snap.Agent != facts.snapshot.Agent {
			return
		}
		p := &pane{id: id, profile: "Claude sign-in", dir: facts.folder, term: terminal, purpose: "sign-in"}
		st.addWindow(sess, p, true)
		if st.runtimeLogins == nil {
			st.runtimeLogins = map[string]*runtimeLogin{}
		}
		login.session, login.pane, login.terminal = sessionName, id, terminal
		adopted = true
		st.repaint(sess)
		st.changed(sess)
	})
	if !adopted {
		return errors.Join(errors.New("the chat closed before sign-in opened"), terminal.Close())
	}
	response := s.runtimeConnectionList(facts, request.Pane, connection.ID)
	response.RequestID, response.State, response.LoginPane = request.RequestID, "signing-in", id
	response.Message = "Complete Claude sign-in in the temporary sign-in window."
	s.reply(c, response)
	select {
	case <-terminal.Done():
	case <-login.ctx.Done():
	case <-s.quit:
	}
	closeErr := terminal.Close()
	s.call(func(st *state) {
		if st.runtimeLogins[connection.ID] == login {
			delete(st.runtimeLogins, connection.ID)
		}
		if sess := st.sessions[sessionName]; sess != nil {
			if w, p := sess.findPane(id); p != nil && p.term == terminal {
				returnToOrigin := sess.active == w.index
				st.removeWindow(sess, w)
				if st.sessions[sessionName] != nil {
					if returnToOrigin && sess.window(originWindow) != nil {
						sess.active = originWindow
					}
					st.repaint(sess)
					st.changed(sess)
				}
			}
		}
	})
	return closeErr
}

func (s *Server) cancelRuntimeLogin(connection string) error {
	var terminal *term.Pane
	s.call(func(st *state) {
		if login := st.runtimeLogins[connection]; login != nil {
			login.cancel()
			terminal = login.terminal
		}
	})
	if terminal == nil {
		return nil
	}
	return terminal.Close()
}

func (s *Server) runtimeLoginStatus(ctx context.Context, facts runtimeFacts, id string) (*api.RuntimeConnection, string, error) {
	connection, err := s.runtimeConnection(facts, id)
	if err != nil {
		return nil, "", err
	}
	info, err := s.connectionStatus(ctx, facts, connection)
	if err != nil {
		return nil, "", err
	}
	var message string
	switch info.Status {
	case api.RuntimeStatusSignedIn:
		message = "The selected Snow connection is signed in. No task was started."
	case api.RuntimeStatusSignedOut:
		message = "Sign-in view closed. The selected Snow connection is signed out."
	case api.RuntimeStatusBlocked:
		message = "Sign-in view closed. " + info.Reason
	default:
		message = "Sign-in view closed. Snow could not check the sign-in state: " + info.Reason
	}
	return &info, message, nil
}
