package shell

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func TestWriteHooks(t *testing.T) {
	dir := t.TempDir()
	h, err := WriteHooks(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bash.sh", "snow.ps1", "wsl.sh", "zsh/.zshenv", "zsh/.zprofile", "zsh/.zshrc"} {
		b, err := os.ReadFile(filepath.Join(h.Dir, filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("hook %s: %v", name, err)
		}
		if strings.Contains(string(b), "\r\n") {
			t.Fatalf("hook %s has CRLF line endings; shells inside WSL reject them", name)
		}
	}
}

// TestWriteHooksNormalizesLineEndings covers a Windows checkout where git
// turned the embedded scripts into CRLF. bash and zsh reject CR characters.
func TestWriteHooksNormalizesLineEndings(t *testing.T) {
	src := fstest.MapFS{
		"hooks/bash.sh":     {Data: []byte("echo one\r\necho two\r\n")},
		"hooks/zsh/.zshenv": {Data: []byte("a=1\r\n")},
	}
	h, err := writeHooks(src, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bash.sh", "zsh/.zshenv"} {
		b, err := os.ReadFile(filepath.Join(h.Dir, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "\r") {
			t.Fatalf("%s still has CR characters: %q", name, b)
		}
	}
}

func TestLaunch(t *testing.T) {
	h := Hooks{Dir: "/cache/snow/shell"}
	tests := []struct {
		name     string
		profile  Profile
		dir      string
		wantArgs []string
		wantEnv  []string
		wantDir  string
	}{
		{"bash loads the hook as its rcfile",
			Profile{Kind: KindBash, Path: "/bin/bash"}, "/home/u",
			[]string{"--rcfile", filepath.Join("/cache/snow/shell", "bash.sh"), "-i"}, nil, "/home/u"},
		{"zsh reads the hook through ZDOTDIR",
			Profile{Kind: KindZsh, Path: "/bin/zsh"}, "",
			[]string{"-l"}, []string{"ZDOTDIR=" + filepath.Join("/cache/snow/shell", "zsh")}, ""},
		{"powershell runs the hook after the user's profile",
			Profile{Kind: KindPwsh, Path: "pwsh.exe", Args: []string{"-NoLogo"}}, `C:\src`,
			[]string{"-NoLogo", "-NoExit", "-Command", ". '" + filepath.Join("/cache/snow/shell", "snow.ps1") + "'"}, nil, `C:\src`},
		{"other shells start as login shells without a hook",
			Profile{Kind: KindOther, Path: "/usr/bin/fish"}, "",
			[]string{"-l"}, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := tt.profile.Launch(h, tt.dir)
			if l.Path != tt.profile.Path || !slices.Equal(l.Args, tt.wantArgs) || l.Dir != tt.wantDir {
				t.Fatalf("Launch() = %+v; want args %q, dir %q", l, tt.wantArgs, tt.wantDir)
			}
			for _, kv := range tt.wantEnv {
				if !slices.Contains(l.Env, kv) {
					t.Fatalf("Launch().Env = %q, missing %q", l.Env, kv)
				}
			}
		})
	}
}

func TestLaunchZshKeepsUserZDOTDIR(t *testing.T) {
	t.Setenv("ZDOTDIR", "/home/u/.config/zsh")
	l := Profile{Kind: KindZsh, Path: "/bin/zsh"}.Launch(Hooks{Dir: "/c"}, "")
	if !slices.Contains(l.Env, "SNOW_USER_ZDOTDIR=/home/u/.config/zsh") {
		t.Fatalf("Env = %q, want the user's ZDOTDIR saved", l.Env)
	}
}

func TestLaunchCmdPrompt(t *testing.T) {
	t.Setenv("COMPUTERNAME", "PC")
	t.Setenv("PROMPT", "")
	l := Profile{Kind: KindCmd, Path: "cmd.exe"}.Launch(Hooks{}, "")
	want := `PROMPT=$E]7;file://PC/$P$E\$P$G`
	if !slices.Contains(l.Env, want) {
		t.Fatalf("Env = %q, want %q", l.Env, want)
	}
}

func TestLaunchWSL(t *testing.T) {
	p := Profile{Kind: KindWSL, Path: "wsl.exe", Args: []string{"-d", "Ubuntu"}, Distro: "Ubuntu"}
	l := p.Launch(Hooks{Dir: `C:\Users\emmy\AppData\Local\snow\shell`}, "/home/emmy/api")
	want := []string{"-d", "Ubuntu", "--cd", "/home/emmy/api", "-e", "sh", "/mnt/c/Users/emmy/AppData/Local/snow/shell/wsl.sh"}
	if !slices.Equal(l.Args, want) || l.Dir != "" {
		t.Fatalf("Launch() = %+v, want args %q and no Windows dir", l, want)
	}
}

func TestWSLPath(t *testing.T) {
	tests := []struct{ in, want string }{
		{`C:\Users\emmy\AppData`, "/mnt/c/Users/emmy/AppData"},
		{`D:\x y\z`, "/mnt/d/x y/z"},
		{"/already/linux", "/already/linux"},
	}
	for _, tt := range tests {
		if got := wslPath(tt.in); got != tt.want {
			t.Errorf("wslPath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
