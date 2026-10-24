package shell

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// tree is a command and the processes it starts: a job object.
type tree struct {
	cmd *exec.Cmd
	job windows.Handle
}

// ntResumeProcess resumes every thread of a process. exec.Cmd does not
// give the main thread's handle, which ResumeThread would need.
var ntResumeProcess = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtResumeProcess")

// newTree makes a job that kills its processes when it is closed, and
// sets cmd to start suspended with no window, so no child can start
// before the job holds the command. A cancel ends the job.
func newTree(cmd *exec.Cmd, cmdLine string) (*tree, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create job: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("set job limits: %w", err)
	}
	t := &tree{cmd: cmd, job: job}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_SUSPENDED,
		HideWindow:    true,
		CmdLine:       cmdLine,
	}
	// A cancel before the command joins the job finds the job empty; then
	// exec kills the process itself after WaitDelay.
	cmd.Cancel = t.kill
	return t, nil
}

// start starts the command, puts it in the job, and resumes it.
func (t *tree) start() error {
	if err := t.cmd.Start(); err != nil {
		return err
	}
	if err := t.adopt(); err != nil {
		_ = t.cmd.Process.Kill()
		_ = t.cmd.Wait()
		return err
	}
	return nil
}

func (t *tree) adopt() error {
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_SUSPEND_RESUME,
		false, uint32(t.cmd.Process.Pid))
	if err != nil {
		return fmt.Errorf("open process: %w", err)
	}
	defer func() { _ = windows.CloseHandle(h) }()
	if err := windows.AssignProcessToJobObject(t.job, h); err != nil {
		return fmt.Errorf("assign job: %w", err)
	}
	if status, _, _ := ntResumeProcess.Call(uintptr(h)); status != 0 {
		return fmt.Errorf("resume process: %w", windows.NTStatus(status))
	}
	return nil
}

// kill ends every process in the job.
func (t *tree) kill() error {
	if err := windows.TerminateJobObject(t.job, 1); err != nil && !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return err
	}
	return nil
}

func (t *tree) close() { _ = windows.CloseHandle(t.job) }
