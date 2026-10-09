// Package shell finds the shells a pane can run and prepares them to report
// their current folder. It imports no other Snow package.
package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
)

// Profile is one shell Snow can start in a pane.
type Profile struct {
	Name   string // shown to the user, for example "Ubuntu"
	Kind   Kind
	Path   string   // the program to run
	Args   []string // arguments before any Snow additions
	Distro string   // the WSL distribution, for KindWSL only
}

// Detect returns the shells available on this machine, most preferred first.
func Detect() []Profile { return detect(currentSystem()) }

// system holds the OS lookups Detect needs, so tests can supply them.
type system struct {
	goos     string
	lookPath func(string) (string, error)
	wslList  func() ([]byte, error)
	shellEnv string
}

func currentSystem() system {
	return system{
		goos:     runtime.GOOS,
		lookPath: exec.LookPath,
		wslList:  func() ([]byte, error) { return exec.Command("wsl.exe", "-l", "-q").Output() },
		shellEnv: os.Getenv("SHELL"),
	}
}

func detect(sys system) []Profile {
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
