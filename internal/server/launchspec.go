package server

import (
	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/internal/term"
)

func launchSpec(launch shell.Launch) term.Spec {
	return term.Spec{Path: launch.Path, Args: launch.Args, Env: launch.Env, Hide: launch.Hide,
		Dir: launch.Dir, CmdLine: launch.CmdLine}
}
