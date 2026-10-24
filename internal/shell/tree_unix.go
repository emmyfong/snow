//go:build !windows

package shell

import (
	"os/exec"
	"syscall"
)

// tree is a command and the processes it starts: a process group.
type tree struct{ cmd *exec.Cmd }

// newTree makes cmd start as the leader of a new process group, which a
// cancel kills.
func newTree(cmd *exec.Cmd, _ string) (*tree, error) {
	t := &tree{cmd: cmd}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = t.kill
	return t, nil
}

func (t *tree) start() error { return t.cmd.Start() }

// kill kills every process in the group. The group's ID is the leader's
// process ID. A group that is gone already is not an error.
func (t *tree) kill() error {
	if err := syscall.Kill(-t.cmd.Process.Pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		return err
	}
	return nil
}

func (t *tree) close() {}
