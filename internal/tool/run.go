package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Limits of the run tool.
const (
	runTimeout    = 2 * time.Minute
	runMaxTimeout = 10 * time.Minute
	runBytes      = 32 << 10 // output kept: the first and the last half
)

type runTool struct{}

func init() { register(runTool{}) }

func (runTool) Def() Def {
	return Def{
		Name: "run",
		Description: "Run one command line in the conversation folder, through the conversation's " +
			"shell, with no input and no terminal. Returns the output, stdout and stderr together, " +
			"and the exit code. Long output keeps its start and end. Write the command for that " +
			"shell: it may be PowerShell, cmd, or a POSIX shell.",
		Schema: schema(map[string]any{
			"command":         prop("string", "the command line"),
			"timeout_seconds": prop("integer", "how long to wait before the command is killed; default 120, at most 600"),
		}, "command"),
		Changes: true,
	}
}

func (runTool) Run(ctx context.Context, env Env, raw json.RawMessage) (string, error) {
	var in struct {
		Command        string `json:"command"`
		TimeoutSeconds int    `json:"timeout_seconds"`
	}
	if err := input(raw, &in); err != nil {
		return "", err
	}
	if in.Command == "" {
		return "", errors.New("no command given")
	}
	if env.Shell.Path == "" {
		return "", errors.New("this conversation has no shell profile")
	}
	timeout := runTimeout
	if in.TimeoutSeconds > 0 {
		timeout = min(time.Duration(in.TimeoutSeconds)*time.Second, runMaxTimeout)
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out := newHeadTail(runBytes)
	l := env.Shell.Command(env.Folder.Path, in.Command)
	l.Hide = env.HideEnv
	code, err := l.Run(runCtx, out)
	switch {
	case ctx.Err() != nil:
		return "", ctx.Err()
	case runCtx.Err() != nil:
		return "", fmt.Errorf("%s[timed out after %s; the command was killed]", out, timeout)
	case err != nil:
		return "", err
	}
	return fmt.Sprintf("%s[exit code %d]\n", out, code), nil
}
