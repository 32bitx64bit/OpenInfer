//go:build linux

package processes

import "syscall"

// platformSetupExtra sets PR_SET_PDEATHSIG: the kernel delivers SIGKILL to
// the child when the backend process dies, preventing abandoned inference
// processes after a crash.
func platformSetupExtra(attr *syscall.SysProcAttr) {
	attr.Pdeathsig = syscall.SIGKILL
}
