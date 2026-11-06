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
func (t *tree) kill() error { return t.signal(syscall.SIGKILL) }

// terminate asks every process in the group to exit.
func (t *tree) terminate() error { return t.signal(syscall.SIGTERM) }

// signal sends sig to the group. ESRCH means the group is gone. macOS
// answers EPERM instead when every process left in the group has exited
// but is not yet reaped, so that is gone too: Snow started the group as
// the same user, so it never lacks the permission itself.
func (t *tree) signal(sig syscall.Signal) error {
	if err := syscall.Kill(-t.cmd.Process.Pid, sig); err != nil && err != syscall.ESRCH && err != syscall.EPERM {
		return err
	}
	return nil
}

func (t *tree) close() {}
