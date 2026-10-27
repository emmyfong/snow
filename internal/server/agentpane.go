package server

import (
	"maps"

	"github.com/emmyfong/snow/pkg/api"
)

// claudeInstall is where the hint sends a user who has no claude. The hint
// is short: the status bar drops a message wider than the screen.
const claudeInstall = "claude.com/claude-code"

// agentSplit splits the target pane with a pane of the agent cmd asks for.
// The profile is the one that fits the target pane; when none is known, a
// goroutine looks again, since claude may have been installed after the
// server started, and the split happens when it answers. With none found,
// c learns how to install it.
func (st *state) agentSplit(c *client, sess *session, cmd *api.Command) {
	_, p := sess.findPane(cmd.Target)
	if p == nil {
		st.enqueue(c, &api.Error{Code: api.CodeCommandFailed, Message: "no pane to split"})
		return
	}
	launch := st.srv.launch
	if name, ok := launch.claude(p.profile); ok {
		st.run(c, sess, withProfile(cmd, name))
		return
	}
	host := p.profile
	distro, _ := st.srv.distroOf(host)
	srv := st.srv
	go func() {
		name, ok := launch.findClaude(host)
		srv.post(func(st *state) {
			if c.dropped || st.sessions[sess.name] != sess || c.session != sess.name {
				return
			}
			if !ok {
				st.enqueue(c, &api.Error{Code: api.CodeCommandFailed, Message: claudeMissing(distro)})
				return
			}
			st.run(c, sess, withProfile(cmd, name))
		})
	}()
}

// withProfile returns a copy of cmd that names profile in place of an agent.
func withProfile(cmd *api.Command, profile string) *api.Command {
	out := *cmd
	out.Args = maps.Clone(cmd.Args)
	delete(out.Args, api.ArgAgent)
	out.Args[api.ArgProfile] = profile
	return &out
}

// claudeMissing says how to get Claude Code, on this machine or inside
// distro.
func claudeMissing(distro string) string {
	if distro != "" {
		return "Claude Code not found in " + distro + ". Get it: " + claudeInstall
	}
	return "Claude Code not found. Get it: " + claudeInstall
}
