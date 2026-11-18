//go:build linux

package cli

import (
	"os/exec"
	"syscall"
)

// bind makes the kernel SIGTERM the child if the run dies (even by kill -9).
// Pdeathsig is tied to the spawning thread, so the caller locks the OS thread.
func bind(c *exec.Cmd) { c.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM} }

func adopt(*exec.Cmd) error { return nil }
