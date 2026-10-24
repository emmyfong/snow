package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/internal/signin"
)

const authUsage = `usage:
  snow auth login <provider>    check an API key and save it
  snow auth logout <provider>   remove a saved API key
  snow auth status              show which providers have a key

Providers: anthropic, openai. Keys go to the OS credential store, or to
~/.snow/credentials where the system has none. An environment variable
such as ANTHROPIC_API_KEY takes precedence over a saved key.
`

// authProviders lists the providers `snow auth` knows. Tests point the
// key checks at a fake server.
var authProviders = signin.Providers

// authAction is what `snow auth` does.
type authAction string

const (
	authLogin  authAction = "login"
	authLogout authAction = "logout"
	authStatus authAction = "status"
)

// authCommand is a parsed `snow auth` command line.
type authCommand struct {
	action   authAction
	provider string
}

// parseAuth reads the arguments after "auth". --api-key is accepted for
// login; it is the only way to sign in until Sign in with ChatGPT (#128).
func parseAuth(args []string) (authCommand, error) {
	if len(args) == 0 {
		return authCommand{}, errors.New("auth needs login, logout, or status")
	}
	var rest []string
	for _, a := range args[1:] {
		switch {
		case a == "--api-key" && args[0] == string(authLogin):
		case strings.HasPrefix(a, "-"):
			return authCommand{}, fmt.Errorf("unknown option %s", a)
		default:
			rest = append(rest, a)
		}
	}
	switch a := authAction(args[0]); a {
	case authStatus:
		if len(rest) > 0 {
			return authCommand{}, errors.New("auth status takes no provider")
		}
		return authCommand{action: a}, nil
	case authLogin, authLogout:
		if len(rest) != 1 {
			return authCommand{}, fmt.Errorf("auth %s needs one provider", a)
		}
		if _, err := signin.Find(authProviders(), rest[0]); err != nil {
			return authCommand{}, err
		}
		return authCommand{action: a, provider: rest[0]}, nil
	default:
		return authCommand{}, fmt.Errorf("unknown auth command %s", a)
	}
}

// checkTimeout bounds the key check, one small request to the vendor.
const checkTimeout = 30 * time.Second

func runAuth(cmd authCommand, stdout, stderr io.Writer) int {
	dir, err := credential.DefaultDir()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "snow:", err)
		return 1
	}
	store := credential.New(dir, func(msg string) { _, _ = fmt.Fprintln(stdout, msg) })
	if cmd.action == authStatus {
		return authStatusCmd(store, stdout)
	}
	// parseAuth has already found the provider.
	p, err := signin.Find(authProviders(), cmd.provider)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "snow:", err)
		return 2
	}
	in := bufio.NewReader(stdin)
	if cmd.action == authLogout {
		return authLogoutCmd(store, p, in, stdout, stderr)
	}
	return authLoginCmd(store, p, in, stdout, stderr)
}

func authLoginCmd(store signin.Store, p signin.Provider, in *bufio.Reader, stdout, stderr io.Writer) int {
	_, _ = fmt.Fprintf(stdout, "Create a %s API key at %s\n", p.Title, p.KeyURL)
	secret, err := readSecret(in, stderr, "Paste the key (it does not show): ")
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "snow:", err)
		return 1
	}
	k, err := credential.NewKey(strings.TrimSpace(secret))
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "snow:", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "Checking the key with %s...\n", p.Title)
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	res, err := p.Login(ctx, store, k)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "snow:", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "Saved the %s key in the %s.\n", p.Name, res.Source)
	if res.Shadowed {
		_, _ = fmt.Fprintf(stdout, "%s is set, and Snow uses it before the saved key. Unset it to use the saved key.\n", credential.EnvVar(p.Name))
	}
	return 0
}

func authLogoutCmd(store signin.Store, p signin.Provider, in *bufio.Reader, stdout, stderr io.Writer) int {
	switch stored, err := p.Stored(store); {
	case err != nil:
		_, _ = fmt.Fprintf(stderr, "snow: %v. Unset it to sign out.\n", err)
		return 1
	case !stored:
		_, _ = fmt.Fprintf(stdout, "No %s key is saved.\n", p.Name)
		return 0
	}
	_, _ = fmt.Fprintf(stderr, "Remove the saved %s key? [y/N] ", p.Name)
	answer, err := in.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		_, _ = fmt.Fprintln(stderr, "snow:", err)
		return 1
	}
	if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
		_, _ = fmt.Fprintln(stdout, "Kept the key.")
		return 0
	}
	if err := p.Logout(store); err != nil {
		_, _ = fmt.Fprintln(stderr, "snow:", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "Removed the %s key.\n", p.Name)
	return 0
}

func authStatusCmd(store signin.Store, stdout io.Writer) int {
	for _, s := range signin.Statuses(authProviders(), store) {
		var line string
		switch {
		case s.Connected():
			line = fmt.Sprintf("API key (%s)", s.Source)
		case errors.Is(s.Err, credential.ErrNoKey):
			line = "no key. Run: snow auth login " + s.Provider.Name
		default:
			line = fmt.Sprintf("cannot read the key: %v", s.Err)
		}
		_, _ = fmt.Fprintf(stdout, "%-10s %s\n", s.Provider.Name, line)
	}
	return 0
}

// readSecret reads one line without showing it when stdin is a terminal.
// From a pipe, for example `echo $KEY | snow auth login openai`, it reads
// the first line. The prompt goes to stderr, so it shows even when stdout
// goes to a file.
func readSecret(in *bufio.Reader, stderr io.Writer, prompt string) (string, error) {
	_, _ = fmt.Fprint(stderr, prompt)
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		b, err := readHidden(int(f.Fd()))
		_, _ = fmt.Fprintln(stderr)
		if err != nil {
			return "", fmt.Errorf("read the key: %w", err)
		}
		return string(b), nil
	}
	line, err := in.ReadString('\n')
	if err != nil && (!errors.Is(err, io.EOF) || line == "") {
		return "", errors.New("read the key: no input")
	}
	return line, nil
}

// readHidden is term.ReadPassword that gives the terminal back on Ctrl+C:
// ReadPassword turns echo off and restores it only when it returns, but
// Ctrl+C ends the process first.
func readHidden(fd int) ([]byte, error) {
	state, err := term.GetState(fd)
	if err != nil {
		return nil, fmt.Errorf("read the terminal state: %w", err)
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	done := make(chan struct{})
	defer func() {
		signal.Stop(stop)
		close(done)
	}()
	go func() {
		select {
		case <-stop:
			_ = term.Restore(fd, state)
			os.Exit(130)
		case <-done:
		}
	}()
	return term.ReadPassword(fd)
}
