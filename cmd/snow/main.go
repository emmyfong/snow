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
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/config"
	"github.com/emmyfong/snow/internal/server"
	"github.com/emmyfong/snow/internal/store"
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

// stdin answers questions asked before the screen starts, such as whether
// to restart an older server. Tests replace it.
var stdin io.Reader = os.Stdin

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
		if err := nested(); err != nil {
			_, _ = fmt.Fprintln(stderr, "snow:", err)
			return 1
		}
		return attach(cmd.session, stdout, stderr)
	}
}

// nested returns an error if this process runs inside a snow pane, which the
// server marks with SNOW. A second client inside the first would send
// commands to the wrong shell. An empty SNOW counts as unset.
func nested() error {
	v := os.Getenv(transport.EnvName)
	if v == "" {
		return nil
	}
	where := "already inside a snow session"
	if _, pane, ok := transport.ParsePaneEnv(v); ok {
		where = fmt.Sprintf("%s (pane %d)", where, pane)
	}
	return fmt.Errorf("%s. Detach with Ctrl+b d, then run snow again. To start one here anyway, first run: %s",
		where, unsetHint())
}

// loadConfig reads ~/.snow/config.toml. A missing file gives the defaults;
// a bad one is reported with its line and key.
func loadConfig(stderr io.Writer) (config.Config, bool) {
	path, err := config.DefaultPath()
	if err == nil {
		var cfg config.Config
		if cfg, err = config.Load(path); err == nil {
			if err = checkKeys(path, cfg); err == nil {
				return cfg, true
			}
		}
	}
	_, _ = fmt.Fprintln(stderr, "snow:", err)
	return config.Config{}, false
}

// checkKeys builds the key table once, so a binding it rejects stops snow
// with the file and line, before any server starts.
func checkKeys(path string, cfg config.Config) error {
	_, err := keys.New(cfg.Keys)
	var ke *keys.Error
	if !errors.As(err, &ke) {
		return err
	}
	where := []string{"keys", "prefix"}
	if ke.Setting == "keys.bind" {
		where = []string{"keys", "bind", ke.Key}
	}
	return config.Locate(path, where, ke.Err.Error(), ke)
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
	cfg, ok := loadConfig(stderr)
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
	// Without a home folder sessions still run; they are only not saved.
	var sessions string
	if cfg.Sessions.Save {
		if sessions, err = store.DefaultDir(); err != nil {
			log.Warn("sessions will not be saved", "err", err)
		}
	}
	savedLines := cfg.Sessions.Lines
	if savedLines == 0 {
		savedLines = -1 // the config's 0 is "no lines", the server's is "default"
	}
	if err := server.Run(ctx, server.Options{
		Version: version, Log: log, SessionsDir: sessions, SavedLines: savedLines,
		Scrollback: cfg.Scrollback.Lines, DefaultProfile: cfg.Profile.Default,
	}, path); err != nil {
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
	cfg, ok := loadConfig(stderr)
	if !ok {
		return 1
	}
	// loadConfig has already found the path; an error here cannot happen.
	cfgPath, _ := config.DefaultPath()
	run := func() (client.Result, error) {
		return client.Run(client.Options{
			Version:    version,
			Config:     &cfg,
			ConfigPath: cfgPath,
			Socket:     path,
			Session:    name,
			Home:       name == "", // plain snow opens the home page
			Start: func() error {
				exe, err := os.Executable()
				if err != nil {
					return err
				}
				return client.StartServer(exe, "server")
			},
		})
	}
	res, err := run()
	var mismatch *api.VersionMismatchError
	if errors.As(err, &mismatch) {
		if !client.AskRestart(stdin, stdout, mismatch) {
			_, _ = fmt.Fprintln(stdout, "Keeping the running server.")
			return 1
		}
		switch rerr := client.RestartServer(path, version); {
		case errors.Is(rerr, client.ErrServerKeptRunning):
			_, _ = fmt.Fprintf(stderr, "snow: the server is older than restarts and kept running. Stop it with:\n  %s\n", stopHint)
			return 1
		case rerr != nil:
			_, _ = fmt.Fprintln(stderr, "snow:", rerr)
			return 1
		}
		res, err = run()
	}
	switch {
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
