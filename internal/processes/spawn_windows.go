//go:build windows

package processes

import (
	"fmt"
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows supervision uses a Job Object configured with
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE, so closing the job handle (including
// the death of OpenInfer Studio) terminates the entire child tree.
//
// The process is started normally and then assigned to the job. There is a
// small window before assignment; llama-server does not spawn children at
// startup, so this is acceptable and keeps the implementation portable.

var (
	jobMu      sync.Mutex
	jobHandles = map[int]windows.Handle{}
)

func platformSetup(cmd *exec.Cmd) error { return nil }

func platformAfterStart(cmd *exec.Cmd) error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("CreateJobObject: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return fmt.Errorf("SetInformationJobObject: %w", err)
	}
	proc, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_INFORMATION,
		false, uint32(cmd.Process.Pid))
	if err != nil {
		windows.CloseHandle(job)
		return fmt.Errorf("OpenProcess: %w", err)
	}
	err = windows.AssignProcessToJobObject(job, proc)
	windows.CloseHandle(proc)
	if err != nil {
		windows.CloseHandle(job)
		return fmt.Errorf("AssignProcessToJobObject: %w", err)
	}
	jobMu.Lock()
	jobHandles[cmd.Process.Pid] = job
	jobMu.Unlock()
	return nil
}

func platformSignal(cmd *exec.Cmd) error {
	// There is no portable graceful console signal for detached trees;
	// TerminateJobObject is the reliable shutdown path on Windows.
	return platformKillTree(cmd)
}

func platformKillTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	pid := cmd.Process.Pid
	jobMu.Lock()
	job, ok := jobHandles[pid]
	if ok {
		delete(jobHandles, pid)
	}
	jobMu.Unlock()
	if ok {
		_ = windows.TerminateJobObject(job, 1)
		windows.CloseHandle(job)
		return nil
	}
	return cmd.Process.Kill()
}
