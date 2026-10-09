// Command snow is a terminal multiplexer: sessions that keep running after
// you close the window, on Windows and Linux.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/emmyfong/snow/internal/client"
	"github.com/emmyfong/snow/internal/server"
	"github.com/emmyfong/snow/internal/transport"
	"github.com/emmyfong/snow/pkg/api"
)

// version is set at release time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `usage:
  snow           start a new session
  snow <name>    attach to session <name>, or create it
  snow version   print the version (also -v, --version)
  snow help      show this help (also -h, --help)

The names version, help, and server are commands, not sessions.
Inside a session, press Ctrl+b then d to detach.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// kind is what a command line asks snow to do.
type kind int

const (
	kindAttach kind = iota
	kindServer
	kindVersion
	kindHelp
)

// command is a parsed command line.
type command struct {
	kind    kind
	session string // for kindAttach; "" creates a numbered session
}

func parseArgs(args []string) (command, error) {
	switch {
	case len(args) == 0:
		return command{kind: kindAttach}, nil
	case len(args) > 1:
		return command{}, errors.New("too many arguments")
	}
	switch a := args[0]; a {
	case "version", "-v", "--version":
		return command{kind: kindVersion}, nil
	case "help", "-h", "--help":
		return command{kind: kindHelp}, nil
	case "server":
		return command{kind: kindServer}, nil
	default:
		if strings.HasPrefix(a, "-") {
			return command{}, fmt.Errorf("unknown option %s", a)
		}
		if strings.ContainsAny(a, `/\`) {
			return command{}, fmt.Errorf("session names cannot contain / or \\: %s", a)
		}
		return command{kind: kindAttach, session: a}, nil
	}
}

func run(args []string, stdout, stderr io.Writer) int {
	cmd, err := parseArgs(args)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "snow: %v\n%s", err, usage)
		return 2
	}
	switch cmd.kind {
	case kindVersion:
		_, _ = fmt.Fprintf(stdout, "snow %s\n", version)
		return 0
	case kindHelp:
		_, _ = fmt.Fprint(stdout, usage)
		return 0
	case kindServer:
		return serve(stderr)
	default:
		return attach(cmd.session, stdout, stderr)
	}
}

func socketPath(stderr io.Writer) (string, bool) {
	path, err := transport.DefaultPath()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "snow:", err)
		return "", false
	}
	return path, true
}

// serve runs the background server. Its output goes to a log file next to
// the socket, because a detached process has no terminal.
func serve(stderr io.Writer) int {
	path, ok := socketPath(stderr)
	if !ok {
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		_, _ = fmt.Fprintln(stderr, "snow:", err)
		return 1
	}
	log, logFile, err := server.OpenLog(filepath.Dir(path))
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "snow:", err)
		return 1
	}
	defer func() { _ = logFile.Close() }()
	// SIGTERM is what kill and service managers send; stopping cleanly
	// removes the socket and ends the shells.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := server.Run(ctx, server.Options{Version: version, Log: log}, path); err != nil {
		log.Error("server stopped", "err", err)
		_, _ = fmt.Fprintln(stderr, "snow:", err)
		return 1
	}
	return 0
}

func attach(name string, stdout, stderr io.Writer) int {
	path, ok := socketPath(stderr)
	if !ok {
		return 1
	}
	res, err := client.Run(client.Options{
		Version: version,
		Socket:  path,
		Session: name,
		Start: func() error {
			exe, err := os.Executable()
			if err != nil {
				return err
			}
			return client.StartServer(exe, "server")
		},
	})
	var mismatch *api.VersionMismatchError
	switch {
	case errors.As(err, &mismatch):
		_, _ = fmt.Fprintf(stderr, "snow: the running server is version %s; this is %s. Exit its sessions to restart it.\n",
			mismatch.ServerVersion, mismatch.ClientVersion)
		return 1
	case err != nil:
		_, _ = fmt.Fprintln(stderr, "snow:", err)
		return 1
	case res.Err != nil:
		_, _ = fmt.Fprintln(stderr, "snow:", res.Err)
		return 1
	case res.Detached:
		_, _ = fmt.Fprintf(stdout, "[detached from %s]\n", res.Session)
	case res.Ended:
		_, _ = fmt.Fprintf(stdout, "[%s ended]\n", res.Session)
	}
	return 0
}
