package server

import (
	"os"
	"slices"
	"testing"

	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/internal/tool"
)

func TestDistroFolder(t *testing.T) {
	tests := []struct {
		name   string
		distro string
		dir    string
		want   string
		ok     bool
	}{
		{"Linux path", "snowtest", "/home/u/api", "/home/u/api", true},
		{"UNC local host", "snowtest", `\\wsl.localhost\snowtest\home\u\api`, "/home/u/api", true},
		{"UNC old host form", "snowtest", `\\wsl$\snowtest\home\u\api`, "/home/u/api", true},
		{"UNC distribution case", "SnowTest", `\\wsl.localhost\snowtest\home\u\api`, "/home/u/api", true},
		{"another distribution", "snowtest", `\\wsl.localhost\other\home\u\api`, "", false},
		{"Windows drive", "snowtest", `C:\snow\api`, "", false},
		{"empty folder", "snowtest", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := distroFolder(tt.distro, tt.dir)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("distroFolder(%q, %q) = %q, %v; want %q, %v", tt.distro, tt.dir, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestProfileDistro(t *testing.T) {
	wsl := shell.Profile{Name: "snowtest", Kind: shell.KindWSL, Path: "wsl.exe", Args: []string{"-d", "snowtest"}, Distro: "snowtest"}
	windows := shell.Profile{Name: "PowerShell", Kind: shell.KindPwsh, Path: "pwsh.exe"}
	withWSLDefault := &Server{launch: detectedLauncher([]shell.Profile{wsl, windows}, shell.Hooks{})}
	withWindowsDefault := &Server{launch: detectedLauncher([]shell.Profile{windows, wsl}, shell.Hooks{})}
	nilLaunch := &Server{}
	tests := []struct {
		name    string
		server  *Server
		profile string
		want    string
		ok      bool
	}{
		{"default WSL profile", withWSLDefault, "", "snowtest", true},
		{"default Windows profile", withWindowsDefault, "", "", false},
		{"named WSL profile", withWindowsDefault, "snowtest", "snowtest", true},
		{"unknown profile", withWSLDefault, "missing", "", false},
		{"nil launcher fields", nilLaunch, "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.server.profileDistro(tt.profile)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("profileDistro(%q) = %q, %v; want %q, %v", tt.profile, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestFolderForSameDistroUNC(t *testing.T) {
	p := shell.Profile{Name: "snowtest", Kind: shell.KindWSL, Path: "wsl.exe", Args: []string{"-d", "snowtest"}, Distro: "snowtest"}
	got := folderFor(p, `\\wsl.localhost\snowtest\home\u\api`)
	if got != "/home/u/api" {
		t.Fatalf("folderFor same-distro UNC = %q, want /home/u/api", got)
	}
}

func TestFolderForOtherDistroStartsHome(t *testing.T) {
	p := shell.Profile{Name: "snowtest", Kind: shell.KindWSL, Path: "wsl.exe", Args: []string{"-d", "snowtest"}, Distro: "snowtest"}
	folder := folderFor(p, `\\wsl.localhost\other\home\u\api`)
	launch := p.Launch(shell.Hooks{}, folder)
	i := slices.Index(launch.Args, "--cd")
	if i < 0 || i+1 >= len(launch.Args) || launch.Args[i+1] != "~" {
		t.Fatalf("other-distro UNC should explicitly select the distro home with --cd ~: %q", launch.Args)
	}
}

func TestChatFolderSameDistroUNC(t *testing.T) {
	wsl := shell.Profile{Name: "snowtest", Kind: shell.KindWSL, Path: "wsl.exe", Args: []string{"-d", "snowtest"}, Distro: "snowtest"}
	s := &Server{launch: detectedLauncher([]shell.Profile{wsl}, shell.Hooks{})}
	want := tool.Folder{Path: "/home/u/api", Distro: "snowtest"}
	if got := s.chatFolder("snowtest", `\\wsl.localhost\snowtest\home\u\api`); got != want {
		t.Fatalf("chatFolder same-distro UNC = %+v, want %+v", got, want)
	}
}

func TestChatFolderOtherDistroKeepsCurrentFallback(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("get home folder: %v", err)
	}
	wsl := shell.Profile{Name: "snowtest", Kind: shell.KindWSL, Path: "wsl.exe", Args: []string{"-d", "snowtest"}, Distro: "snowtest"}
	s := &Server{launch: detectedLauncher([]shell.Profile{wsl}, shell.Hooks{})}
	want := tool.Folder{Path: home}
	if got := s.chatFolder("snowtest", `\\wsl.localhost\snow-test-other-9f3a\home\u\api`); got != want {
		t.Fatalf("chatFolder other-distro UNC = %+v, want existing host fallback %+v", got, want)
	}
}

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
		{"WSL UNC folder in same distribution", wsl, `\\wsl.localhost\snowtest\home\u\api`, "/home/u/api"},
		{"WSL UNC folder in another distribution", wsl, `\\wsl.localhost\other\home\u\api`, "~"},
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
