package server

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// ErrNoSecret means an agent profile needs a secret that is not saved, or
// cannot be read. The error names the secret, never its value.
var ErrNoSecret = errors.New("server: secret not available")

// SecretSource reads a saved secret by name, such as the token of a
// gateway. credential.Store satisfies it. A read may wait on the OS key
// store, so the server calls it off the state goroutine.
type SecretSource interface {
	APIKey(name string) (string, error)
}

// AgentProfile is an agent profile from config.toml: a program for a pane,
// with its own environment.
type AgentProfile struct {
	Name    string   // the name the picker shows and saved sessions keep
	Command []string // the program and its arguments
	Env     []EnvVar
}

// EnvVar is a variable of an agent profile's pane: a value, or the name of
// a secret whose value the server reads when the pane starts.
type EnvVar struct {
	Name   string
	Value  string // used when Secret is empty
	Secret string // the secret to read from Options.Secrets
}

// gatewayVars are inherited variables an agent profile's pane never gets:
// the Claude Code settings a profile may point at another host. A key the
// user's environment holds must not reach that host.
var gatewayVars = []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_BASE_URL"}

// agentSet holds the agent profiles and the secrets their panes need.
type agentSet struct {
	byName map[string]AgentProfile
	broken map[string]error // a profile whose program was not found
	hide   []string
	src    SecretSource
	cache  atomic.Pointer[map[string]string] // secret name to value; swapped whole
}

// newAgentSet returns the agent profiles of opts as shell profiles to add
// after taken, and the set that fills in their environment. A profile
// named like one in taken is skipped with a log line.
func newAgentSet(opts Options, taken []shell.Profile, log *slog.Logger) (*agentSet, []shell.Profile) {
	s := &agentSet{byName: map[string]AgentProfile{}, broken: map[string]error{}, src: opts.Secrets}
	s.hide = append(slices.Clone(gatewayVars), opts.KeyVars...)
	s.cache.Store(&map[string]string{})
	var out []shell.Profile
	for _, a := range opts.Agents {
		if hasProfile(taken, a.Name) || hasProfile(out, a.Name) {
			log.Warn("agent profile skipped: another profile has its name", "profile", a.Name)
			continue
		}
		p, err := shell.Program(a.Name, a.Command)
		if err != nil {
			p = shell.Profile{Name: a.Name, Kind: shell.KindCommand}
			s.broken[a.Name] = err
			log.Warn("agent profile: program not found", "profile", a.Name, "err", err)
		}
		if len(a.Command) > 0 && runsClaude(a.Command[0]) {
			p.Kind = shell.KindClaude
		}
		s.byName[a.Name] = a
		out = append(out, p)
	}
	return s, out
}

// runsClaude reports whether program is Claude Code, which reports
// through hooks.
func runsClaude(program string) bool {
	base := strings.ToLower(filepath.Base(program))
	return strings.TrimSuffix(base, filepath.Ext(base)) == "claude"
}

// secrets returns the secrets a pane of profile needs; none for a profile
// that is not an agent profile.
func (s *agentSet) secrets(profile string) []string {
	var names []string
	for _, v := range s.byName[profile].Env {
		if v.Secret != "" && !slices.Contains(names, v.Secret) {
			names = append(names, v.Secret)
		}
	}
	return names
}

// all returns every secret the profiles need.
func (s *agentSet) all() []string {
	var names []string
	for _, name := range slices.Sorted(maps.Keys(s.byName)) {
		for _, n := range s.secrets(name) {
			if !slices.Contains(names, n) {
				names = append(names, n)
			}
		}
	}
	return names
}

// load reads names from the secret source into the cache, which the state
// goroutine reads. A secret that cannot be read leaves the cache, so a
// removed token stops working. It may block, so it never runs on the
// state goroutine.
func (s *agentSet) load(names []string) error {
	values := map[string]string{}
	var errs []error
	for _, name := range names {
		v, err := s.read(name)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		values[name] = v
	}
	// Two loads may run at once, for example the one at start and one for
	// a new pane, so each merges only its own names into the latest map.
	for {
		old := s.cache.Load()
		next := maps.Clone(*old)
		for _, name := range names {
			if v, ok := values[name]; ok {
				next[name] = v
			} else {
				delete(next, name)
			}
		}
		if s.cache.CompareAndSwap(old, &next) {
			return errors.Join(errs...)
		}
	}
}

func (s *agentSet) read(name string) (string, error) {
	if s.src == nil {
		return "", fmt.Errorf("%w: %s; Snow has no credential store", ErrNoSecret, name)
	}
	v, err := s.src.APIKey(name)
	if err != nil {
		return "", fmt.Errorf("%w: %s (%w). Run: snow auth login %s", ErrNoSecret, name, err, name)
	}
	return v, nil
}

// apply gives spec the environment of agent profile profile: the inherited
// key variables left out, then the profile's variables, with secrets from
// the cache. It reads no store, so the state goroutine may call it. A
// profile that is not an agent profile is left alone.
func (s *agentSet) apply(profile string, spec *term.Spec) error {
	a, ok := s.byName[profile]
	if !ok {
		return nil
	}
	if err := s.broken[profile]; err != nil {
		return err
	}
	cache := *s.cache.Load()
	spec.Hide = append(spec.Hide, s.hide...)
	for _, v := range a.Env {
		value := v.Value
		if v.Secret != "" {
			var ok bool
			if value, ok = cache[v.Secret]; !ok {
				return fmt.Errorf("%w: %s. Run: snow auth login %s", ErrNoSecret, v.Secret, v.Secret)
			}
		}
		spec.Env = append(spec.Env, v.Name+"="+value)
	}
	return nil
}

// runWithSecrets runs cmd as run does. When cmd opens a terminal pane of
// an agent profile that names secrets, a goroutine reads them first, since
// the read may wait on the OS key store, and the command runs when it
// answers. A secret that cannot be read refuses the command with an error
// that names it.
func (st *state) runWithSecrets(c *client, sess *session, cmd *api.Command) {
	names := st.paneSecrets(sess, cmd)
	if len(names) == 0 {
		st.run(c, sess, cmd)
		return
	}
	srv := st.srv
	go func() {
		err := srv.launch.loadSecrets(names)
		srv.post(func(st *state) {
			if c.dropped || st.sessions[sess.name] != sess || c.session != sess.name {
				return
			}
			if err != nil {
				st.enqueue(c, &api.Error{Code: api.CodeCommandFailed, Message: err.Error()})
				return
			}
			st.run(c, sess, cmd)
		})
	}()
}

// paneSecrets returns the secrets the terminal pane cmd opens needs; none
// for a command that opens no terminal pane.
func (st *state) paneSecrets(sess *session, cmd *api.Command) []string {
	if cmd.Args[api.ArgKind] == api.PaneChat {
		return nil
	}
	var from int
	switch cmd.Op {
	case api.OpSplit:
		from = cmd.Target
	case api.OpNewWindow:
	default:
		return nil
	}
	profile, _ := st.newPaneArgs(sess, cmd, from)
	return st.srv.launch.secrets(profile)
}
