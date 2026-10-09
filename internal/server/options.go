package server

import (
	"log/slog"

	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/internal/term"
)

// Options configures a Server. The zero value plus a Version works.
type Options struct {
	Version    string
	Scrollback int          // lines per pane; 0 means term.DefaultScrollback
	Log        *slog.Logger // nil discards logs
	// Spec returns the program for a new pane and its profile name. Nil means
	// the first detected shell profile, with its folder hook.
	Spec func() (term.Spec, string)
}

func (o Options) logger() *slog.Logger {
	if o.Log != nil {
		return o.Log
	}
	return slog.New(slog.DiscardHandler)
}

// defaultSpec starts the first detected shell with its OSC 7 folder hook.
func defaultSpec(log *slog.Logger) func() (term.Spec, string) {
	var hooks shell.Hooks
	if dir, err := shell.DefaultHooksDir(); err == nil {
		if h, err := shell.WriteHooks(dir); err == nil {
			hooks = h
		} else {
			log.Warn("write shell hooks", "err", err)
		}
	}
	return func() (term.Spec, string) {
		p := shell.Detect()[0]
		l := p.Launch(hooks, "")
		return term.Spec{Path: l.Path, Args: l.Args, Env: l.Env, Dir: l.Dir}, p.Name
	}
}
