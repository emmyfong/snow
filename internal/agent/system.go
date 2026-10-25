package agent

import (
	"fmt"
	"runtime"

	"github.com/emmyfong/snow/internal/shell"
	"github.com/emmyfong/snow/internal/tool"
)

// DefaultSystem is the system prompt of a conversation in folder whose run
// tool uses sh, for a Config that sets none.
func DefaultSystem(folder tool.Folder, sh shell.Profile) string {
	where := folder.Path
	if folder.Distro != "" {
		where = fmt.Sprintf("%s in the WSL distribution %s", folder.Path, folder.Distro)
	}
	shellName := sh.Name
	if shellName == "" {
		shellName = "the system shell"
	}
	return fmt.Sprintf(`You are Snow, a coding agent in the user's terminal. You work in the folder %s, on %s. The run tool runs commands in %s.

Read files before you change them. Make the smallest change that does what the user asked, and say what you changed. Ask when the request is unclear instead of guessing. Tools that change files or run commands may need the user's approval; if the user says no, do not try another way to do the same thing.`, where, runtime.GOOS, shellName)
}
