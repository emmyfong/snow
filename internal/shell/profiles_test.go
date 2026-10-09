package shell

import (
	"errors"
	"slices"
	"testing"
	"unicode/utf16"
)

// utf16le encodes s the way wsl.exe writes its output.
func utf16le(s string, bom bool) []byte {
	var b []byte
	if bom {
		b = append(b, 0xFF, 0xFE)
	}
	for _, u := range utf16.Encode([]rune(s)) {
		b = append(b, byte(u), byte(u>>8))
	}
	return b
}

func TestParseWSLList(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want []string
	}{
		{"distros", utf16le("Ubuntu\r\nDebian\r\n", false), []string{"Ubuntu", "Debian"}},
		{"with BOM", utf16le("Ubuntu-24.04\r\n", true), []string{"Ubuntu-24.04"}},
		{"drops Docker's internal distros", utf16le("Ubuntu\r\ndocker-desktop\r\ndocker-desktop-data\r\n", false), []string{"Ubuntu"}},
		{"blank lines and padding", utf16le("\r\n  Ubuntu  \r\n\r\n", false), []string{"Ubuntu"}},
		{"empty", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseWSLList(tt.in); !slices.Equal(got, tt.want) {
				t.Fatalf("parseWSLList() = %q, want %q", got, tt.want)
			}
		})
	}
}

func names(ps []Profile) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Name)
	}
	return out
}

func TestDetectWindows(t *testing.T) {
	tests := []struct {
		name    string
		found   []string
		distros []byte
		wslErr  error
		want    []string
	}{
		{"everything", []string{"pwsh.exe", "powershell.exe", "cmd.exe", "wsl.exe"},
			utf16le("Ubuntu\r\nDebian\r\n", false),
			nil, []string{"PowerShell", "Windows PowerShell", "Command Prompt", "Ubuntu", "Debian"}},
		{"no pwsh", []string{"powershell.exe", "cmd.exe", "wsl.exe"},
			utf16le("Ubuntu\r\n", false), nil, []string{"Windows PowerShell", "Command Prompt", "Ubuntu"}},
		{"wsl without distros", []string{"powershell.exe", "cmd.exe", "wsl.exe"},
			nil, errors.New("exit status 1"), []string{"Windows PowerShell", "Command Prompt"}},
		{"no wsl", []string{"cmd.exe"}, nil, nil, []string{"Command Prompt"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sys := system{
				goos: "windows",
				lookPath: func(name string) (string, error) {
					if slices.Contains(tt.found, name) {
						return `C:\bin\` + name, nil
					}
					return "", errors.New("not found")
				},
				wslList: func() ([]byte, error) { return tt.distros, tt.wslErr },
			}
			if got := names(detect(sys)); !slices.Equal(got, tt.want) {
				t.Fatalf("detect() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectWSLProfile(t *testing.T) {
	sys := system{
		goos:     "windows",
		lookPath: func(name string) (string, error) { return `C:\Windows\System32\` + name, nil },
		wslList:  func() ([]byte, error) { return utf16le("Ubuntu\r\n", false), nil },
	}
	var wsl Profile
	for _, p := range detect(sys) {
		if p.Kind == KindWSL {
			wsl = p
		}
	}
	if wsl.Distro != "Ubuntu" || wsl.Path != `C:\Windows\System32\wsl.exe` || !slices.Equal(wsl.Args, []string{"-d", "Ubuntu"}) {
		t.Fatalf("WSL profile = %+v", wsl)
	}
}

func TestDetectUnix(t *testing.T) {
	tests := []struct {
		name, shell, wantName, wantPath string
		wantKind                        Kind
	}{
		{"zsh from SHELL", "/usr/bin/zsh", "zsh", "/usr/bin/zsh", KindZsh},
		{"bash from SHELL", "/bin/bash", "bash", "/bin/bash", KindBash},
		{"fish is unsupported for hooks", "/usr/bin/fish", "fish", "/usr/bin/fish", KindOther},
		{"no SHELL falls back to sh", "", "sh", "/bin/sh", KindOther},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sys := system{goos: "linux", shellEnv: tt.shell}
			got := detect(sys)
			if len(got) != 1 || got[0].Name != tt.wantName || got[0].Path != tt.wantPath || got[0].Kind != tt.wantKind {
				t.Fatalf("detect() = %+v, want one %s profile at %s", got, tt.wantName, tt.wantPath)
			}
		})
	}
}
