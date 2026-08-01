//go:build linux || darwin

package processes

import (
	"os/exec"
	"syscall"
)

// platformSetup places the child in its own process group so the whole tree
// can be signaled at once. Linux additionally arranges PR_SET_PDEATHSIG (see
// platformSetupExtra) so the kernel kills the child if OpenInfer Studio dies
// unexpectedly.
func platformSetup(cmd *exec.Cmd) error {
	attr := &syscall.SysProcAttr{Setpgid: true}
	platformSetupExtra(attr)
	cmd.SysProcAttr = attr
	return nil
}

func platformAfterStart(cmd *exec.Cmd) error { return nil }

// platformSignal sends SIGTERM to the process group for graceful shutdown.
func platformSignal(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
}

// platformKillTree sends SIGKILL to the entire process group.
func platformKillTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	// Negative PID targets the process group created with Setpgid.
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		// Group may be gone; try the direct process.
		_ = cmd.Process.Kill()
		return nil
	}
	return nil
}
