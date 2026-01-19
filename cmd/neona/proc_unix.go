//go:build !windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

func configureDaemonProc(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func restartSelf() error {
	argv0, err := exec.LookPath(os.Args[0])
	if err != nil {
		return err
	}
	return syscall.Exec(argv0, os.Args, os.Environ())
}
