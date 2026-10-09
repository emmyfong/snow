package term

import (
	"os"
	"strconv"
)

// cwd reads the program's folder from /proc. It is a fallback for shells
// that do not report OSC 7, and it sees the shell itself, not its children.
func (u *unixPTY) cwd() string {
	dir, err := os.Readlink("/proc/" + strconv.Itoa(u.cmd.Process.Pid) + "/cwd")
	if err != nil {
		return ""
	}
	return dir
}
