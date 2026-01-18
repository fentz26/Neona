//go:build windows

package main

import (
	"os/exec"
)

func configureDaemonProc(cmd *exec.Cmd) {
	// Windows doesn't use Setsid.
	// By default, started processes can be independent enough for our simple use case.
}
