package clauderuntime

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/emmyfong/snow/internal/shell"
)

// Connect starts a profile's structured worker. A WSL worker owns a Linux
// process group and verifies its cleanup before releasing the Windows bridge.
// It performs initialization only and submits no user prompt.
func (p Profile) Connect(ctx context.Context, program shell.Profile, launch shell.Launch) (*Client, error) {
	stream, err := p.startStream(program, launch)
	if err != nil {
		return nil, err
	}
	return initializeStream(ctx, stream)
}

func (p Profile) startStream(program shell.Profile, launch shell.Launch) (*Stream, error) {
	if program.Distro != p.Distro {
		return nil, ErrProfile
	}
	if p.Distro == "" {
		return StartStream(launch)
	}
	owned, receipt, err := p.ownedWSLLaunch(program, launch, strings.ToLower(rand.Text()))
	if err != nil {
		return nil, err
	}
	return startStream(owned, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		code, err := receipt.RunIO(ctx, io.Discard, io.Discard)
		if err != nil || code != 0 {
			return errors.Join(ErrStop, fmt.Errorf("verify Linux process cleanup in %s: %w", p.Distro, errors.Join(err, ErrControl)))
		}
		return nil
	})
}

func (p Profile) ownedWSLLaunch(program shell.Profile, launch shell.Launch, run string) (shell.Launch, shell.Launch, error) {
	if !validID(p.ID) || !validID(run) || p.Distro == "" || program.Distro != p.Distro || launch.Path != program.Path {
		return shell.Launch{}, shell.Launch{}, ErrProfile
	}
	index := slices.Index(launch.Args, wslScript)
	if index < 0 || index+2 >= len(launch.Args) || launch.Args[index+2] != p.ID {
		return shell.Launch{}, shell.Launch{}, ErrProfile
	}
	owned := launch
	owned.Args = slices.Clone(launch.Args)
	// Each run has a private, unpredictable directory. Early launch failures
	// write not-started; exec transfers responsibility to the group supervisor.
	prefix := `set -eu
id=$1
run="$HOME/` + wslConfigBase + `/$id/runs/` + run + `"
(umask 077; mkdir -p "${run%/*}"; mkdir "$run")
trap 'printf "not-started\n" > "$run/receipt"' 0
`
	script := strings.TrimPrefix(wslScript, "set -eu\n")
	script = strings.TrimSuffix(script, `exec "$claude" "$@"`)
	quoted := "'" + strings.ReplaceAll(wslOwnedScript, "'", `'"'"'`) + "'"
	owned.Args[index] = prefix + script + "exec sh -c " + quoted + ` snow-owned "$run" "$claude" "$@"`
	args := append(slices.Clone(program.Args), "-e", "sh", "-c", wslReceiptScript, "snow-receipt", p.ID, run)
	receipt := shell.Launch{Path: program.Path, Args: args, Hide: []string{"WSLENV"}}
	return owned, receipt, nil
}
