package server

import (
	"os"
	"slices"
	"testing"

	"github.com/emmyfong/snow/internal/shell"
)

func TestWSLFolderShapes(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("get home folder: %v", err)
	}
	wsl := shell.Profile{Name: "snowtest", Kind: shell.KindWSL, Path: "wsl.exe", Args: []string{"-d", "snowtest"}, Distro: "snowtest"}
	windows := shell.Profile{Name: "PowerShell", Kind: shell.KindPwsh, Path: "pwsh.exe"}

	// Rows 2 and 3 record the UNC paths that #466 changes. A Windows profile
	// keeps the existing host-folder behavior for the same UNC paths.
	tests := []struct {
		name    string
		profile shell.Profile
		dir     string
		want    string
	}{
		{"WSL Linux folder", wsl, "/home/u/api", "/home/u/api"},
		{"WSL UNC folder in same distribution", wsl, `\\wsl.localhost\snowtest\home\u\api`, home},
		{"WSL UNC folder in another distribution", wsl, `\\wsl.localhost\other\home\u\api`, home},
		{"WSL missing drive folder", wsl, `C:\snow-test-missing-9f3a`, home},
		{"WSL empty folder", wsl, "", home},
		{"Windows UNC folder", windows, `\\wsl.localhost\snowtest\home\u\api`, home},
		{"Windows missing Linux-style folder", windows, "/snow-test-missing/x", home},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			folder := folderFor(tt.profile, tt.dir)
			launch := tt.profile.Launch(shell.Hooks{}, folder)
			got := launch.Dir
			if tt.profile.Kind == shell.KindWSL {
				got = ""
				if i := slices.Index(launch.Args, "--cd"); i >= 0 && i+1 < len(launch.Args) {
					got = launch.Args[i+1]
				}
			}
			if got != tt.want {
				t.Fatalf("folderFor(%q) reaches launch as %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}
