//go:build linux || darwin

package main

import "syscall"

// parentAlive probes the parent desktop process with signal 0.
func parentAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
