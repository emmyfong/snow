package shell

import (
	"errors"
	"slices"
	"testing"
)

func TestAppendArgs(t *testing.T) {
	script, err := programAt(system{goos: "windows", lookPath: func(string) (string, error) { return `C:\Windows\cmd.exe`, nil }}, `C:\npm\claude.cmd`, []string{"--verbose"})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		args     []string
		cmdLine  string
		extra    []string
		wantArgs []string
		wantLine string
		wantErr  error
	}{
		{"plain program", []string{"-d", "Ubuntu"}, "", []string{"--permission-mode", "plan"}, []string{"-d", "Ubuntu", "--permission-mode", "plan"}, "", nil},
		{"cmd script", script.Args, script.CmdLine, []string{"--append-system-prompt-file", `C:\Users\Emmy Fong\.snow\roles\reviewer.md`},
			[]string{"/D", "/S", "/C", `"C:\npm\claude.cmd" --verbose --append-system-prompt-file "C:\Users\Emmy Fong\.snow\roles\reviewer.md"`},
			`"C:\Windows\cmd.exe" /D /S /C ""C:\npm\claude.cmd" --verbose --append-system-prompt-file "C:\Users\Emmy Fong\.snow\roles\reviewer.md""`, nil},
		{"unsafe for cmd", script.Args, script.CmdLine, []string{"100%"}, nil, "", ErrScriptArg},
		{"nothing to add", script.Args, script.CmdLine, nil, script.Args, script.CmdLine, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, line, err := AppendArgs(tt.args, tt.cmdLine, tt.extra...)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if !slices.Equal(args, tt.wantArgs) || line != tt.wantLine {
				t.Fatalf("got %q\n%s\nwant %q\n%s", args, line, tt.wantArgs, tt.wantLine)
			}
		})
	}
	if len(script.Args) != 4 || script.Args[3] != `"C:\npm\claude.cmd" --verbose` {
		t.Fatalf("AppendArgs changed its input: %q", script.Args)
	}
}
