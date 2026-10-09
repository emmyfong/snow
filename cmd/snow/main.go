// Command snow is a terminal multiplexer: sessions that keep running after
// you close the window, on Windows and Linux.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

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
  snow version   print the version
  snow help      show this help

Inside a session, press Ctrl+b then d to detach.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// command is what the arguments ask for.
type command struct {
	kind    string // "attach", "server", "version", "help"
	session string
}

func parseArgs(args []string) (command, bool) {
	switch {
	case len(args) == 0:
		return command{kind: "attach"}, true
	case len(args) > 1:
		return command{}, false
	}
	switch a := args[0]; a {
	case "version":
		return command{kind: "version"}, true
	case "help", "-h", "--help":
		return command{kind: "help"}, true
	case "server":
		return command{kind: "server"}, true
	default:
		if strings.HasPrefix(a, "-") || strings.ContainsAny(a, `/\`) {
			return command{}, false
		}
		return command{kind: "attach", session: a}, true
	}
}

func run(args []string, stdout, stderr io.Writer) int {
	cmd, ok := parseArgs(args)
	if !ok {
		_, _ = fmt.Fprint(stderr, usage)
		return 2
	}
	switch cmd.kind {
	case "version":
		_, _ = fmt.Fprintf(stdout, "snow %s\n", version)
		return 0
	case "help":
		_, _ = fmt.Fprint(stdout, usage)
		return 0
	case "server":
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
	logFile, err := os.OpenFile(filepath.Join(filepath.Dir(path), "server.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "snow:", err)
		return 1
	}
	defer func() { _ = logFile.Close() }()
	log := slog.New(slog.NewTextHandler(logFile, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
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
