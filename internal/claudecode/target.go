package claudecode

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ErrChanged means the settings file changed while snow claude waited
// for the user's answer. Snow writes nothing over the newer file.
var ErrChanged = errors.New("claudecode: the settings changed while Snow asked; nothing changed, run it again")

// Target is one Claude Code settings file that snow claude changes.
type Target interface {
	// Name says where the file is, for people.
	Name() string
	// Command is the snow hook command as this Claude Code runs it.
	Command() string
	// Read returns the file's bytes; a missing file is nil and no error.
	Read() ([]byte, error)
	// Write replaces the file with data. It keeps a copy of the file, at
	// Backup, the first time only, so the copy holds the file from before
	// Snow. A file that is a link stays one; the file it points to
	// changes.
	Write(data []byte) error
	// Backup says where Write keeps the copy, for people.
	Backup() string
}

// backupSuffix names the copy Snow keeps; its own name leaves the user's
// own backups alone.
const backupSuffix = ".snow.bak"

// SettingsPath is Claude Code's user settings file: settings.json in
// CLAUDE_CONFIG_DIR when it is set, as Claude Code reads it, or else in
// ~/.claude.
func SettingsPath() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "settings.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("claudecode: find the home folder: %w", err)
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// FileTarget is a settings file on this machine.
type FileTarget struct {
	Path string
	Cmd  string // the hook command; see HookCommand
}

// Name implements Target.
func (t FileTarget) Name() string { return t.Path }

// Command implements Target.
func (t FileTarget) Command() string { return t.Cmd }

// Backup implements Target.
func (t FileTarget) Backup() string { return t.real() + backupSuffix }

// real is the file Path names, past any links. A file that does not exist
// yet is Path itself.
func (t FileTarget) real() string {
	if p, err := filepath.EvalSymlinks(t.Path); err == nil {
		return p
	}
	return t.Path
}

// Read implements Target.
func (t FileTarget) Read() ([]byte, error) {
	b, err := os.ReadFile(t.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claudecode: %w", err)
	}
	return b, nil
}

// Write implements Target. It writes a temporary file next to the real
// file, syncs it, and renames it over the file, so Claude Code never reads
// half a file. The file keeps its permissions.
func (t FileTarget) Write(data []byte) error {
	file := t.real()
	mode := fs.FileMode(0o600)
	old, err := os.ReadFile(file)
	switch {
	case err == nil:
		if fi, err := os.Stat(file); err == nil {
			mode = fi.Mode().Perm()
		}
		if err := backupOnce(file+backupSuffix, old, mode); err != nil {
			return err
		}
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("claudecode: %w", err)
	}
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("claudecode: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "settings-*.json")
	if err != nil {
		return fmt.Errorf("claudecode: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // fails once renamed, as it should
	_, werr := tmp.Write(data)
	serr := tmp.Sync()
	cerr := tmp.Close()
	if err := errors.Join(werr, serr, cerr, os.Chmod(tmp.Name(), mode)); err != nil {
		return fmt.Errorf("claudecode: write the settings: %w", err)
	}
	if err := os.Rename(tmp.Name(), file); err != nil {
		return fmt.Errorf("claudecode: write the settings: %w", err)
	}
	return nil
}

// backupOnce writes data to path unless a copy is there already.
func backupOnce(path string, data []byte, mode fs.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("claudecode: back up the settings: %w", err)
	}
	_, werr := f.Write(data)
	if err := errors.Join(werr, f.Sync(), f.Close()); err != nil {
		return fmt.Errorf("claudecode: back up the settings: %w", err)
	}
	return nil
}

// WSLTarget is the settings file inside a WSL distribution, read and
// written through wsl.exe. Its Claude Code runs the Windows snow.exe
// through WSL's interop, which reaches the Windows server's socket.
type WSLTarget struct {
	WSL    string // the path of wsl.exe
	Distro string
	Cmd    string // the hook command, with snow.exe's path as WSL sees it
	// run runs wsl.exe with args and stdin; nil runs it for real. Tests
	// replace it.
	run func(args []string, stdin []byte) ([]byte, error)
}

// wslTimeout bounds one wsl.exe call: it may start the distribution.
const wslTimeout = 30 * time.Second

// wslSettings finds the settings file inside the distribution, as Claude
// Code there finds it, past a link.
const wslSettings = `d="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"; f="$d/settings.json"; ` +
	`if [ -L "$f" ]; then f=$(readlink -f "$f") || exit 1; fi; `

// wslWrite writes stdin over the settings file. The temporary file starts
// as a copy, so it keeps the file's mode, and umask keeps a new one
// private. The copy goes to the backup once. A short input, of fewer bytes
// than $1, leaves the file alone.
const wslWrite = `umask 077; ` + wslSettings + `mkdir -p "$(dirname "$f")" || exit 1; t="$f.snow.tmp"; ` +
	`if [ -f "$f" ]; then cp -p "$f" "$t" || exit 1; [ -e "$f` + backupSuffix + `" ] || cp -p "$f" "$f` + backupSuffix + `" || exit 1; fi; ` +
	`cat > "$t" || exit 1; ` +
	`if [ "$(wc -c < "$t")" -ne "$1" ]; then rm -f "$t"; echo "short write" >&2; exit 3; fi; ` +
	`mv "$t" "$f"`

// Name implements Target.
func (t WSLTarget) Name() string { return "Claude Code's settings in " + t.Distro }

// Command implements Target.
func (t WSLTarget) Command() string { return t.Cmd }

// Backup implements Target.
func (t WSLTarget) Backup() string {
	return "settings.json" + backupSuffix + ", next to them in " + t.Distro
}

// Read implements Target. A first line says whether the file exists, so a
// missing file and an empty one differ.
func (t WSLTarget) Read() ([]byte, error) {
	out, err := t.sh(wslSettings+`if [ -f "$f" ]; then echo file; cat "$f"; else echo none; fi`, nil)
	if err != nil {
		return nil, err
	}
	head, body, _ := bytes.Cut(out, []byte("\n"))
	switch strings.TrimSpace(string(head)) {
	case "none":
		return nil, nil
	case "file":
		return body, nil
	}
	return nil, fmt.Errorf("claudecode: read the settings in %s: unexpected output %q", t.Distro, head)
}

// Write implements Target, as FileTarget.Write does, inside the
// distribution.
func (t WSLTarget) Write(data []byte) error {
	_, err := t.sh(wslWrite, data, strconv.Itoa(len(data)))
	return err
}

// sh runs script with sh inside the distribution, with args as $1 on.
func (t WSLTarget) sh(script string, stdin []byte, args ...string) ([]byte, error) {
	argv := append([]string{"-d", t.Distro, "-e", "sh", "-c", script, "sh"}, args...)
	run := t.run
	if run == nil {
		run = func(argv []string, stdin []byte) ([]byte, error) {
			ctx, cancel := context.WithTimeout(context.Background(), wslTimeout)
			defer cancel()
			cmd := exec.CommandContext(ctx, t.WSL, argv...)
			cmd.Stdin = bytes.NewReader(stdin)
			return cmd.Output()
		}
	}
	out, err := run(argv, stdin)
	if err != nil {
		return nil, fmt.Errorf("claudecode: settings in %s: %w", t.Distro, err)
	}
	return out, nil
}
