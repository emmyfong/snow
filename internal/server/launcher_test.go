package server

import (
	"errors"
	"os"
	"slices"
	"strings"
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

func TestPreferProfile(t *testing.T) {
	names := func(ps []shell.Profile) string {
		var out []string
		for _, p := range ps {
			out = append(out, p.Name)
		}
		return strings.Join(out, ",")
	}
	abc := []shell.Profile{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	tests := []struct{ prefer, want string }{
		{"", "a,b,c"}, {"a", "a,b,c"}, {"b", "b,a,c"}, {"c", "c,a,b"}, {"nope", "a,b,c"},
	}
	for _, tt := range tests {
		if got := names(preferProfile(abc, tt.prefer)); got != tt.want {
			t.Errorf("preferProfile(%q) = %s, want %s", tt.prefer, got, tt.want)
		}
	}
	if got := names(abc); got != "a,b,c" {
		t.Errorf("preferProfile changed its input: %s", got)
	}
}

func TestDefaultProfileComesFirst(t *testing.T) {
	l := detectedLauncher(preferProfile(testProfiles, testProfiles[len(testProfiles)-1].Name), shell.Hooks{})
	_, name, err := l.spec("", "")
	if err != nil || name != testProfiles[len(testProfiles)-1].Name {
		t.Fatalf("empty profile = %q, %v; want the preferred profile", name, err)
	}
}
