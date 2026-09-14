//go:build !windows

package dev

import (
	"os/exec"
	"syscall"
)

// Group puts a command in a process group of its own.
//
// `go tool templ` starts the tool as a child of its own, so a kill of the
// command leaves the tool behind. The tool then holds the output of the loop,
// and the loop never ends. A kill of the group ends both.
func Group(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// KillGroup ends a command and every process that it started.
func KillGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	// A negative identifier names the group.
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		_ = cmd.Process.Kill()
	}
}
