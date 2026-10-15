package server

import (
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/emmyfong/snow/internal/shell"
)

var testProfiles = []shell.Profile{
	{Name: "pwsh", Kind: shell.KindPwsh, Path: "pwsh"},
	{Name: "Ubuntu", Kind: shell.KindWSL, Path: "wsl.exe", Args: []string{"-d", "Ubuntu"}, Distro: "Ubuntu"},
}

func TestLauncherPicksProfileByName(t *testing.T) {
	l := detectedLauncher(testProfiles, shell.Hooks{})
	if got := l.profiles(); !slices.Equal(got, []string{"pwsh", "Ubuntu"}) {
		t.Fatalf("profiles %v, want [pwsh Ubuntu]", got)
	}
	if _, name, err := l.spec("", ""); err != nil || name != "pwsh" {
		t.Fatalf("default profile %q, %v; want pwsh", name, err)
	}
	if spec, name, err := l.spec("Ubuntu", ""); err != nil || name != "Ubuntu" || spec.Path != "wsl.exe" {
		t.Fatalf("Ubuntu: %+v %q %v", spec, name, err)
	}
	if _, _, err := l.spec("fish", ""); !errors.Is(err, ErrUnknownProfile) {
		t.Fatalf("unknown profile error %v, want ErrUnknownProfile", err)
	}
}

// TestWSLKeepsLinuxFolder: a WSL pane reports Linux folders; a split from it
// must start there, even on a Windows server where the path does not exist.
func TestWSLKeepsLinuxFolder(t *testing.T) {
	l := detectedLauncher(testProfiles, shell.Hooks{})
	spec, _, err := l.spec("Ubuntu", "/home/emmy/api")
	if err != nil {
		t.Fatal(err)
	}
	i := slices.Index(spec.Args, "--cd")
	if i < 0 || i+1 >= len(spec.Args) || spec.Args[i+1] != "/home/emmy/api" {
		t.Fatalf("args %q, want --cd /home/emmy/api", spec.Args)
	}
}

func TestNativeProfileFallsBackToHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	l := detectedLauncher(testProfiles, shell.Hooks{})
	spec, _, err := l.spec("pwsh", "/no/such/folder-snow-test")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Dir != home {
		t.Fatalf("dir %q, want the home folder %q", spec.Dir, home)
	}
}
