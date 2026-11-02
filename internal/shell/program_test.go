package shell

import (
	"errors"
	"slices"
	"testing"
)

func TestProgram(t *testing.T) {
	tests := []struct {
		name  string
		goos  string
		found map[string]string // program name to the path lookPath gives
		argv  []string
		want  Profile
		err   error
	}{
		{name: "on PATH", goos: "linux", found: map[string]string{"claude": "/bin/claude"},
			argv: []string{"claude", "--model", "opus"},
			want: Profile{Kind: KindCommand, Path: "/bin/claude", Args: []string{"--model", "opus"}}},
		{name: "not found", goos: "linux", argv: []string{"claude"}, err: ErrProgramNotFound},
		{name: "no program", goos: "linux", argv: nil, err: ErrProgramNotFound},
		{name: "an .exe runs as it is", goos: "windows", found: map[string]string{"claude": `C:\bin\claude.exe`},
			argv: []string{"claude"}, want: Profile{Kind: KindCommand, Path: `C:\bin\claude.exe`}},
		{name: "a .cmd runs through cmd", goos: "windows",
			found: map[string]string{"npx": `C:\Program Files\nodejs\npx.cmd`, "cmd.exe": `C:\bin\cmd.exe`},
			argv:  []string{"npx", "omniroute", "--port", "a b"},
			want: Profile{Kind: KindCommand, Path: `C:\bin\cmd.exe`,
				Args:    []string{"/D", "/S", "/C", `"C:\Program Files\nodejs\npx.cmd" omniroute --port "a b"`},
				CmdLine: `"C:\bin\cmd.exe" /D /S /C ""C:\Program Files\nodejs\npx.cmd" omniroute --port "a b""`}},
		{name: "a .cmd argument cmd would expand", goos: "windows",
			found: map[string]string{"npx": `C:\bin\npx.cmd`, "cmd.exe": `C:\bin\cmd.exe`},
			argv:  []string{"npx", "%PATH%"}, err: ErrScriptArg},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sys := system{goos: tt.goos, lookPath: func(name string) (string, error) {
				if p, ok := tt.found[name]; ok {
					return p, nil
				}
				return "", errors.New("not found")
			}}
			got, err := program(sys, "test", tt.argv)
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("error %v, want %v", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Name != "test" || got.Kind != tt.want.Kind || got.Path != tt.want.Path ||
				!slices.Equal(got.Args, tt.want.Args) || got.CmdLine != tt.want.CmdLine {
				t.Fatalf("program() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
