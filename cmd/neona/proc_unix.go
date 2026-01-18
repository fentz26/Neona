//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

func configureDaemonProc(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
