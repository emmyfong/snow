//go:build !windows

package shell

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestSignalZombieGroup: a group whose only process has exited but is not
// yet reaped is gone. macOS answers EPERM for such a group (#279).
func TestSignalZombieGroup(t *testing.T) {
	cmd := exec.CommandContext(context.Background(), "/bin/sh", "-c", "exit 0")
	tr, err := newTree(cmd, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Wait() })
	waitZombie(t, cmd.Process.Pid)
	if err := tr.terminate(); err != nil {
		t.Fatalf("terminate: %v, want nil for a group that has exited", err)
	}
	if err := tr.kill(); err != nil {
		t.Fatalf("kill: %v, want nil for a group that has exited", err)
	}
}

// TestSignalForeignGroup: a group that Snow may not signal is an error, not
// a group that is gone. Only signal 0 is sent, so the group is not touched.
func TestSignalForeignGroup(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root may signal every group")
	}
	// A root process can exit between the scan and the signal. Its group then
	// answers ESRCH, which signal reports as nil. Try each candidate until one
	// still answers, so a process that exits mid-test does not fail the test.
	for _, pid := range rootGroupLeaders(t) {
		err := (&tree{cmd: &exec.Cmd{Process: &os.Process{Pid: pid}}}).signal(0)
		if err == nil {
			continue
		}
		if !errors.Is(err, syscall.EPERM) {
			t.Fatalf("signal to root's group %d: %v, want EPERM", pid, err)
		}
		return
	}
	t.Skip("every root group leader has exited")
}

// waitZombie waits until pid has exited and is not reaped.
func waitZombie(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		out, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
		if err == nil && strings.HasPrefix(strings.TrimSpace(string(out)), "Z") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("process %d did not become a zombie", pid)
}

// rootGroupLeaders returns the processes that root owns and that lead their
// own group, other than init. It skips the test when there is none, as in some
// containers.
func rootGroupLeaders(t *testing.T) []int {
	t.Helper()
	out, err := exec.Command("ps", "-A", "-o", "pid=,pgid=,uid=").Output()
	if err != nil {
		t.Skipf("ps: %v", err)
	}
	var pids []int
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) != 3 || f[0] != f[1] || f[2] != "0" {
			continue
		}
		if pid, err := strconv.Atoi(f[0]); err == nil && pid > 1 {
			pids = append(pids, pid)
		}
	}
	if len(pids) == 0 {
		t.Skip("no process group that root owns")
	}
	return pids
}
