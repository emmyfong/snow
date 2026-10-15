package server

import (
	"errors"
	"fmt"
	"log/slog"
	"path"
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
	Scrollback int           // lines per pane; 0 means term.DefaultScrollback
	Log        *slog.Logger  // nil discards logs
	IdleExit   time.Duration // exit if no session starts within this; 0 means 10 s
	// Spec returns the program for a new pane: the named profile, started in
	// dir, and the profile's name. An empty profile means the default. Nil
	// means the shell profiles detected on this machine.
	Spec func(profile, dir string) (term.Spec, string, error)
	// Profiles lists the names Spec accepts, the default first. Nil means
	// the detected profiles, or only the default when Spec is set.
	Profiles func() []string
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
}

// newLauncher returns the launcher for opts. It detects shell profiles once:
// on Windows, detection runs wsl.exe.
func newLauncher(opts Options, log *slog.Logger) launcher {
	if opts.Spec != nil {
		l := launcher{profiles: opts.Profiles, spec: func(profile, dir string) (term.Spec, string, error) {
			return opts.Spec(profile, usableDir(dir))
		}}
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
	if dir, err := shell.DefaultHooksDir(); err == nil {
		if h, err := shell.WriteHooks(dir); err == nil {
			hooks = h
		} else {
			log.Warn("write shell hooks", "err", err)
		}
	}
	return detectedLauncher(shell.Detect(), hooks)
}

// detectedLauncher runs the given shell profiles, the first being the
// default.
func detectedLauncher(detected []shell.Profile, hooks shell.Hooks) launcher {
	names := make([]string, len(detected))
	for i, p := range detected {
		names[i] = p.Name
	}
	return launcher{
		profiles: func() []string { return names },
		spec: func(profile, dir string) (term.Spec, string, error) {
			for _, p := range detected {
				if profile == "" || p.Name == profile {
					l := p.Launch(hooks, folderFor(p, dir))
					return term.Spec{Path: l.Path, Args: l.Args, Env: l.Env, Dir: l.Dir}, p.Name, nil
				}
			}
			return term.Spec{}, "", fmt.Errorf("%w: %q", ErrUnknownProfile, profile)
		},
	}
}

// folderFor returns the folder profile p starts in. A WSL pane reports Linux
// folders, which do not exist for a Windows server, so an absolute Linux
// path passes through and wsl --cd checks it. Other profiles get dir if it
// exists here, or the home folder.
func folderFor(p shell.Profile, dir string) string {
	if p.Kind == shell.KindWSL && path.IsAbs(dir) {
		return dir
	}
	return usableDir(dir)
}
