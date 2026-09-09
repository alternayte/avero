//go:build windows

package dev

import "os/exec"

// Group puts a command in a process group of its own. Windows carries no
// process group of this shape, so the function does nothing.
func Group(cmd *exec.Cmd) {}

// KillGroup ends a command.
func KillGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
