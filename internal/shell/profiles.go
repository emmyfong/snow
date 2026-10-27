// Package shell finds the shells a pane can run and prepares them to report
// their current folder. It imports no other Snow package.
package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"
)

// Kind says which shell a profile runs. It decides how Snow injects the
// folder hook.
type Kind int

// Shell kinds.
const (
	KindOther Kind = iota // no folder hook
	KindBash
	KindZsh
	KindPwsh // PowerShell 7 or Windows PowerShell
	KindCmd
	KindWSL
	// KindClaude runs Claude Code, natively or, when Distro is set, inside
	// a WSL distribution. It is an agent profile: no folder hook.
	KindClaude
)

// Profile is one shell Snow can start in a pane.
type Profile struct {
	Name   string // shown to the user, for example "Ubuntu"
	Kind   Kind
	Path   string   // the program to run
	Args   []string // arguments before any Snow additions
	Distro string   // the WSL distribution, for KindWSL and a WSL KindClaude
	// CmdLine, when set, is the whole command line on Windows, passed as
	// it is: a .cmd program runs through cmd, which reads its own quotes.
	CmdLine string
}

// IsAgent reports whether p runs a coding agent rather than a shell.
func (p Profile) IsAgent() bool { return p.Kind == KindClaude }

// Detect returns the shells available on this machine, most preferred first.
func Detect() []Profile { return detect(currentSystem()) }

// system holds the OS lookups Detect needs, so tests can supply them.
type system struct {
	goos       string
	lookPath   func(string) (string, error)
	wslList    func() ([]byte, error)
	wslRunning func() ([]string, error)
	// claudeIn reports whether claude is on the user's PATH inside a WSL
	// distribution, asked through the wsl.exe at the given path. An error
	// means the distribution gave no answer.
	claudeIn func(wsl, distro string) (bool, error)
	shellEnv string
}

func currentSystem() system {
	return system{
		goos:       runtime.GOOS,
		lookPath:   exec.LookPath,
		wslList:    func() ([]byte, error) { return exec.Command("wsl.exe", "-l", "-q").Output() },
		wslRunning: RunningDistros,
		claudeIn:   claudeInDistro,
		shellEnv:   os.Getenv("SHELL"),
	}
}

// detect lists the shells, then the Claude Code profiles, so the default
// stays a shell.
func detect(sys system) []Profile {
	shells := detectShells(sys)
	var distros []string
	for _, p := range shells {
		if p.Kind == KindWSL {
			distros = append(distros, p.Distro)
		}
	}
	return append(shells, detectClaude(sys, distros)...)
}

func detectShells(sys system) []Profile {
	if sys.goos != "windows" {
		return []Profile{unixProfile(sys.shellEnv)}
	}
	var out []Profile
	for _, c := range []struct {
		exe, name string
		kind      Kind
		args      []string
	}{
		{"pwsh.exe", "PowerShell", KindPwsh, []string{"-NoLogo"}},
		{"powershell.exe", "Windows PowerShell", KindPwsh, []string{"-NoLogo"}},
		{"cmd.exe", "Command Prompt", KindCmd, nil},
	} {
		if path, err := sys.lookPath(c.exe); err == nil {
			out = append(out, Profile{Name: c.name, Kind: c.kind, Path: path, Args: c.args})
		}
	}
	wsl, err := sys.lookPath("wsl.exe")
	if err != nil {
		return out
	}
	// wsl.exe exits non-zero when no distribution is installed.
	list, err := sys.wslList()
	if err != nil {
		return out
	}
	for _, d := range parseWSLList(list) {
		out = append(out, Profile{Name: d, Kind: KindWSL, Path: wsl, Args: []string{"-d", d}, Distro: d})
	}
	return out
}

func unixProfile(shellEnv string) Profile {
	if shellEnv == "" {
		shellEnv = "/bin/sh"
	}
	name := filepath.Base(shellEnv)
	kind := KindOther
	switch name {
	case "bash":
		kind = KindBash
	case "zsh":
		kind = KindZsh
	}
	return Profile{Name: name, Kind: kind, Path: shellEnv}
}

// parseWSLList decodes the output of `wsl.exe -l -q`, which is UTF-16LE, and
// drops Docker Desktop's internal distributions, which are not shells.
func parseWSLList(b []byte) []string {
	units := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		units = append(units, uint16(b[i])|uint16(b[i+1])<<8)
	}
	text := strings.TrimPrefix(string(utf16.Decode(units)), "\uFEFF")
	var out []string
	for line := range strings.Lines(text) {
		name := strings.TrimSpace(strings.Trim(line, "\x00"))
		if name == "" || strings.HasPrefix(name, "docker-desktop") {
			continue
		}
		out = append(out, name)
	}
	return out
}

// wslTimeout bounds a wsl.exe query: while WSL shuts down it can hang.
const wslTimeout = 3 * time.Second

// RunningDistros lists the WSL distributions running now. wsl.exe exits
// non-zero when none runs, which is still an answer: an empty list. An
// error means there is no answer, because wsl.exe is missing, could not
// start, or hung.
func RunningDistros() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), wslTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "wsl.exe", "-l", "-q", "--running")
	hideWindow(cmd)
	out, err := cmd.Output()
	if _, exited := errors.AsType[*exec.ExitError](err); exited && ctx.Err() == nil {
		return parseWSLList(out), nil
	}
	if err != nil {
		return nil, fmt.Errorf("list running WSL distributions: %w", err)
	}
	return parseWSLList(out), nil
}
