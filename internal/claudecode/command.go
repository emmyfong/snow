package claudecode

import (
	"errors"
	"fmt"
	"path"
	"strings"
)

// ErrNotSnow means snow runs under another file name, such as a download
// called snow-linux-amd64. Its hook could not be told apart from other
// commands, so setup would add it again each run and remove could not
// find it.
var ErrNotSnow = errors.New("claudecode: snow must run by its own name, snow or snow.exe, to set up its hooks")

// CheckProgram returns ErrNotSnow unless exe, the snow program, is named
// snow or snow.exe.
func CheckProgram(exe string) error {
	if !isSnowName(exe) {
		return fmt.Errorf("%w; it is %s", ErrNotSnow, exe)
	}
	return nil
}

// HookCommand is the command Claude Code runs for Snow's hooks: the snow
// program at exe, in single quotes, then hook. Claude Code runs hook
// commands with bash, or with Git Bash on Windows, so the path uses
// forward slashes.
func HookCommand(exe string) string {
	return shellQuote(strings.ReplaceAll(exe, `\`, "/")) + " hook"
}

// quote puts s in single quotes for a shell, with an embedded quote closed,
// escaped, and opened again.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// IsSnowCommand reports whether cmd runs snow hook: a program named snow or
// snow.exe, in any folder and quoted or not, with the one argument hook.
func IsSnowCommand(cmd string) bool {
	prog, rest, ok := firstWord(strings.TrimSpace(cmd))
	return ok && strings.TrimSpace(rest) == "hook" && isSnowName(prog)
}

// isSnowName reports whether the program at p is named snow or snow.exe.
func isSnowName(p string) bool {
	name := strings.ToLower(path.Base(strings.ReplaceAll(p, `\`, "/")))
	return name == "snow" || name == "snow.exe"
}

// firstWord splits off the first shell word of s, removing its quotes.
func firstWord(s string) (word, rest string, ok bool) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == ' ' || c == '\t':
			return b.String(), s[i:], b.Len() > 0
		case c == '\'' || c == '"':
			j := strings.IndexByte(s[i+1:], c)
			if j < 0 {
				return "", "", false
			}
			b.WriteString(s[i+1 : i+1+j])
			i += j + 1
		case c == '\\' && i+1 < len(s):
			b.WriteByte(s[i+1])
			i++
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), "", b.Len() > 0
}
