package server

import (
	"errors"
	"fmt"
	"log/slog"
	"path"
	"slices"
	"sync/atomic"
	"time"

	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/internal/term"
)

// ErrUnknownProfile means a split or a new window named a profile the server
// does not have.
var ErrUnknownProfile = errors.New("server: unknown profile")

// Options configures a Server. The zero value plus a Version works.
type Options struct {
	Version    string
	Socket     string // socket path, passed to panes in SNOW; Run sets it
	Scrollback int    // lines per pane; 0 means term.DefaultScrollback
	// DefaultProfile names the detected profile new panes start with and the
	// picker preselects. Empty, or a name that is not detected, keeps the
	// detected order. It does not apply when Spec is set.
	DefaultProfile string
	Log            *slog.Logger  // nil discards logs
	IdleExit       time.Duration // exit if no session starts within this; 0 means 10 s
	// Spec returns the program for a new pane: the named profile, started in
	// dir, and the profile's name. An empty profile means the default. Nil
	// means the shell profiles detected on this machine.
	Spec func(profile, dir string) (term.Spec, string, error)
	// Profiles lists the names Spec accepts, the default first. Nil means
	// the detected profiles, or only the default when Spec is set.
	Profiles func() []string
	// SessionsDir is where sessions are saved, so they survive a reboot.
	// Empty saves nothing.
	SessionsDir string
	// SavedLines is how many lines of each pane a saved session keeps: 0
	// means store.MaxScrollback, and less than 0 none, only the layout and
	// folders.
	SavedLines int
	// Chat is what chat panes need. Nil means the server cannot open them:
	// a split or a new window of kind chat fails with ErrNoChat.
	Chat *ChatOptions
	// Agents are the agent profiles from config.toml. The picker lists
	// them after the detected profiles. They are ignored when Spec is set.
	Agents []AgentProfile
	// Secrets reads the secrets agent profiles name. Nil means none can
	// be read: such a profile's panes fail with ErrNoSecret.
	Secrets SecretSource
	// KeyVars names the variables that hold provider keys, such as
	// OPENAI_API_KEY. Agent profile panes do not inherit them.
	KeyVars []string

	saveDelay, busyEvery, endDelay time.Duration // tests change them; 0 means the defaults

	// Tests replace these WSL checks; nil means the launcher's profiles and
	// shell.RunningDistros.
	paneDistro    func(profile string) (string, bool)
	distroRunning func(distro string) bool
	// Tests with Spec set give the Claude Code profiles with these; see
	// launcher.claude and launcher.findClaude. Nil finds none.
	claudeProfile, findClaude func(host string) (string, bool)
	// agentProfile tells tests with Spec set which profiles run an agent;
	// nil means none.
	agentProfile func(profile string) bool
	// detect stands in for shell.Detect in tests with Spec unset; set, no
	// hook scripts are written either.
	detect func() []shell.Profile
	// afterHookWait runs f when an agent pane has had defaultHookWait to
	// send its first hook; nil means a timer. Tests fire it by hand.
	afterHookWait func(f func())
}

func (o Options) logger() *slog.Logger {
	if o.Log != nil {
		return o.Log
	}
	return slog.New(slog.DiscardHandler)
}

// launcher builds the programs panes run.
type launcher struct {
	spec     func(profile, dir string) (term.Spec, string, error)
	profiles func() []string
	// distro returns the WSL distribution profile runs, for WSL profiles.
	distro func(profile string) (string, bool)
	// shell returns the shell that commands for a pane running profile
	// name go through: the profile itself, or for an agent profile the
	// shell under it. An empty name means the default. It is nil when
	// Options.Spec is set.
	shell func(name string) (shell.Profile, bool)
	// claude returns the Claude Code profile for a pane next to one
	// running host, from the profiles known now.
	claude func(host string) (string, bool)
	// findClaude looks for Claude Code again, for a pane next to one
	// running host, and adds what it finds to the profiles. It can run
	// wsl.exe, so it never runs on the model goroutine.
	findClaude func(host string) (string, bool)
	// isAgent reports whether profile runs an agent that reports through
	// hooks, such as Claude Code.
	isAgent func(profile string) bool
	// secrets returns the secrets a pane of profile needs, and
	// loadSecrets reads them so the next spec call finds them. loadSecrets
	// may block on the OS key store: never call it on the model goroutine.
	secrets     func(profile string) []string
	loadSecrets func(names []string) error
}

// newLauncher returns the launcher for opts. It detects shell profiles once:
// on Windows, detection runs wsl.exe.
func newLauncher(opts Options, log *slog.Logger) launcher {
	if opts.Spec != nil {
		l := launcher{profiles: opts.Profiles, spec: func(profile, dir string) (term.Spec, string, error) {
			return opts.Spec(profile, usableDir(dir))
		}, claude: orNone(opts.claudeProfile), findClaude: orNone(opts.findClaude), isAgent: opts.agentProfile}
		if l.isAgent == nil {
			l.isAgent = func(string) bool { return false }
		}
		l.secrets = func(string) []string { return nil }
		l.loadSecrets = func([]string) error { return nil }
		if l.profiles == nil {
			_, name, err := opts.Spec("", "")
			l.profiles = func() []string {
				if err != nil {
					return nil
				}
				return []string{name}
			}
		}
		return l
	}
	var hooks shell.Hooks
	detect := opts.detect
	if detect == nil {
		detect = shell.Detect
		if dir, err := shell.DefaultHooksDir(); err == nil {
			if h, err := shell.WriteHooks(dir); err == nil {
				hooks = h
			} else {
				log.Warn("write shell hooks", "err", err)
			}
		}
	}
	detected := detect()
	agents, extra := newAgentSet(opts, detected, log)
	detected = append(detected, extra...)
	if names := agents.all(); len(names) > 0 {
		go func() {
			if err := agents.load(names); err != nil {
				log.Warn("agent profile secrets", "err", err)
			}
		}()
	}
	if opts.DefaultProfile != "" && !hasProfile(detected, opts.DefaultProfile) {
		log.Warn("default profile not found; using the first detected", "profile", opts.DefaultProfile)
	}
	return agentLauncher(preferProfile(detected, opts.DefaultProfile), hooks, agents)
}

