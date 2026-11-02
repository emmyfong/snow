package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"time"
)

// Command returns how to run one command line in p, in dir, with no
// terminal and no folder hook. Each kind gets the form its shell reads:
// a WSL profile takes the folder as --cd and runs sh inside the distro.
func (p Profile) Command(dir, line string) Launch {
	l := Launch{Path: p.Path, Args: slices.Clone(p.Args), Dir: dir}
	switch p.Kind {
	case KindPwsh:
		l.Args = append(l.Args, "-NoProfile", "-NonInteractive", "-Command", line)
	case KindCmd:
		// cmd does not split its arguments as other programs do, so the
		// whole line goes through as it is. /S keeps the outer quotes from
		// being stripped from a line that starts with one.
		l.Args = append(l.Args, "/D", "/S", "/C", line)
		l.CmdLine = `"` + p.Path + `" /D /S /C "` + line + `"`
	case KindWSL:
		if dir != "" {
			l.Args = append(l.Args, "--cd", dir)
		}
		l.Args = append(l.Args, "-e", "sh", "-c", line)
		l.Dir = ""
	default:
		l.Args = append(l.Args, "-c", line)
	}
	return l
}

// waitDelay is how long Run waits for the output pipes after the command
// exits or is canceled. A child that keeps them open is not waited for.
const waitDelay = 2 * time.Second

// Run runs l with stdin closed and its output, stdout and stderr together,
// written to out. It returns the exit code. When ctx ends, Run kills the
// command and every process it started, and returns ctx's error. When the
// command exits, processes it left running are killed too.
func (l Launch) Run(ctx context.Context, out io.Writer) (int, error) {
	cmd, tree, err := l.start(ctx, out)
	if err != nil {
		return -1, err
	}
	err = cmd.Wait()
	// Processes the command left running are killed. The command's own
	// result is what counts, so a failed cleanup does not change it.
	_ = tree.kill()
	tree.close()
	if ctx.Err() != nil {
		return -1, ctx.Err()
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	if errors.Is(err, exec.ErrWaitDelay) {
		return cmd.ProcessState.ExitCode(), nil
	}
	if err != nil {
		return -1, fmt.Errorf("run %s: %w", l.Path, err)
	}
	return 0, nil
}

// start starts l with stdin closed and its output written to out, as the
// leader of a process tree that a cancel of ctx kills.
func (l Launch) start(ctx context.Context, out io.Writer) (*exec.Cmd, *tree, error) {
	cmd := exec.CommandContext(ctx, l.Path, l.Args...)
	cmd.Dir = l.Dir
	cmd.Env = append(hideVars(os.Environ(), l.Hide), l.Env...)
	cmd.Stdout, cmd.Stderr = out, out
	cmd.WaitDelay = waitDelay
	tree, err := newTree(cmd, l.CmdLine)
	if err != nil {
		return nil, nil, fmt.Errorf("start %s: %w", l.Path, err)
	}
	if err := tree.start(); err != nil {
		tree.close()
		return nil, nil, fmt.Errorf("start %s: %w", l.Path, err)
	}
	return cmd, tree, nil
}

// hideVars returns env without the variables named in hide.
func hideVars(env, hide []string) []string {
	if len(hide) == 0 {
		return env
	}
	out := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if !slices.ContainsFunc(hide, func(h string) bool { return sameVar(h, name) }) {
			out = append(out, kv)
		}
	}
	return out
}

// sameVar reports whether two variable names are the same variable.
// Windows names ignore case.
func sameVar(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
