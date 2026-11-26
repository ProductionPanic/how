//go:build !windows

package server

import (
	"os/exec"
	"syscall"
)

// detach starts the process in its own session so it keeps running
// after how exits and isn't hit by ctrl+c in this terminal.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