func hasProfile(profiles []shell.Profile, name string) bool {
	return slices.ContainsFunc(profiles, func(p shell.Profile) bool { return p.Name == name })
}

// preferProfile returns profiles with the one called name first. It returns
// them unchanged when there is no such profile.
func preferProfile(profiles []shell.Profile, name string) []shell.Profile {
	i := slices.IndexFunc(profiles, func(p shell.Profile) bool { return p.Name == name })
	if i <= 0 {
		return profiles
	}
	out := make([]shell.Profile, 0, len(profiles))
	out = append(out, profiles[i])
	out = append(out, profiles[:i]...)
	return append(out, profiles[i+1:]...)
}

// orNone returns f, or a function that finds nothing when f is nil.
func orNone(f func(string) (string, bool)) func(string) (string, bool) {
	if f != nil {
		return f
	}
	return func(string) (string, bool) { return "", false }
}

// profileSet holds the detected profiles. A Claude Code profile found later
// is added by findClaude off the model goroutine while others read the
// set, so it is swapped whole, never changed in place.
type profileSet struct {
	p atomic.Pointer[[]shell.Profile]
}

func newProfileSet(profiles []shell.Profile) *profileSet {
	s := &profileSet{}
	s.p.Store(&profiles)
	return s
}

func (s *profileSet) all() []shell.Profile { return *s.p.Load() }

// add appends p unless a profile with its name is there already.
func (s *profileSet) add(p shell.Profile) {
	for {
		old := s.p.Load()
		if hasProfile(*old, p.Name) {
			return
		}
		next := append(slices.Clone(*old), p)
		if s.p.CompareAndSwap(old, &next) {
			return
		}
	}
}

// detectedLauncher runs the given shell profiles, the first being the
// default.
func detectedLauncher(detected []shell.Profile, hooks shell.Hooks) launcher {
	agents, _ := newAgentSet(Options{}, nil, slog.New(slog.DiscardHandler))
	return agentLauncher(detected, hooks, agents)
}

// agentLauncher is detectedLauncher with the agent profiles of agents
// among detected.
func agentLauncher(detected []shell.Profile, hooks shell.Hooks, agents *agentSet) launcher {
	set := newProfileSet(detected)
	return launcher{
		secrets: func(profile string) []string {
			p, ok := shell.Find(set.all(), profile)
			if !ok {
				return nil
			}
			return agents.secrets(p.Name)
		},
		loadSecrets: agents.load,
		profiles: func() []string {
			all := set.all()
			names := make([]string, len(all))
			for i, p := range all {
				names[i] = p.Name
			}
			return names
		},
		shell: func(name string) (shell.Profile, bool) {
			p, ok := shell.Find(set.all(), name)
			if !ok {
				return shell.Profile{}, false
			}
			return shell.HostShell(set.all(), p)
		},
		isAgent: func(profile string) bool {
			p, ok := shell.Find(set.all(), profile)
			return ok && profile != "" && p.IsAgent()
		},
		distro: func(profile string) (string, bool) {
			for _, p := range set.all() {
				if p.Name == profile && p.Distro != "" {
					return p.Distro, true
				}
			}
			return "", false
		},
		claude: func(host string) (string, bool) {
			p, ok := shell.ClaudeFor(set.all(), host)
			return p.Name, ok
		},
		findClaude: func(host string) (string, bool) {
			h, _ := shell.Find(set.all(), host)
			if h.IsAgent() {
				return h.Name, true
			}
			p, ok := shell.FindClaude(h)
			if !ok {
				return "", false
			}
			set.add(p)
			return p.Name, true
		},
		spec: func(profile, dir string) (term.Spec, string, error) {
			p, ok := shell.Find(set.all(), profile)
			if !ok {
				return term.Spec{}, "", fmt.Errorf("%w: %q", ErrUnknownProfile, profile)
			}
			l := p.Launch(hooks, folderFor(p, dir))
			spec := term.Spec{Path: l.Path, Args: l.Args, Env: l.Env, Dir: l.Dir, CmdLine: l.CmdLine}
			if err := agents.apply(p.Name, &spec); err != nil {
				return term.Spec{}, "", err
			}
			return spec, p.Name, nil
		},
	}
}

// folderFor returns the folder profile p starts in. A WSL pane reports Linux
// folders, which do not exist for a Windows server, so an absolute Linux
// path passes through and wsl --cd checks it. Other profiles get dir if it
// exists here, or the home folder.
func folderFor(p shell.Profile, dir string) string {
	if p.Distro != "" && path.IsAbs(dir) {
		return dir
	}
	return usableDir(dir)
}
