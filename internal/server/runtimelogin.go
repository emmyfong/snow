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

// signIn runs Claude's sign-in for login's connection, then reads the
// state it left. Once the window opened, the sign-in may have changed even
// if the window then failed to close, so the state is read again.
func (s *Server) signIn(ctx context.Context, c *client, request *api.RuntimeRequest, facts runtimeFacts, login *runtimeLogin) (*api.RuntimeConnection, string, error) {
	err := s.changeRuntimeAuth(ctx, facts, login.connection, func(connection clauderuntime.Connection) error {
		return s.loginRuntime(c, request, facts, connection, login)
	})
	if err != nil && !errors.Is(err, context.Canceled) && login.pane == 0 {
		return nil, "", err
	}
	closeErr := err
	status, message, err := s.runtimeLoginStatus(ctx, facts, login.connection)
	if err != nil {
		return nil, "", err
	}
	var failed *signInFailed
	switch {
	case errors.As(closeErr, &failed):
		message = failed.Error()
	case closeErr != nil && !errors.Is(closeErr, context.Canceled):
		message += " The sign-in window did not close cleanly: " + closeErr.Error()
	}
	return status, message, nil
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
	// An outdated Claude would end its sign-in window at once, too fast to
	// read, so it is refused with the update step before the window opens.
	if err := profile.CheckSupport(login.ctx, program, facts.folder, ""); err != nil {
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
	if code, known := terminal.ExitStatus(); known && code != 0 && login.ctx.Err() == nil {
		// Claude ended with an error that it printed in the window. Closing
		// the window would hide it, so it stays until the user closes it.
		s.call(func(st *state) {
			if st.runtimeLogins[connection.ID] == login {
				delete(st.runtimeLogins, connection.ID)
			}
		})
		return &signInFailed{code: code}
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

// signInFailed is a sign-in whose window stays open because Claude ended
// with an error that it printed there.
type signInFailed struct{ code int }

// Error says where the reason is.
func (e *signInFailed) Error() string {
	return fmt.Sprintf("Claude sign-in failed (exit status %d). Its window stays open so you can read why. Close it when you are done", e.code)
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
