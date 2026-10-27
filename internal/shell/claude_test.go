package shell

import (
	"errors"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestDetectClaude(t *testing.T) {
	tests := []struct {
		name    string
		goos    string
		found   []string // program names lookPath finds
		running string   // wsl.exe -l -q --running
		inside  []string // distros where claude is on the user's PATH
		failing string   // a distro that does not answer
		want    []Profile
	}{
		{name: "found on PATH", goos: "linux", found: []string{"claude"},
			want: []Profile{{Name: "Claude Code", Kind: KindClaude, Path: "/bin/claude"}}},
		{name: "not found", goos: "linux", want: nil},
		{name: "claude.exe on Windows", goos: "windows", found: []string{"claude.exe", "claude.cmd", "cmd.exe"},
			want: []Profile{{Name: "Claude Code", Kind: KindClaude, Path: `C:\bin\claude.exe`}}},
		{name: ".cmd on Windows runs through cmd", goos: "windows", found: []string{"claude.cmd", "cmd.exe"},
			want: []Profile{{Name: "Claude Code", Kind: KindClaude, Path: `C:\bin\cmd.exe`, Args: []string{"/D", "/S", "/C", `"C:\bin\claude.cmd"`},
				CmdLine: `"C:\bin\cmd.exe" /D /S /C ""C:\bin\claude.cmd""`}}},
		{name: "inside a WSL distro", goos: "windows", found: []string{"wsl.exe"},
			running: "Ubuntu\r\nDebian\r\n", inside: []string{"Ubuntu"},
			want: []Profile{{Name: "Claude Code (Ubuntu)", Kind: KindClaude, Path: `C:\bin\wsl.exe`, Args: []string{"-d", "Ubuntu"}, Distro: "Ubuntu"}}},
		{name: "a distro that does not run is not asked", goos: "windows", found: []string{"wsl.exe"},
			running: "", inside: []string{"Ubuntu"}, want: nil},
		{name: "a distro that does not answer is skipped", goos: "windows", found: []string{"wsl.exe"},
			running: "Ubuntu\r\nDebian\r\n", inside: []string{"Debian"}, failing: "Ubuntu",
			want: []Profile{{Name: "Claude Code (Debian)", Kind: KindClaude, Path: `C:\bin\wsl.exe`, Args: []string{"-d", "Debian"}, Distro: "Debian"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				mu    sync.Mutex
				asked []string
			)
			sys := system{
				goos: tt.goos,
				lookPath: func(name string) (string, error) {
					if !slices.Contains(tt.found, name) {
						return "", errors.New("not found")
					}
					if tt.goos == "windows" {
						return `C:\bin\` + name, nil
					}
					return "/bin/" + name, nil
				},
				wslList:    func() ([]byte, error) { return utf16le("Ubuntu\r\nDebian\r\n", false), nil },
				wslRunning: func() ([]string, error) { return parseWSLList(utf16le(tt.running, false)), nil },
				claudeIn: func(_, distro string) (bool, error) {
					mu.Lock()
					asked = append(asked, distro)
					mu.Unlock()
					if distro == tt.failing {
						return false, errors.New("timed out")
					}
					return slices.Contains(tt.inside, distro), nil
				},
			}
			got := detectClaude(sys, parseWSLList(utf16le("Ubuntu\r\nDebian\r\n", false)))
			if len(got) != len(tt.want) {
				t.Fatalf("detectClaude() = %+v, want %+v", got, tt.want)
			}
			for i := range got {
				g, w := got[i], tt.want[i]
				if g.Name != w.Name || g.Kind != w.Kind || g.Path != w.Path || !slices.Equal(g.Args, w.Args) || g.Distro != w.Distro || g.CmdLine != w.CmdLine {
					t.Fatalf("profile %d = %+v, want %+v", i, g, w)
				}
				if !g.IsAgent() {
					t.Fatalf("profile %d is not marked as an agent", i)
				}
			}
			if tt.running == "" && len(asked) > 0 {
				t.Fatalf("asked distros %q that do not run; starting one takes seconds", asked)
			}
		})
	}
}

// TestDetectListsClaudeLast: the shells come first, so the default profile
// stays a shell.
func TestDetectListsClaudeLast(t *testing.T) {
	sys := system{
		goos:     "linux",
		shellEnv: "/bin/bash",
		lookPath: func(name string) (string, error) { return "/bin/" + name, nil },
	}
	if got := names(detect(sys)); !slices.Equal(got, []string{"bash", "Claude Code"}) {
		t.Fatalf("detect() = %q, want [bash Claude Code]", got)
	}
}

func TestClaudeFor(t *testing.T) {
	native := Profile{Name: "Claude Code", Kind: KindClaude, Path: "/bin/claude"}
	ubuntu := Profile{Name: "Claude Code (Ubuntu)", Kind: KindClaude, Path: "wsl.exe", Args: []string{"-d", "Ubuntu"}, Distro: "Ubuntu"}
	profiles := []Profile{
		{Name: "pwsh", Kind: KindPwsh},
		{Name: "Ubuntu", Kind: KindWSL, Distro: "Ubuntu"},
		{Name: "Debian", Kind: KindWSL, Distro: "Debian"},
		native, ubuntu,
	}
	tests := []struct {
		host, want string
		ok         bool
	}{
		{"pwsh", "Claude Code", true},
		{"Ubuntu", "Claude Code (Ubuntu)", true},
		{"Debian", "", false}, // no claude inside Debian; never the Windows one
		{"Claude Code (Ubuntu)", "Claude Code (Ubuntu)", true},
		{"", "Claude Code", true},
	}
	for _, tt := range tests {
		got, ok := ClaudeFor(profiles, tt.host)
		if ok != tt.ok || got.Name != tt.want {
			t.Errorf("ClaudeFor(%q) = %q, %v; want %q, %v", tt.host, got.Name, ok, tt.want, tt.ok)
		}
	}
}

func TestLaunchClaude(t *testing.T) {
	h := Hooks{Dir: `C:\Users\emmy\AppData\Local\snow\shell`}
	t.Run("native runs claude in the folder", func(t *testing.T) {
		l := Profile{Kind: KindClaude, Path: "/bin/claude"}.Launch(h, "/home/emmy/api")
		if l.Path != "/bin/claude" || len(l.Args) != 0 || l.Dir != "/home/emmy/api" {
			t.Fatalf("Launch() = %+v", l)
		}
	})
	t.Run("WSL runs claude through the login shell", func(t *testing.T) {
		t.Setenv("WSLENV", "")
		p := Profile{Kind: KindClaude, Path: "wsl.exe", Args: []string{"-d", "Ubuntu"}, Distro: "Ubuntu"}
		l := p.Launch(h, "/home/emmy/api")
		want := []string{"-d", "Ubuntu", "--cd", "/home/emmy/api", "-e", "sh", "/mnt/c/Users/emmy/AppData/Local/snow/shell/claude.sh"}
		if !slices.Equal(l.Args, want) || l.Dir != "" || !slices.Contains(l.Env, "WSLENV=SNOW:SNOW_PANE:SNOW_SOCKET") {
			t.Fatalf("Launch() = %+v, want args %q", l, want)
		}
	})
}

func TestHostShell(t *testing.T) {
	profiles := []Profile{
		{Name: "bash", Kind: KindBash},
		{Name: "Ubuntu", Kind: KindWSL, Distro: "Ubuntu"},
		{Name: "Claude Code", Kind: KindClaude},
		{Name: "Claude Code (Ubuntu)", Kind: KindClaude, Distro: "Ubuntu"},
	}
	for host, want := range map[string]string{"Claude Code": "bash", "Claude Code (Ubuntu)": "Ubuntu", "bash": "bash", "Ubuntu": "Ubuntu"} {
		p, _ := Find(profiles, host)
		if got, ok := HostShell(profiles, p); !ok || got.Name != want {
			t.Errorf("HostShell(%q) = %q, %v; want %q", host, got.Name, ok, want)
		}
	}
}

// TestFoundClaude: startup files may print lines of their own before the
// path.
func TestFoundClaude(t *testing.T) {
	tests := []struct {
		out  string
		want bool
	}{
		{"/home/emmy/.local/bin/claude\n", true},
		{"Welcome to Ubuntu\n/home/emmy/.nvm/versions/node/v22/bin/claude\n", true},
		{"", false},
		{"claude: aliased to claude --verbose\n", false},
		{"/usr/bin/claude-helper\n", false},
	}
	for _, tt := range tests {
		if got := foundClaude(tt.out); got != tt.want {
			t.Errorf("foundClaude(%q) = %v, want %v", tt.out, got, tt.want)
		}
	}
}

// TestLaunchClaudeCmd: a claude.cmd runs through cmd with the whole command
// line given as it is, so a folder such as "Program Files (x86)" keeps its
// parentheses and spaces.
func TestLaunchClaudeCmd(t *testing.T) {
	p := Profile{Kind: KindClaude, Path: `C:\Windows\cmd.exe`, Args: []string{"/D", "/S", "/C", `"C:\a b (x86)\claude.cmd"`},
		CmdLine: `"C:\Windows\cmd.exe" /D /S /C ""C:\a b (x86)\claude.cmd""`}
	if l := p.Launch(Hooks{}, `C:\src`); l.CmdLine != p.CmdLine || l.Dir != `C:\src` {
		t.Fatalf("Launch() = %+v, want the profile's command line in C:\\src", l)
	}
}

// TestClaudeProbeTimeout: asking a WSL distribution for claude may start
// it and run the user's startup files, so it gets far longer than a list
// query.
func TestClaudeProbeTimeout(t *testing.T) {
	if claudeProbeTimeout < 15*time.Second || wslTimeout > 5*time.Second {
		t.Fatalf("probe %v, list %v; want a probe of 15 s or more and a short list query", claudeProbeTimeout, wslTimeout)
	}
}
