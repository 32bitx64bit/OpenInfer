//go:build darwin

package processes

import "syscall"

// platformSetupExtra is a no-op on macOS; process-group cleanup plus the
// backend's parent-death watchdog cover abnormal exits.
func platformSetupExtra(attr *syscall.SysProcAttr) {}
