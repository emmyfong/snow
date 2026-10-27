package shell

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

var (
	// ErrProgramNotFound means a program is not on this machine's PATH.
	ErrProgramNotFound = errors.New("program not found")
	// ErrScriptArg means an argument cannot pass safely to a .cmd or .bat
	// script: cmd expands % and reads " itself.
	ErrScriptArg = errors.New(`a .cmd or .bat program cannot take an argument that holds ", %, or ! characters`)
)

// Program returns a profile called name that runs argv on this machine:
// argv[0] as found on PATH, then the other arguments. A .cmd or .bat
// script, such as npx on Windows, runs through cmd. The profile has
// KindCommand; callers may change Name and Kind.
func Program(name string, argv []string) (Profile, error) {
	return program(currentSystem(), name, argv)
}

func program(sys system, name string, argv []string) (Profile, error) {
	if len(argv) == 0 || argv[0] == "" {
		return Profile{}, fmt.Errorf("%s: %w: no program given", name, ErrProgramNotFound)
	}
	path, err := sys.lookPath(argv[0])
	if err != nil {
		return Profile{}, fmt.Errorf("%s: %w: %s", name, ErrProgramNotFound, argv[0])
	}
	p, err := programAt(sys, path, argv[1:])
	if err != nil {
		return Profile{}, fmt.Errorf("%s: %w", name, err)
	}
	p.Name = name
	return p, nil
}

// programAt returns a KindCommand profile that runs the program at path
// with args. On Windows a .cmd or .bat script runs through cmd, with the
// whole command line in CmdLine: /S strips only the outer quotes, so a
// path with spaces, parentheses, or & stays one quoted word.
func programAt(sys system, path string, args []string) (Profile, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if sys.goos != "windows" || ext != ".cmd" && ext != ".bat" {
		return Profile{Kind: KindCommand, Path: path, Args: args}, nil
	}
	cmd, err := sys.lookPath("cmd.exe")
	if err != nil {
		return Profile{}, fmt.Errorf("%w: cmd.exe, to run %s", ErrProgramNotFound, path)
	}
	line := `"` + path + `"`
	for _, a := range args {
		q, err := cmdArg(a)
		if err != nil {
			return Profile{}, err
		}
		line += " " + q
	}
	return Profile{
		Kind: KindCommand, Path: cmd,
		Args:    []string{"/D", "/S", "/C", line},
		CmdLine: `"` + cmd + `" /D /S /C "` + line + `"`,
	}, nil
}

// cmdArg quotes a for cmd when it holds a blank or a character cmd reads.
func cmdArg(a string) (string, error) {
	if strings.ContainsAny(a, `"%!`) {
		return "", ErrScriptArg
	}
	if a == "" || strings.ContainsAny(a, " \t&|<>^(),;=") {
		return `"` + a + `"`, nil
	}
	return a, nil
}
