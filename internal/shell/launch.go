package shell

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// hookFiles holds the folder-hook scripts. "all:" includes the zsh dotfiles.
//
//go:embed all:hooks
var hookFiles embed.FS

// Hooks is a folder on disk that holds the hook scripts.
type Hooks struct{ Dir string }

// DefaultHooksDir returns the folder Snow writes hooks to: snow/shell in the
// user's cache folder (%LOCALAPPDATA% on Windows, ~/.cache on Linux).
func DefaultHooksDir() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("find cache folder: %w", err)
	}
	return filepath.Join(cache, "snow", "shell"), nil
}

// WriteHooks writes the hook scripts into dir, replacing older copies.
func WriteHooks(dir string) (Hooks, error) { return writeHooks(hookFiles, dir) }

// writeHooks copies the hooks folder of src into dir. It writes LF line
// endings: a Windows checkout can turn the embedded scripts into CRLF, and
// bash and zsh reject CR characters.
func writeHooks(src fs.FS, dir string) (Hooks, error) {
	err := fs.WalkDir(src, "hooks", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(name, "hooks")))
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		b, err := fs.ReadFile(src, name)
		if err != nil {
			return err
		}
		return os.WriteFile(target, bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")), 0o600)
	})
	if err != nil {
		return Hooks{}, fmt.Errorf("write shell hooks: %w", err)
	}
	return Hooks{Dir: dir}, nil
}

// Launch is how to start a profile with Snow's folder hook.
type Launch struct {
	Path string
	Args []string
	Env  []string // variables to add to the inherited environment
	Dir  string   // working folder; empty for WSL, which takes --cd instead
}

// Launch returns the command that starts p in dir with the folder hook from
// h. Shells without a hook start as login shells, like tmux does.
func (p Profile) Launch(h Hooks, dir string) Launch {
	l := Launch{Path: p.Path, Args: slices.Clone(p.Args), Dir: dir}
	switch p.Kind {
	case KindBash:
		l.Args = append(l.Args, "--rcfile", filepath.Join(h.Dir, "bash.sh"), "-i")
	case KindZsh:
		l.Args = append(l.Args, "-l")
		l.Env = []string{
			"SNOW_USER_ZDOTDIR=" + userZDOTDIR(),
			"ZDOTDIR=" + filepath.Join(h.Dir, "zsh"),
		}
	case KindPwsh:
		l.Args = append(l.Args, "-NoExit", "-Command", ". '"+filepath.Join(h.Dir, "snow.ps1")+"'")
	case KindCmd:
		l.Env = []string{"PROMPT=" + cmdPrompt()}
	case KindWSL:
		if dir != "" {
			l.Args = append(l.Args, "--cd", dir)
		}
		l.Args = append(l.Args, "-e", "sh", wslPath(filepath.Join(h.Dir, "wsl.sh")))
		l.Dir = ""
	default:
		l.Args = append(l.Args, "-l")
	}
	return l
}

func userZDOTDIR() string {
	if d := os.Getenv("ZDOTDIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir() // empty home only means zsh falls back to $HOME
	return home
}

// cmdPrompt prefixes the user's PROMPT with an OSC 7 report. cmd expands $E
// to ESC and $P to the current folder; the host name is filled in now
// because cmd does not expand %VARIABLES% inside PROMPT.
func cmdPrompt() string {
	user := os.Getenv("PROMPT")
	if user == "" {
		user = "$P$G"
	}
	return `$E]7;file://` + os.Getenv("COMPUTERNAME") + `/$P$E\` + user
}

// wslPath turns a Windows path into the path WSL mounts it at, assuming the
// default automount root /mnt. Other paths are returned unchanged.
func wslPath(p string) string {
	if len(p) < 3 || p[1] != ':' || !isLetter(p[0]) {
		return p
	}
	return "/mnt/" + strings.ToLower(p[:1]) + strings.ReplaceAll(p[2:], `\`, "/")
}

func isLetter(c byte) bool { return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') }
